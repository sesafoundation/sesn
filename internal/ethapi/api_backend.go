// Copyright 2015 The go-ethereum Authors
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

package ethapi

import (
	"context"
	"errors"
	"math/big"
	"sync"
	"time"
	"github.com/sesafoundation/sesn/accounts"
	"github.com/sesafoundation/sesn/common"
	"github.com/sesafoundation/sesn/consensus"
	"github.com/sesafoundation/sesn/core"
	"github.com/sesafoundation/sesn/core/bloombits"
	"github.com/sesafoundation/sesn/core/rawdb"
	"github.com/sesafoundation/sesn/core/state"
	"github.com/sesafoundation/sesn/core/types"
	"github.com/sesafoundation/sesn/core/vm"
	"github.com/sesafoundation/sesn/eth/downloader"
	"github.com/sesafoundation/sesn/eth/gasprice"
	"github.com/sesafoundation/sesn/ethdb"
	"github.com/sesafoundation/sesn/event"
	"github.com/sesafoundation/sesn/miner"
	"github.com/sesafoundation/sesn/params"
	"github.com/sesafoundation/sesn/rpc"
	"github.com/sesafoundation/sesn/preconf"
	// eth "github.com/sesafoundation/sesn/eth"
)

const (
    bloomFilterThreads  = 16
    bloomRetrievalBatch = 16
    bloomRetrievalWait  = 50 * time.Millisecond
)

type EthAPIBackend struct {
    extRPCEnabled   bool
    backend         Backend          // ← pointer to full Ethereum backend (interface)
    gpo             *gasprice.Oracle // injected later

    // PRECONF
    preconfMu       sync.RWMutex
    preconfReceipts map[common.Hash]*preconf.PreconfReceipt
    preconfFeed     event.Feed
}



func NewEthAPIBackend(ext bool, eth *Ethereum) *EthAPIBackend {
    return &EthAPIBackend{
        extRPCEnabled:   ext,
        backend:            eth,
		preconfReceipts: make(map[common.Hash]*preconf.PreconfReceipt),
    }
}

func (b *EthAPIBackend) BlockChain() *core.BlockChain         { return b.backend.BlockChain() }
//func (b *EthAPIBackend) TxPool() *core.TxPool                  { return b.backend.TxPool() }
//func (b *EthAPIBackend) AccountManager() *accounts.Manager     { return b.backend.AccountManager() }
//func (b *EthAPIBackend) ProtocolVersion() int                  { return b.backend.ProtocolVersion() }
//func (b *EthAPIBackend) Miner() *miner.Miner                   { return b.backend.Miner() }
func (b *EthAPIBackend) ProtocolManager() interface{}          { return b.backend.ProtocolManager() }
func (b *EthAPIBackend) NetVersion() uint64                    { return b.backend.NetVersion() }
func (b *EthAPIBackend) ChainConfig() *params.ChainConfig      { return b.backend.ChainConfig() }
func (b *EthAPIBackend) NodeInfo() interface{}                 { return b.backend.NodeInfo() }
//func (b *EthAPIBackend) SubscribeChainHead(ch chan<- core.ChainHeadEvent) event.Subscription {
//    return b.backend.SubscribeChainHead(ch)
//}





// ---------------- Core chain access ----------------

func (b *EthAPIBackend) CurrentBlock() *types.Block {
	return b.backend.BlockChain().CurrentBlock()
}

func (b *EthAPIBackend) SetHead(number uint64) {
	// cancel any in-progress download, then move head
	if dl := b.backend.Downloader(); dl != nil {
		dl.Cancel()
	}
	b.backend.BlockChain().SetHead(number)
}

func (b *EthAPIBackend) HeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Header, error) {
	// Pending block is only known by the miner
	if number == rpc.PendingBlockNumber {
		block := b.backend.Miner().PendingBlock()
		return block.Header(), nil
	}
	// Otherwise resolve and return the block
	if number == rpc.LatestBlockNumber {
		return b.backend.BlockChain().CurrentBlock().Header(), nil
	}
	return b.backend.BlockChain().GetHeaderByNumber(uint64(number)), nil
}

func (b *EthAPIBackend) HeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Header, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return b.HeaderByNumber(ctx, blockNr)
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header := b.backend.BlockChain().GetHeaderByHash(hash)
		if header == nil {
			return nil, errors.New("header for hash not found")
		}
		if blockNrOrHash.RequireCanonical && b.backend.BlockChain().GetCanonicalHash(header.Number.Uint64()) != hash {
			return nil, errors.New("hash is not currently canonical")
		}
		return header, nil
	}
	return nil, errors.New("invalid arguments; neither block nor hash specified")
}

func (b *EthAPIBackend) HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error) {
	return b.backend.BlockChain().GetHeaderByHash(hash), nil
}

