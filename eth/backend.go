// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// Package eth implements the Ethereum protocol.
package eth

import (
	"errors"
	"fmt"
	"math/big"
	"runtime"
	"sync"
	"sync/atomic"
	"context"
	"time"
	

	"github.com/sesafoundation/sesn/accounts"
	"github.com/sesafoundation/sesn/common"
	"github.com/sesafoundation/sesn/common/hexutil"
	"github.com/sesafoundation/sesn/consensus"
	"github.com/sesafoundation/sesn/consensus/clique"
	"github.com/sesafoundation/sesn/consensus/ethash"
	"github.com/sesafoundation/sesn/consensus/sonium"
	"github.com/sesafoundation/sesn/core"
	"github.com/sesafoundation/sesn/core/bloombits"
	"github.com/sesafoundation/sesn/core/rawdb"
	"github.com/sesafoundation/sesn/core/types"
	"github.com/sesafoundation/sesn/core/vm"
	"github.com/sesafoundation/sesn/eth/downloader"
	"github.com/sesafoundation/sesn/eth/filters"
	"github.com/sesafoundation/sesn/eth/gasprice"
	"github.com/sesafoundation/sesn/ethdb"
	"github.com/sesafoundation/sesn/event"
	"github.com/sesafoundation/sesn/internal/ethapi"
	"github.com/sesafoundation/sesn/log"
	"github.com/sesafoundation/sesn/miner"
	"github.com/sesafoundation/sesn/node"
	"github.com/sesafoundation/sesn/p2p"
	"github.com/sesafoundation/sesn/p2p/enode"
	"github.com/sesafoundation/sesn/p2p/enr"
	"github.com/sesafoundation/sesn/params"
	"github.com/sesafoundation/sesn/rlp"
	"github.com/sesafoundation/sesn/rpc"
	"github.com/sesafoundation/sesn/core/state"
	"github.com/sesafoundation/sesn/preconf"
)
var _ ethapi.Backend = (*Ethereum)(nil)
// Ethereum implements the Ethereum full node service.
type Ethereum struct {
	config *Config

	// Handlers
	txPool          *core.TxPool
	blockchain      *core.BlockChain
	protocolManager *ProtocolManager
	dialCandidates  enode.Iterator

	// DB interfaces
	chainDb ethdb.Database // Block chain database

	eventMux       *event.TypeMux
	engine         consensus.Engine
	accountManager *accounts.Manager

	bloomRequests     chan chan *bloombits.Retrieval // Channel receiving bloom data retrieval requests
	bloomIndexer      *core.ChainIndexer             // Bloom indexer operating during block imports
	closeBloomHandler chan struct{}

	// RPC backend wrapper (internal/ethapi)
	APIBackend *ethapi.EthAPIBackend

	miner     *miner.Miner
	gasPrice  *big.Int
	etherbase common.Address

	networkID     uint64
	netRPCService *ethapi.PublicNetAPI

	p2pServer *p2p.Server

	lock sync.RWMutex // Protects the variadic fields (e.g. gas price and etherbase)
}



