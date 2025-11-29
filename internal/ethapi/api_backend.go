// internal/ethapi/api_backend.go
package ethapi

import (
	"context"
	"errors"
	"math/big"
	"sync"

	"github.com/sesafoundation/sesn/accounts"
	"github.com/sesafoundation/sesn/common"
	"github.com/sesafoundation/sesn/consensus"
	"github.com/sesafoundation/sesn/core"
	"github.com/sesafoundation/sesn/core/bloombits"
	"github.com/sesafoundation/sesn/core/state"
	"github.com/sesafoundation/sesn/core/types"
	"github.com/sesafoundation/sesn/eth/downloader"
	"github.com/sesafoundation/sesn/eth/gasprice"
	"github.com/sesafoundation/sesn/ethdb"
	"github.com/sesafoundation/sesn/event"
	"github.com/sesafoundation/sesn/miner"
	"github.com/sesafoundation/sesn/params"
	"github.com/sesafoundation/sesn/preconf"
	"github.com/sesafoundation/sesn/rpc"
	"github.com/sesafoundation/sesn/core/vm"
)

type EthAPIBackend struct {
	backend Backend            // must remain interface
	extRPCEnabled bool
	gpo *gasprice.Oracle

	preconfMu       sync.RWMutex
	preconfReceipts map[common.Hash]*preconf.PreconfReceipt
	preconfFeed     event.Feed
}

func NewEthAPIBackend(extRPC bool, b Backend) *EthAPIBackend {
	return &EthAPIBackend{
		backend:        b,
		extRPCEnabled:  extRPC,
		preconfReceipts: make(map[common.Hash]*preconf.PreconfReceipt),
	}
}

func (b *EthAPIBackend) SetOracle(o *gasprice.Oracle) { b.gpo = o }

//
// ---------------- General / Sync / DB ----------------
//
func (b *EthAPIBackend) Downloader() *downloader.Downloader { return b.backend.Downloader() }
func (b *EthAPIBackend) ProtocolVersion() int               { return b.backend.ProtocolVersion() }
func (b *EthAPIBackend) ChainDb() ethdb.Database            { return b.backend.ChainDb() }
func (b *EthAPIBackend) AccountManager() *accounts.Manager  { return b.backend.AccountManager() }
func (b *EthAPIBackend) EventMux() *event.TypeMux           { return b.backend.EventMux() }
//func (b *EthAPIBackend) ExtRPCEnabled() bool                { return b.extRPCEnabled }
func (b *EthAPIBackend) RPCGasCap() uint64                  { return b.backend.RPCGasCap() }
func (b *EthAPIBackend) RPCTxFeeCap() float64               { return b.backend.RPCTxFeeCap() }
func (b *EthAPIBackend) ChainConfig() *params.ChainConfig   { return b.backend.ChainConfig() }
func (b *EthAPIBackend) Engine() consensus.Engine           { return b.backend.Engine() }

func (b *EthAPIBackend) SuggestPrice(ctx context.Context) (*big.Int, error) {
	if b.gpo == nil {
		return nil, errors.New("gas price oracle not initialised")
	}
	return b.gpo.SuggestPrice(ctx)
}

//
// ---------------- Blockchain ----------------
//
func (b *EthAPIBackend) SetHead(n uint64)                                 { b.backend.SetHead(n) }
//func (b *EthAPIBackend) CurrentHeader() *types.Header                     { return b.backend.CurrentHeader() }
func (b *EthAPIBackend) CurrentBlock() *types.Block                       { return b.backend.CurrentBlock() }
func (b *EthAPIBackend) HeaderByNumber(ctx context.Context, n rpc.BlockNumber) (*types.Header, error) {
	return b.backend.HeaderByNumber(ctx, n)
}
func (b *EthAPIBackend) HeaderByHash(ctx context.Context, h common.Hash) (*types.Header, error) {
	return b.backend.HeaderByHash(ctx, h)
}
func (b *EthAPIBackend) HeaderByNumberOrHash(ctx context.Context, bh rpc.BlockNumberOrHash) (*types.Header, error) {
	return b.backend.HeaderByNumberOrHash(ctx, bh)
}
func (b *EthAPIBackend) BlockByNumber(ctx context.Context, n rpc.BlockNumber) (*types.Block, error) {
	return b.backend.BlockByNumber(ctx, n)
}
func (b *EthAPIBackend) BlockByHash(ctx context.Context, h common.Hash) (*types.Block, error) {
	return b.backend.BlockByHash(ctx, h)
}
func (b *EthAPIBackend) BlockByNumberOrHash(ctx context.Context, bh rpc.BlockNumberOrHash) (*types.Block, error) {
	return b.backend.BlockByNumberOrHash(ctx, bh)
}
func (b *EthAPIBackend) StateAndHeaderByNumber(ctx context.Context, n rpc.BlockNumber) (*state.StateDB, *types.Header, error) {
	return b.backend.StateAndHeaderByNumber(ctx, n)
}
//func (b *EthAPIBackend) StateAndHeaderByNumberOrHash(ctx context.Context, bh rpc.BlockNumberOrHash) (*state.StateDB, *types.Header, error) {
//	return b.backend.StateAndHeaderByNumberOrHash(ctx, bh)
//}
func (b *EthAPIBackend) GetReceipts(ctx context.Context, hash common.Hash) (types.Receipts, error) {
	return b.backend.GetReceipts(ctx, hash)
}
func (b *EthAPIBackend) GetLogs(ctx context.Context, hash common.Hash) ([][]*types.Log, error) {
	return b.backend.GetLogs(ctx, hash)
}
func (b *EthAPIBackend) GetTd(ctx context.Context, hash common.Hash) *big.Int {
	return b.backend.GetTd(ctx, hash)
}


