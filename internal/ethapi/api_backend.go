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
    "github.com/sesafoundation/sesn/core/types"
    "github.com/sesafoundation/sesn/core/vm"
    //"github.com/sesafoundation/sesn/core/gasprice"
    "github.com/sesafoundation/sesn/preconf"
    //"github.com/sesafoundation/sesn/downloader"
    "github.com/sesafoundation/sesn/event"
    "github.com/sesafoundation/sesn/log"
    "github.com/sesafoundation/sesn/params"
    "github.com/sesafoundation/sesn/rpc"
    "github.com/sesafoundation/sesn/ethdb"
	"github.com/sesafoundation/sesn/gasprice"
	"github.com/sesafoundation/sesn/downloader"
)

const (
    bloomFilterThreads  = 16
    bloomRetrievalBatch = 16
    bloomRetrievalWait  = 50 * time.Millisecond
)

//
// Backend interface — implemented by *eth.Ethereum
//
type Backend interface {
    BlockChain() *core.BlockChain
    TxPool() *core.TxPool
    Miner() *core.Miner
    Downloader() *downloader.Downloader
    ChainDb() ethdb.Database
    AccountManager() *accounts.Manager
    Engine() consensus.Engine

    // sync / misc
    EventMux() *event.TypeMux
    EthVersion() int
    NetVersion() uint64
    NodeInfo() interface{}
    RPCGasCap() uint64
    RPCTxFeeCap() float64
}

//
// Main API backend wrapper
//
type EthAPIBackend struct {
    backend        Backend
    extRPCEnabled  bool
    gpo            *gasprice.Oracle

    preconfMu       sync.RWMutex
    preconfReceipts map[common.Hash]*preconf.PreconfReceipt
    preconfFeed     event.Feed
}

func NewEthAPIBackend(extRPC bool, backend Backend) *EthAPIBackend {
    return &EthAPIBackend{
        backend:        backend,
        extRPCEnabled:  extRPC,
        preconfReceipts: make(map[common.Hash]*preconf.PreconfReceipt),
    }
}

//
// Gas price oracle
//
func (b *EthAPIBackend) SetOracle(o *gasprice.Oracle) { b.gpo = o }

//
// Chain access
//
func (b *EthAPIBackend) CurrentBlock() *types.Block {
    return b.backend.BlockChain().CurrentBlock()
}

func (b *EthAPIBackend) HeaderByNumber(ctx context.Context, num rpc.BlockNumber) (*types.Header, error) {
    if num == rpc.PendingBlockNumber {
        blk := b.backend.Miner().PendingBlock()
        return blk.Header(), nil
    }
    if num == rpc.LatestBlockNumber {
        return b.backend.BlockChain().CurrentBlock().Header(), nil
    }
    return b.backend.BlockChain().GetHeaderByNumber(uint64(num)), nil
}

func (b *EthAPIBackend) HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error) {
    return b.backend.BlockChain().GetHeaderByHash(hash), nil
}

func (b *EthAPIBackend) BlockByNumber(ctx context.Context, num rpc.BlockNumber) (*types.Block, error) {
    if num == rpc.PendingBlockNumber {
        return b.backend.Miner().PendingBlock(), nil
    }
    if num == rpc.LatestBlockNumber {
        return b.backend.BlockChain().CurrentBlock(), nil
    }
    return b.backend.BlockChain().GetBlockByNumber(uint64(num)), nil
}

func (b *EthAPIBackend) BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error) {
    return b.backend.BlockChain().GetBlockByHash(hash), nil
}

func (b *EthAPIBackend) StateAndHeaderByNumber(ctx context.Context, num rpc.BlockNumber) (*state.StateDB, *types.Header, error) {
    if num == rpc.PendingBlockNumber {
        blk, st := b.backend.Miner().Pending()
        return st, blk.Header(), nil
    }
    hdr, err := b.HeaderByNumber(ctx, num)
    if err != nil || hdr == nil {
        return nil, nil, errors.New("header not found")
    }
    s, err := b.backend.BlockChain().StateAt(hdr.Root)
    return s, hdr, err
}

//
// Receipts / logs / TD
//
func (b *EthAPIBackend) GetReceipts(ctx context.Context, hash common.Hash) (types.Receipts, error) {
    return b.backend.BlockChain().GetReceiptsByHash(hash), nil
}

func (b *EthAPIBackend) GetLogs(ctx context.Context, hash common.Hash) ([][]*types.Log, error) {
    receipts := b.backend.BlockChain().GetReceiptsByHash(hash)
    if receipts == nil { return nil, nil }
    logs := make([][]*types.Log, len(receipts))
    for i, r := range receipts { logs[i] = r.Logs }
    return logs, nil
}

//
// Tx pool
//
func (b *EthAPIBackend) SendTx(ctx context.Context, tx *types.Transaction) error {
    return b.backend.TxPool().AddLocal(tx)
}

func (b *EthAPIBackend) GetPoolTransaction(hash common.Hash) *types.Transaction {
    return b.backend.TxPool().Get(hash)
}

func (b *EthAPIBackend) SubscribeNewTxsEvent(ch chan<- core.NewTxsEvent) event.Subscription {
    return b.backend.TxPool().SubscribeNewTxsEvent(ch)
}

//
// Sync / network
//
func (b *EthAPIBackend) ProtocolVersion() int      { return b.backend.EthVersion() }
func (b *EthAPIBackend) Downloader() *downloader.Downloader { return b.backend.Downloader() }
func (b *EthAPIBackend) AccountManager() *accounts.Manager   { return b.backend.AccountManager() }
func (b *EthAPIBackend) ChainDb() ethdb.Database              { return b.backend.ChainDb() }
func (b *EthAPIBackend) Engine() consensus.Engine              { return b.backend.Engine() }
func (b *EthAPIBackend) ExtRPCEnabled() bool                   { return b.extRPCEnabled }
func (b *EthAPIBackend) RPCGasCap() uint64                     { return b.backend.RPCGasCap() }
func (b *EthAPIBackend) RPCTxFeeCap() float64                  { return b.backend.RPCTxFeeCap() }

//
// -------- PRECONF --------
//
func (b *EthAPIBackend) StorePreconfReceipt(hash common.Hash, r *preconf.PreconfReceipt) {
    b.preconfMu.Lock()
    b.preconfReceipts[hash] = r
    b.preconfMu.Unlock()
    b.preconfFeed.Send(r)
}

func (b *EthAPIBackend) LoadPreconfReceipt(hash common.Hash) *preconf.PreconfReceipt {
    b.preconfMu.RLock()
    r := b.preconfReceipts[hash]
    b.preconfMu.RUnlock()
    return r
}

func (b *EthAPIBackend) PreconfSubscribe(ch chan *preconf.PreconfReceipt) event.Subscription {
    return b.preconfFeed.Subscribe(ch)
}