// New creates a new Ethereum object (including the initialisation of the common Ethereum object)
func New(stack *node.Node, config *Config) (*Ethereum, error) {
	// ---- Safety / config checks ----
	if config.SyncMode == downloader.LightSync {
		return nil, errors.New("can't run eth.Ethereum in light sync mode, use les.LightEthereum")
	}
	if !config.SyncMode.IsValid() {
		return nil, fmt.Errorf("invalid sync mode %d", config.SyncMode)
	}
	if config.Miner.GasPrice == nil || config.Miner.GasPrice.Cmp(params.MinimalGasPrice) < 0 {
		log.Warn("Sanitizing invalid miner gas price", "provided", config.Miner.GasPrice, "updated", params.MinimalGasPrice.String())
		config.Miner.GasPrice = new(big.Int).Set(params.MinimalGasPrice)
	}
	if config.Miner.GasCeil > params.MaxGasTarget {
		log.Warn("Miner gas ceil invalid", "provided", config.Miner.GasCeil, "updated", params.MaxGasTarget)
		config.Miner.GasCeil = params.MaxGasTarget
	}
	if config.Miner.GasFloor < params.MinGasTarget {
		log.Warn("Miner gas floor invalid", "provided", config.Miner.GasFloor, "updated", params.MinGasTarget)
		config.Miner.GasFloor = params.MinGasTarget
	}
	if config.NoPruning && config.TrieDirtyCache > 0 {
		if config.SnapshotCache > 0 {
			config.TrieCleanCache += config.TrieDirtyCache * 3 / 5
			config.SnapshotCache += config.TrieDirtyCache * 2 / 5
		} else {
			config.TrieCleanCache += config.TrieDirtyCache
		}
		config.TrieDirtyCache = 0
	}
	log.Info("Allocated trie memory caches",
		"clean", common.StorageSize(config.TrieCleanCache)*1024*1024,
		"dirty", common.StorageSize(config.TrieDirtyCache)*1024*1024,
	)

	// ---- Assemble Ethereum object ----
	chainDb, err := stack.OpenDatabaseWithFreezer("chaindata", config.DatabaseCache, config.DatabaseHandles, config.DatabaseFreezer, "eth/db/chaindata/")
	if err != nil {
		return nil, err
	}
	chainConfig, genesisHash, genesisErr := core.SetupGenesisBlock(chainDb, config.Genesis)
	if _, ok := genesisErr.(*params.ConfigCompatError); genesisErr != nil && !ok {
		return nil, genesisErr
	}
	log.Info("Initialised chain configuration", "config", chainConfig)

	eth := &Ethereum{
		config:            config,
		chainDb:           chainDb,
		eventMux:          stack.EventMux(),
		accountManager:    stack.AccountManager(),
		closeBloomHandler: make(chan struct{}),
		networkID:         config.NetworkId,
		gasPrice:          config.Miner.GasPrice,
		etherbase:         config.Miner.Etherbase,
		bloomRequests:     make(chan chan *bloombits.Retrieval),
		bloomIndexer:      NewBloomIndexer(chainDb, params.BloomBitsBlocks, params.BloomConfirms),
		p2pServer:         stack.Server(),
	}

	// ---- (1) Create internal Backend wrapper used by RPC + consensus ----
	eth.APIBackend = ethapi.NewEthAPIBackend(stack.Config().ExtRPCEnabled(), eth)
	

	// ---- (2) Public chain API for consensus engines (must use APIBackend) ----
	ethAPI := ethapi.NewPublicBlockChainAPI(eth.APIBackend)

	// ---- (3) Create consensus engine ----
	eth.engine = CreateConsensusEngine(stack, chainConfig, &config.Ethash, config.Miner.Notify, config.Miner.Noverify, chainDb, ethAPI)

	// ---- Continue blockchain init ----
	bcVersion := rawdb.ReadDatabaseVersion(chainDb)
	if bcVersion != nil && *bcVersion > core.BlockChainVersion {
		return nil, fmt.Errorf("database version is v%d, Geth %s only supports v%d", *bcVersion, params.VersionWithMeta, core.BlockChainVersion)
	} else if bcVersion == nil || *bcVersion < core.BlockChainVersion {
		rawdb.WriteDatabaseVersion(chainDb, core.BlockChainVersion)
	}

	vmConfig := vm.Config{
		EnablePreimageRecording: config.EnablePreimageRecording,
		EWASMInterpreter:        config.EWASMInterpreter,
		EVMInterpreter:          config.EVMInterpreter,
	}
	cacheConfig := &core.CacheConfig{
		TrieCleanLimit:      config.TrieCleanCache,
		TrieCleanJournal:    stack.ResolvePath(config.TrieCleanCacheJournal),
		TrieCleanRejournal:  config.TrieCleanCacheRejournal,
		TrieCleanNoPrefetch: config.NoPrefetch,
		TrieDirtyLimit:      config.TrieDirtyCache,
		TrieDirtyDisabled:   config.NoPruning,
		TrieTimeLimit:       config.TrieTimeout,
		SnapshotLimit:       config.SnapshotCache,
		Preimages:           config.Preimages,
	}
	eth.blockchain, err = core.NewBlockChain(chainDb, cacheConfig, chainConfig, eth.engine, vmConfig, eth.shouldPreserve, &config.TxLookupLimit)
	if err != nil {
		return nil, err
	}
	if compat, ok := genesisErr.(*params.ConfigCompatError); ok {
		eth.blockchain.SetHead(compat.RewindTo)
		rawdb.WriteChainConfig(chainDb, genesisHash, chainConfig)
	}
	eth.bloomIndexer.Start(eth.blockchain)

	if config.TxPool.Journal != "" {
		config.TxPool.Journal = stack.ResolvePath(config.TxPool.Journal)
	}
	eth.txPool = core.NewTxPool(config.TxPool, chainConfig, eth.blockchain)

	cacheLimit := cacheConfig.TrieCleanLimit + cacheConfig.TrieDirtyLimit + cacheConfig.SnapshotLimit
	checkpoint := config.Checkpoint
	if checkpoint == nil {
		checkpoint = params.TrustedCheckpoints[genesisHash]
	}
	if eth.protocolManager, err = NewProtocolManager(chainConfig, checkpoint, config.SyncMode, config.NetworkId, eth.eventMux, eth.txPool, eth.engine, eth.blockchain, chainDb, cacheLimit, config.Whitelist); err != nil {
		return nil, err
	}

	eth.miner = miner.New(eth, &config.Miner, chainConfig, eth.EventMux(), eth.engine, eth.isLocalBlock)
	eth.miner.SetExtra(makeExtraData(config.Miner.ExtraData))

	// ---- Gas Oracle ----
	gpoParams := config.GPO
	if gpoParams.Default == nil {
		gpoParams.Default = config.Miner.GasPrice
	}
	oracle := gasprice.NewOracle(eth.APIBackend, gpoParams)
	eth.APIBackend.SetOracle(oracle)

	eth.dialCandidates, err = eth.setupDiscovery()
	if err != nil {
		return nil, err
	}

	eth.netRPCService = ethapi.NewPublicNetAPI(eth.p2pServer, eth.NetVersion())
	stack.RegisterAPIs(eth.APIs())
	stack.RegisterProtocols(eth.Protocols())
	stack.RegisterLifecycle(eth)
	return eth, nil
}