func (b *EthAPIBackend) BlockByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Block, error) {
	// Pending block is only known by the miner
	if number == rpc.PendingBlockNumber {
		block := b.backend.Miner().PendingBlock()
		return block, nil
	}
	// Otherwise resolve and return the block
	if number == rpc.LatestBlockNumber {
		return b.backend.BlockChain().CurrentBlock(), nil
	}
	return b.backend.BlockChain().GetBlockByNumber(uint64(number)), nil
}

func (b *EthAPIBackend) BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error) {
	return b.backend.BlockChain().GetBlockByHash(hash), nil
}

func (b *EthAPIBackend) BlockByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Block, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return b.BlockByNumber(ctx, blockNr)
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header := b.backend.BlockChain().GetHeaderByHash(hash)
		if header == nil {
			return nil, errors.New("header for hash not found")
		}
		if blockNrOrHash.RequireCanonical && b.backend.BlockChain().GetCanonicalHash(header.Number.Uint64()) != hash {
			return nil, errors.New("hash is not currently canonical")
		}
		block := b.backend.BlockChain().GetBlock(hash, header.Number.Uint64())
		if block == nil {
			return nil, errors.New("header found, but block body is missing")
		}
		return block, nil
	}
	return nil, errors.New("invalid arguments; neither block nor hash specified")
}

// ---------------- State access ----------------

func (b *EthAPIBackend) StateAndHeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*state.StateDB, *types.Header, error) {
	// Pending state is only known by the miner
	if number == rpc.PendingBlockNumber {
		block, state := b.backend.Miner().Pending()
		return state, block.Header(), nil
	}
	// Otherwise resolve the block number and return its state
	header, err := b.HeaderByNumber(ctx, number)
	if err != nil {
		return nil, nil, err
	}
	if header == nil {
		return nil, nil, errors.New("header not found")
	}
	stateDb, err := b.backend.BlockChain().StateAt(header.Root)
	return stateDb, header, err
}

func (b *EthAPIBackend) StateAndHeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*state.StateDB, *types.Header, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return b.StateAndHeaderByNumber(ctx, blockNr)
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header, err := b.HeaderByHash(ctx, hash)
		if err != nil {
			return nil, nil, err
		}
		if header == nil {
			return nil, nil, errors.New("header for hash not found")
		}
		if blockNrOrHash.RequireCanonical && b.backend.BlockChain().GetCanonicalHash(header.Number.Uint64()) != hash {
			return nil, nil, errors.New("hash is not currently canonical")
		}
		stateDb, err := b.backend.BlockChain().StateAt(header.Root)
		return stateDb, header, err
	}
	return nil, nil, errors.New("invalid arguments; neither block nor hash specified")
}

// ---------------- Receipts / logs / TD / EVM ----------------

func (b *EthAPIBackend) GetReceipts(ctx context.Context, hash common.Hash) (types.Receipts, error) {
	return b.backend.BlockChain().GetReceiptsByHash(hash), nil
}

func (b *EthAPIBackend) GetLogs(ctx context.Context, hash common.Hash) ([][]*types.Log, error) {
	receipts := b.backend.BlockChain().GetReceiptsByHash(hash)
	if receipts == nil {
		return nil, nil
	}
	logs := make([][]*types.Log, len(receipts))
	for i, receipt := range receipts {
		logs[i] = receipt.Logs
	}
	return logs, nil
}

func (b *EthAPIBackend) GetTd(ctx context.Context, hash common.Hash) *big.Int {
	return b.backend.BlockChain().GetTdByHash(hash)
}

func (b *EthAPIBackend) GetEVM(ctx context.Context, msg core.Message, state *state.StateDB, header *types.Header) (*vm.EVM, func() error, error) {
	vmError := func() error { return nil }

	txContext := core.NewEVMTxContext(msg)
	blockCtx := core.NewEVMBlockContext(header, b.backend.BlockChain(), nil)
	return vm.NewEVM(blockCtx, txContext, state, b.backend.BlockChain().Config(), *b.backend.BlockChain().GetVMConfig()), vmError, nil
}

// ---------------- Subscriptions ----------------

func (b *EthAPIBackend) SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription {
	return b.backend.BlockChain().SubscribeRemovedLogsEvent(ch)
}

func (b *EthAPIBackend) SubscribePendingLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return b.backend.Miner().SubscribePendingLogs(ch)
}

func (b *EthAPIBackend) SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription {
	return b.backend.BlockChain().SubscribeChainEvent(ch)
}

func (b *EthAPIBackend) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return b.backend.BlockChain().SubscribeChainHeadEvent(ch)
}

func (b *EthAPIBackend) SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription {
	return b.backend.BlockChain().SubscribeChainSideEvent(ch)
}