//
// ---------------- Subscriptions ----------------
//
func (b *EthAPIBackend) SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription {
	return b.backend.SubscribeChainEvent(ch)
}
func (b *EthAPIBackend) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return b.backend.SubscribeChainHeadEvent(ch)
}
func (b *EthAPIBackend) SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription {
	return b.backend.SubscribeChainSideEvent(ch)
}
func (b *EthAPIBackend) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return b.backend.SubscribeLogsEvent(ch)
}
func (b *EthAPIBackend) SubscribePendingLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return b.backend.SubscribePendingLogsEvent(ch)
}
func (b *EthAPIBackend) SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription {
	return b.backend.SubscribeRemovedLogsEvent(ch)
}

//
// ---------------- Tx pool ----------------
//
func (b *EthAPIBackend) SendTx(ctx context.Context, tx *types.Transaction) error {
	return b.backend.SendTx(ctx, tx)
}
func (b *EthAPIBackend) GetTransaction(ctx context.Context, h common.Hash) (*types.Transaction, common.Hash, uint64, uint64, error) {
	return b.backend.GetTransaction(ctx, h)
}
func (b *EthAPIBackend) GetPoolTransactions() (types.Transactions, error) {
	return b.backend.GetPoolTransactions()
}
func (b *EthAPIBackend) GetPoolTransaction(h common.Hash) *types.Transaction {
	return b.backend.GetPoolTransaction(h)
}
func (b *EthAPIBackend) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return b.backend.GetPoolNonce(ctx, addr)
}
func (b *EthAPIBackend) Stats() (int, int) {
	return b.backend.Stats()
}
func (b *EthAPIBackend) TxPoolContent() (map[common.Address]types.Transactions, map[common.Address]types.Transactions) {
	return b.backend.TxPoolContent()
}
func (b *EthAPIBackend) SubscribeNewTxsEvent(ch chan<- core.NewTxsEvent) event.Subscription {
	return b.backend.SubscribeNewTxsEvent(ch)
}

//
// ---------------- Filters ----------------
//
func (b *EthAPIBackend) BloomStatus() (uint64, uint64) {
	return b.backend.BloomStatus()
}
func (b *EthAPIBackend) ServiceFilter(ctx context.Context, session *bloombits.MatcherSession) {
	b.backend.ServiceFilter(ctx, session)
}

//
// ---------------- Mining ----------------
//
func (b *EthAPIBackend) Miner() *miner.Miner               { return b.backend.Miner() }
func (b *EthAPIBackend) StartMining(t int) error           { return b.backend.StartMining(t) }

//
// ---------------- PRECONF ----------------
//
func (b *EthAPIBackend) StorePreconfReceipt(h common.Hash, r *preconf.PreconfReceipt) {
	b.preconfMu.Lock()
	b.preconfReceipts[h] = r
	b.preconfMu.Unlock()
	b.preconfFeed.Send(r)
}
func (b *EthAPIBackend) LoadPreconfReceipt(h common.Hash) *preconf.PreconfReceipt {
	b.preconfMu.RLock()
	r := b.preconfReceipts[h]
	b.preconfMu.RUnlock()
	return r
}
func (b *EthAPIBackend) PreconfSubscribe(ch chan *preconf.PreconfReceipt) event.Subscription {
	return b.preconfFeed.Subscribe(ch)
}

// EthVersion returns the ETH protocol version for RPC
func (b *EthAPIBackend) EthVersion() int {
    return b.backend.EthVersion()
}

func (b *EthAPIBackend) NetVersion() uint64 {
    return b.backend.NetVersion()
}

func (b *EthAPIBackend) NodeInfo() interface{} {
    return b.backend.NodeInfo()
}

func (b *EthAPIBackend) BlockChain() *core.BlockChain {
    return b.backend.BlockChain()
}

func (b *EthAPIBackend) StateAndHeaderByNumberOrHash(ctx context.Context, bh rpc.BlockNumberOrHash) (*state.StateDB, *types.Header, error) {

    // If BlockNumber is set
    if bh.BlockNumber != nil {
        return b.StateAndHeaderByNumber(ctx, *bh.BlockNumber)
    }

    // If BlockHash is set
    if h, ok := bh.Hash(); ok {
        block, err := b.BlockByHash(ctx, h)
        if err != nil {
            return nil, nil, err
        }
        if block == nil {
            return nil, nil, errors.New("block not found")
        }

        st, err := b.backend.BlockChain().StateAt(block.Root())
        if err != nil {
            return nil, nil, err
        }
        return st, block.Header(), nil
    }

    // Neither number nor hash
    return nil, nil, errors.New("invalid BlockNumberOrHash: neither blockNumber nor blockHash specified")

}


func (b *EthAPIBackend) TxPool() *core.TxPool {
    return b.backend.TxPool()
}

func (b *EthAPIBackend) ExtRPCEnabled() bool {
    return b.backend.ExtRPCEnabled()
}

func (b *EthAPIBackend) GetEVM(
    msg core.Message,
    header *types.Header,
    statedb *state.StateDB,
    cfg vm.Config,
) (*vm.EVM, error) {

    // delegate to backend (Ethereum)
    return b.backend.GetEVM(msg, header, statedb, cfg)
}