func makeExtraData(extra []byte) []byte {
	if len(extra) == 0 {
		// create default extradata
		extra, _ = rlp.EncodeToBytes([]interface{}{
			uint(params.VersionMajor<<16 | params.VersionMinor<<8 | params.VersionPatch),
			"geth",
			runtime.Version(),
			runtime.GOOS,
		})
	}
	if uint64(len(extra)) > params.MaximumExtraDataSize {
		log.Warn("Miner extra data exceed limit", "extra", hexutil.Bytes(extra), "limit", params.MaximumExtraDataSize)
		extra = nil
	}
	return extra
}

// CreateConsensusEngine creates the required type of consensus engine instance for an Ethereum service.
func CreateConsensusEngine(stack *node.Node, chainConfig *params.ChainConfig, config *ethash.Config, notify []string, noverify bool, db ethdb.Database, ethAPI *ethapi.PublicBlockChainAPI) consensus.Engine {
	// If proof-of-authority is requested, set it up
	if chainConfig.Clique != nil {
		return clique.New(chainConfig.Clique, db)
	}
	if chainConfig.Sonium != nil {
		return sonium.New(chainConfig, db, ethAPI)
	}
	// Otherwise assume proof-of-work
	switch config.PowMode {
	case ethash.ModeFake:
		log.Warn("Ethash used in fake mode")
		return ethash.NewFaker()
	case ethash.ModeTest:
		log.Warn("Ethash used in test mode")
		return ethash.NewTester(nil, noverify)
	case ethash.ModeShared:
		log.Warn("Ethash used in shared mode")
		return ethash.NewShared()
	default:
		engine := ethash.New(ethash.Config{
			CacheDir:         stack.ResolvePath(config.CacheDir),
			CachesInMem:      config.CachesInMem,
			CachesOnDisk:     config.CachesOnDisk,
			CachesLockMmap:   config.CachesLockMmap,
			DatasetDir:       config.DatasetDir,
			DatasetsInMem:    config.DatasetsInMem,
			DatasetsOnDisk:   config.DatasetsOnDisk,
			DatasetsLockMmap: config.DatasetsLockMmap,
		}, notify, noverify)
		engine.SetThreads(-1) // Disable CPU mining
		return engine
	}
}