func (b *EthAPIBackend) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return b.backend.BlockChain().SubscribeLogsEvent(ch)
}

// ---------------- Tx pool access ----------------

func (b *EthAPIBackend) SendTx(ctx context.Context, signedTx *types.Transaction) error {
	return b.backend.TxPool().AddLocal(signedTx)
}

func (b *EthAPIBackend) GetPoolTransactions() (types.Transactions, error) {
	pending, err := b.backend.TxPool().Pending()
	if err != nil {
		return nil, err
	}
	var txs types.Transactions
	for _, batch := range pending {
		txs = append(txs, batch...)
	}
	return txs, nil
}

func (b *EthAPIBackend) GetPoolTransaction(hash common.Hash) *types.Transaction {
	return b.backend.TxPool().Get(hash)
}

func (b *EthAPIBackend) GetTransaction(ctx context.Context, txHash common.Hash) (*types.Transaction, common.Hash, uint64, uint64, error) {
	tx, blockHash, blockNumber, index := rawdb.ReadTransaction(b.backend.ChainDb(), txHash)
	return tx, blockHash, blockNumber, index, nil
}

func (b *EthAPIBackend) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return b.backend.TxPool().Nonce(addr), nil
}

func (b *EthAPIBackend) Stats() (pending int, queued int) {
	return b.backend.TxPool().Stats()
}

func (b *EthAPIBackend) TxPoolContent() (map[common.Address]types.Transactions, map[common.Address]types.Transactions) {
	return b.backend.TxPool().Content()
}

func (b *EthAPIBackend) TxPool() *core.TxPool {
	return b.backend.TxPool()
}

func (b *EthAPIBackend) SubscribeNewTxsEvent(ch chan<- core.NewTxsEvent) event.Subscription {
	return b.backend.TxPool().SubscribeNewTxsEvent(ch)
}

// ---------------- Sync / network / config ----------------

func (b *EthAPIBackend) Downloader() *downloader.Downloader {
	return b.backend.Downloader()
}

func (b *EthAPIBackend) ProtocolVersion() int {
	return b.backend.EthVersion()
}

func (b *EthAPIBackend) SuggestPrice(ctx context.Context) (*big.Int, error) {
	return b.gpo.SuggestPrice(ctx)
}

func (b *EthAPIBackend) ChainDb() ethdb.Database {
	return b.backend.ChainDb()
}

func (b *EthAPIBackend) EventMux() *event.TypeMux {
	return b.backend.EventMux()
}

func (b *EthAPIBackend) AccountManager() *accounts.Manager {
	return b.backend.AccountManager()
}

func (b *EthAPIBackend) ExtRPCEnabled() bool {
	return b.extRPCEnabled
}

func (b *EthAPIBackend) RPCGasCap() uint64 {
	return b.backend.RPCGasCap()
}

func (b *EthAPIBackend) RPCTxFeeCap() float64 {
	return b.backend.RPCTxFeeCap()
}

func (b *EthAPIBackend) BloomStatus() (uint64, uint64) {
	sections, _, _ := b.backend.BloomIndexer().Sections()
	return params.BloomBitsBlocks, sections
}

func (b *EthAPIBackend) ServiceFilter(ctx context.Context, session *bloombits.MatcherSession) {
	for i := 0; i < bloomFilterThreads; i++ {
		go session.Multiplex(bloomRetrievalBatch, bloomRetrievalWait, b.backend.BloomRequests())
	}
}

func (b *EthAPIBackend) Engine() consensus.Engine {
	return b.backend.Engine()
}

func (b *EthAPIBackend) CurrentHeader() *types.Header {
	return b.backend.BlockChain().CurrentHeader()
}

func (b *EthAPIBackend) Miner() *miner.Miner {
	return b.backend.Miner()
}

func (b *EthAPIBackend) StartMining(threads int) error {
	return b.backend.StartMining(threads)
}

// ---------------- PRECONF integration ----------------

// StorePreconfReceipt implements preconfapi.PreconfBackend
func (b *EthAPIBackend) StorePreconfReceipt(h common.Hash, r *preconf.PreconfReceipt) {
	b.preconfMu.Lock()
	b.preconfReceipts[h] = r
	b.preconfMu.Unlock()
	b.preconfFeed.Send(r)
}

// LoadPreconfReceipt implements preconfapi.PreconfBackend
func (b *EthAPIBackend) LoadPreconfReceipt(h common.Hash) *preconf.PreconfReceipt {
	b.preconfMu.RLock()
	r := b.preconfReceipts[h]
	b.preconfMu.RUnlock()
	return r
}

// PreconfSubscribe implements preconfapi.PreconfBackend
func (b *EthAPIBackend) PreconfSubscribe(ch chan *preconf.PreconfReceipt) event.Subscription {
	return b.preconfFeed.Subscribe(ch)
}