// APIs return the collection of RPC services the ethereum package offers.
func (s *Ethereum) APIs() []rpc.API {
	// Core JSON-RPC APIs from internal/ethapi (eth, txpool, personal, debug, preconf, etc.)
	apis := ethapi.GetAPIs(s.APIBackend)

	// Append any APIs exposed explicitly by the consensus engine
	apis = append(apis, s.engine.APIs(s.BlockChain())...)

	// Local node APIs
	local := []rpc.API{
		{
			Namespace: "eth",
			Version:   "1.0",
			Service:   downloader.NewPublicDownloaderAPI(s.protocolManager.downloader, s.eventMux),
			Public:    true,
		},
		{
			Namespace: "eth",
			Version:   "1.0",
			Service:   filters.NewPublicFilterAPI(s.APIBackend, false),
			Public:    true,
		},
		{
			Namespace: "miner",
			Version:   "1.0",
			Service:   NewPrivateMinerAPI(s),
			Public:    false,
		},
		{
			Namespace: "admin",
			Version:   "1.0",
			Service:   NewPrivateAdminAPI(s),
			Public:    false,
		},
		{
			Namespace: "net",
			Version:   "1.0",
			Service:   s.netRPCService,
			Public:    true,
		},
		{
   		 	Namespace: "debug",
    		Version:   "1.0",
    		Service:   NewPublicDebugAPI(s.APIBackend),
    		Public:    true,
		},
		{
    		Namespace: "debug",
    		Version:   "1.0",
    		Service:   NewPrivateDebugAPI(s.APIBackend),
    		Public:    false,
		},
		 {
            Namespace: "eth",
            Version:   "1.0",
            Service:   NewPublicEthereumAPI(s),
            Public:    true,
        },
        {
            Namespace: "eth",
            Version:   "1.0",
            Service:   NewPublicMinerAPI(s),
            Public:    true,
        },
      
    }

	return append(apis, local...)
}

// BlockByNumberOrHash implements flexible block lookup for ethapi.Backend
func (s *Ethereum) BlockByNumberOrHash(ctx context.Context, bh rpc.BlockNumberOrHash) (*types.Block, error) {
    if num, ok := bh.Number(); ok {
        return s.BlockByNumber(ctx, num)
    }
    if hash, ok := bh.Hash(); ok {
        hdr := s.blockchain.GetHeaderByHash(hash)
        if hdr == nil {
            return nil, nil
        }
        return s.blockchain.GetBlock(hash, hdr.Number.Uint64()), nil
    }
    return nil, errors.New("invalid BlockNumberOrHash")
}




func (s *Ethereum) ResetWithGenesisBlock(gb *types.Block) {
	s.blockchain.ResetWithGenesisBlock(gb)
}

func (s *Ethereum) Etherbase() (common.Address, error) {
	s.lock.RLock()
	etherbase := s.etherbase
	s.lock.RUnlock()

	if etherbase != (common.Address{}) {
		return etherbase, nil
	}
	if wallets := s.AccountManager().Wallets(); len(wallets) > 0 {
		if accounts := wallets[0].Accounts(); len(accounts) > 0 {
			etherbase = accounts[0].Address

			s.lock.Lock()
			s.etherbase = etherbase
			s.lock.Unlock()

			log.Info("Etherbase automatically configured", "address", etherbase)
			return etherbase, nil
		}
	}
	return common.Address{}, fmt.Errorf("etherbase must be explicitly specified")
}

// isLocalBlock checks whether the specified block is mined by local miner accounts.
func (s *Ethereum) isLocalBlock(block *types.Block) bool {
	author, err := s.engine.Author(block.Header())
	if err != nil {
		log.Warn("Failed to retrieve block author", "number", block.NumberU64(), "hash", block.Hash(), "err", err)
		return false
	}
	// Check whether the given address is etherbase.
	s.lock.RLock()
	etherbase := s.etherbase
	s.lock.RUnlock()
	if author == etherbase {
		return true
	}
	// Check whether the given address is specified by `txpool.locals` CLI flag.
	for _, account := range s.config.TxPool.Locals {
		if account == author {
			return true
		}
	}
	return false
}

// shouldPreserve checks whether we should preserve the given block during the chain reorg
// depending on whether the author of block is a local account.
func (s *Ethereum) shouldPreserve(block *types.Block) bool {
	if _, ok := s.engine.(*clique.Clique); ok {
		return false
	}
	if _, ok := s.engine.(*sonium.Sonium); ok {
		return false
	}
	return s.isLocalBlock(block)
}

// SetEtherbase sets the mining reward address.
func (s *Ethereum) SetEtherbase(etherbase common.Address) {
	s.lock.Lock()
	s.etherbase = etherbase
	s.lock.Unlock()

	s.miner.SetEtherbase(etherbase)
}

// StartMining starts the miner with the given number of CPU threads.
func (s *Ethereum) StartMining(threads int) error {
	// Update the thread count within the consensus engine
	type threaded interface {
		SetThreads(threads int)
	}
	if th, ok := s.engine.(threaded); ok {
		log.Info("Updated mining threads", "threads", threads)
		if threads == 0 {
			threads = -1 // Disable the miner from within
		}
		th.SetThreads(threads)
	}
	// If the miner was not running, initialize it
	if !s.IsMining() {
		// Propagate the initial price point to the transaction pool
		s.lock.RLock()
		price := s.gasPrice
		s.lock.RUnlock()
		s.txPool.SetGasPrice(price)

		// Configure the local mining address
		eb, err := s.Etherbase()
		if err != nil {
			log.Error("Cannot start mining without etherbase", "err", err)
			return fmt.Errorf("etherbase missing: %v", err)
		}
		if clique, ok := s.engine.(*clique.Clique); ok {
			wallet, err := s.accountManager.Find(accounts.Account{Address: eb})
			if wallet == nil || err != nil {
				log.Error("Etherbase account unavailable locally", "err", err)
				return fmt.Errorf("signer missing: %v", err)
			}
			clique.Authorize(eb, wallet.SignData)
		}
		if son, ok := s.engine.(*sonium.Sonium); ok {
			wallet, err := s.accountManager.Find(accounts.Account{Address: eb})
			if wallet == nil || err != nil {
				log.Error("Etherbase account unavailable locally", "err", err)
				return fmt.Errorf("signer missing: %v", err)
			}
			son.Authorize(eb, wallet.SignData, wallet.SignTx)
		}
		// If mining is started, we can disable the transaction rejection mechanism
		// introduced to speed sync times.
		atomic.StoreUint32(&s.protocolManager.acceptTxs, 1)

		go s.miner.Start(eb)
	}
	return nil
}

// StopMining terminates the miner.
func (s *Ethereum) StopMining() {
	// Update the thread count within the consensus engine
	type threaded interface {
		SetThreads(threads int)
	}
	if th, ok := s.engine.(threaded); ok {
		th.SetThreads(-1)
	}
	// Stop the block creating itself
	s.miner.Stop()
}

func (s *Ethereum) CurrentHeader() *types.Header {
    return s.blockchain.CurrentHeader()
}

func (s *Ethereum) IsMining() bool      { return s.miner.Mining() }
func (s *Ethereum) Miner() *miner.Miner { return s.miner }

func (s *Ethereum) AccountManager() *accounts.Manager { return s.accountManager }
//func (s *Ethereum) BlockChain() *core.BlockChain      { return s.blockchain }
func (s *Ethereum) TxPool() *core.TxPool              { return s.txPool }
func (s *Ethereum) EventMux() *event.TypeMux          { return s.eventMux }
func (s *Ethereum) Engine() consensus.Engine          { return s.engine }
func (s *Ethereum) ChainDb() ethdb.Database           { return s.chainDb }
func (s *Ethereum) IsListening() bool                 { return true } // Always listening
func (s *Ethereum) EthVersion() int                   { return int(ProtocolVersions[0]) }
func (s *Ethereum) NetVersion() uint64                { return s.networkID }
func (s *Ethereum) Downloader() *downloader.Downloader {
	return s.protocolManager.downloader
}
func (s *Ethereum) Synced() bool                     { return atomic.LoadUint32(&s.protocolManager.acceptTxs) == 1 }
func (s *Ethereum) ArchiveMode() bool                { return s.config.NoPruning }
func (s *Ethereum) BloomIndexer() *core.ChainIndexer { return s.bloomIndexer }

// Protocols returns all the currently configured network protocols to start.
func (s *Ethereum) Protocols() []p2p.Protocol {
	protos := make([]p2p.Protocol, len(ProtocolVersions))
	for i, vsn := range ProtocolVersions {
		protos[i] = s.protocolManager.makeProtocol(vsn)
		protos[i].Attributes = []enr.Entry{s.currentEthEntry()}
		protos[i].DialCandidates = s.dialCandidates
	}
	return protos
}

// Start implements node.Lifecycle, starting all internal goroutines needed by the Ethereum protocol implementation.
func (s *Ethereum) Start() error {
	s.startEthEntryUpdate(s.p2pServer.LocalNode())

	// Start the bloom bits servicing goroutines
	s.startBloomHandlers(params.BloomBitsBlocks)

	// Figure out a max peers count based on the server limits
	maxPeers := s.p2pServer.MaxPeers
	if s.config.LightServ > 0 {
		if s.config.LightPeers >= s.p2pServer.MaxPeers {
			return fmt.Errorf("invalid peer config: light peer count (%d) >= total peer count (%d)", s.config.LightPeers, s.p2pServer.MaxPeers)
		}
		maxPeers -= s.config.LightPeers
	}
	// Start the networking layer and the light server if requested
	s.protocolManager.Start(maxPeers)
	return nil
}

// Stop implements node.Lifecycle, terminating all internal goroutines used by the Ethereum protocol.
func (s *Ethereum) Stop() error {
	// Stop all the peer-related stuff first.
	s.protocolManager.Stop()

	// Then stop everything else.
	s.bloomIndexer.Close()
	close(s.closeBloomHandler)
	s.txPool.Stop()
	s.miner.Stop()
	s.blockchain.Stop()
	s.engine.Close()
	s.chainDb.Close()
	s.eventMux.Stop()
	return nil
}

///new
// ---- Implementation required by ethapi.Backend ----

func (s *Ethereum) StateAndHeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*state.StateDB, *types.Header, error) {
	return s.APIBackend.StateAndHeaderByNumber(ctx, number)
}

func (s *Ethereum) StateAndHeaderByNumberOrHash(ctx context.Context, bh rpc.BlockNumberOrHash) (*state.StateDB, *types.Header, error) {
	return s.APIBackend.StateAndHeaderByNumberOrHash(ctx, bh)
}

//new
// === ethapi.Backend interface compatibility ===

func (s *Ethereum) BlockChain() *core.BlockChain {
    return s.blockchain
}

func (s *Ethereum) BlockByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Block, error) {
    switch number {
    case rpc.PendingBlockNumber:
        return s.miner.PendingBlock(), nil
    case rpc.LatestBlockNumber:
        return s.blockchain.CurrentBlock(), nil
    default:
        return s.blockchain.GetBlockByNumber(uint64(number)), nil
    }
}

func (s *Ethereum) BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error) {
    return s.blockchain.GetBlockByHash(hash), nil
}

func (s *Ethereum) HeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Header, error) {
    switch number {
    case rpc.PendingBlockNumber:
        return s.miner.PendingBlock().Header(), nil
    case rpc.LatestBlockNumber:
        return s.blockchain.CurrentBlock().Header(), nil
    default:
        return s.blockchain.GetHeaderByNumber(uint64(number)), nil
    }
}

func (s *Ethereum) HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error) {
    return s.blockchain.GetHeaderByHash(hash), nil
}

func (s *Ethereum) BloomStatus() (uint64, uint64) {
    sections, _, _ := s.bloomIndexer.Sections()
    return params.BloomBitsBlocks, sections
}

func (s *Ethereum) ServiceFilter(ctx context.Context, session *bloombits.MatcherSession) {
    for i := 0; i < 16; i++ { // 16 = bloomFilterThreads in internal/ethapi
        go session.Multiplex(16, 100*time.Millisecond, s.bloomRequests)
    }
}

func (s *Ethereum) ProtocolVersion() int {
    return int(ProtocolVersions[0])
}

func (s *Ethereum) RPCGasCap() uint64 {
    return s.config.RPCGasCap
}

func (s *Ethereum) RPCTxFeeCap() float64 {
    return s.config.RPCTxFeeCap
}

func (s *Ethereum) ChainConfig() *params.ChainConfig {
    // BlockChain has Config() in go-ethereum
    return s.blockchain.Config()
}

func (s *Ethereum) NodeInfo() interface{} {
    if s.p2pServer != nil {
        return s.p2pServer.NodeInfo()
    }
    return nil
}

func (s *Ethereum) CurrentBlock() *types.Block {
    return s.blockchain.CurrentBlock()
}

func (s *Ethereum) ExtRPCEnabled() bool {
    return s.config.ExtRPCEnabled
}

// GetEVM implements ethapi.Backend. It returns an EVM instance with the
// given block and state, used by the debug/tracer APIs.
func (s *Ethereum) GetEVM(
    msg core.Message,
    header *types.Header,
    statedb *state.StateDB,
    cfg vm.Config,
) (*vm.EVM, error) {

    // Prepare block and tx context
    blockCtx := core.NewEVMBlockContext(header, s.blockchain, nil)
    txCtx := core.NewEVMTxContext(msg)

    return vm.NewEVM(blockCtx, txCtx, statedb, s.blockchain.Config(), cfg), nil
}

// GetLogs implements ethapi.Backend.
// It returns all logs matching the given block hash and filter.
// GetLogs implements ethapi.Backend.
// It returns logs for the given block hash grouped per transaction.
func (eth *Ethereum) GetLogs(ctx context.Context, hash common.Hash) ([][]*types.Log, error) {
	block := eth.blockchain.GetBlockByHash(hash)
	if block == nil {
		return nil, fmt.Errorf("block %s not found", hash.Hex())
	}

	receipts := eth.blockchain.GetReceiptsByHash(hash)
	if receipts == nil {
		return nil, fmt.Errorf("receipts for block %s not found", hash.Hex())
	}

	logs := make([][]*types.Log, len(receipts))
	for i, receipt := range receipts {
		logs[i] = receipt.Logs
	}
	return logs, nil
}

func (eth *Ethereum) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
    if eth.txPool == nil {
        return 0, errors.New("txpool not initialized")
    }
    _, err := eth.txPool.Pending()
    if err != nil {
        return 0, err
    }
    return eth.txPool.Nonce(addr), nil
}


func (eth *Ethereum) GetPoolTransaction(hash common.Hash) *types.Transaction {
    if eth.txPool == nil {
        return nil
    }
    return eth.txPool.Get(hash)
}

func (eth *Ethereum) GetPoolTransactions() (types.Transactions, error) {
    if eth.txPool == nil {
        return nil, errors.New("txpool not initialized")
    }
    pend, err := eth.txPool.Pending()
    if err != nil {
        return nil, err
    }
    var txs types.Transactions
    for _, addrTxs := range pend { // flatten map into slice
        txs = append(txs, addrTxs...)
    }
    return txs, nil
}


func (eth *Ethereum) GetReceipts(ctx context.Context, hash common.Hash) (types.Receipts, error) {
    block := eth.blockchain.GetBlockByHash(hash)
    if block == nil {
        return nil, fmt.Errorf("block %#x not found", hash)
    }

    receipts := rawdb.ReadReceipts(eth.chainDb, block.Hash(), block.NumberU64(), eth.blockchain.Config())
    if receipts == nil {
        return nil, fmt.Errorf("receipts not found for block %#x", hash)
    }
    return receipts, nil
}

func (eth *Ethereum) GetTd(ctx context.Context, hash common.Hash) *big.Int {
    block := eth.blockchain.GetBlockByHash(hash)
    if block == nil {
        return nil
    }
    return eth.blockchain.GetTd(hash, block.NumberU64())
}

// GetTransaction implements ethapi.Backend.
// Returns (tx, blockHash, blockNumber, txIndex, error)
func (eth *Ethereum) GetTransaction(ctx context.Context, hash common.Hash) (*types.Transaction, common.Hash, uint64, uint64, error) {
    tx, blockHash, blockNumber, txIndex := rawdb.ReadTransaction(eth.ChainDb(), hash)
    if tx == nil {
        return nil, common.Hash{}, 0, 0, errors.New("transaction not found")
    }
    return tx, blockHash, blockNumber, txIndex, nil
}

// HeaderByNumberOrHash implements ethapi.Backend.
func (eth *Ethereum) HeaderByNumberOrHash(ctx context.Context, input rpc.BlockNumberOrHash) (*types.Header, error) {
    // Case 1: block number is provided
    if input.BlockNumber != nil {
        number := uint64(*input.BlockNumber)

        // Pending block can be requested explicitly
        if input.RequireCanonical && number == rpc.PendingBlockNumber.Int64() {
            if eth.miner != nil && eth.miner.PendingBlock() != nil {
                return eth.miner.PendingBlock().Header(), nil
            }
            return nil, errors.New("pending block not available")
        }

        header := eth.blockchain.GetHeaderByNumber(number)
        if header == nil {
            return nil, errors.New("header not found")
        }
        return header, nil
    }

    // Case 2: block hash is provided
    hash, ok := input.Hash()
    if !ok {
        return nil, errors.New("invalid BlockNumberOrHash")
    }

    header := eth.blockchain.GetHeaderByHash(hash)
    if header == nil {
        return nil, errors.New("header not found")
    }
    return header, nil
}


func (eth *Ethereum) LoadPreconfReceipt(hash common.Hash) *preconf.PreconfReceipt {
    // If your chain does not support preconf receipts yet, return nil
    return nil
}




