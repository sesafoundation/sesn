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
	"github.com/sesafoundation/sesn/params"
	"github.com/sesafoundation/sesn/preconf"
	"github.com/sesafoundation/sesn/rpc"
)

// NOTE: Backend interface is defined in backend.go and is implemented by *eth.Ethereum.
// EthAPIBackend wraps that Backend to:
//   - act as the Backend passed into internal/ethapi APIs
//   - provide a gas-price oracle backend (gasprice.OracleBackend)
//   - add preconf (mini-block) receipt storage + subscriptions.

type EthAPIBackend struct {
	backend       Backend
	extRPCEnabled bool
	gpo           *gasprice.Oracle

	preconfMu       sync.RWMutex
	preconfReceipts map[common.Hash]*preconf.PreconfReceipt
	preconfFeed     event.Feed
}

// NewEthAPIBackend constructs the wrapper used by eth.Ethereum.
func NewEthAPIBackend(extRPC bool, backend Backend) *EthAPIBackend {
	return &EthAPIBackend{
		backend:         backend,
		extRPCEnabled:   extRPC,
		preconfReceipts: make(map[common.Hash]*preconf.PreconfReceipt),
	}
}

// SetOracle sets the gas price oracle instance used by SuggestPrice.
func (b *EthAPIBackend) SetOracle(o *gasprice.Oracle) {
	b.gpo = o
}

// -----------------------------------------------------------------------------
// General / sync / DB / accounts
// -----------------------------------------------------------------------------

func (b *EthAPIBackend) Downloader() *downloader.Downloader {
	return b.backend.Downloader()
}

func (b *EthAPIBackend) ProtocolVersion() int {
	return b.backend.ProtocolVersion()
}

// gasprice.OracleBackend + ethapi.Backend
func (b *EthAPIBackend) SuggestPrice(ctx context.Context) (*big.Int, error) {
	if b.gpo == nil {
		return nil, errors.New("gas price oracle not initialised")
	}
	return b.gpo.SuggestPrice(ctx)
}

func (b *EthAPIBackend) ChainDb() ethdb.Database {
	return b.backend.ChainDb()
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

func (b *EthAPIBackend) EventMux() *event.TypeMux {
	return b.backend.EventMux()
}

func (b *EthAPIBackend) ChainConfig() *params.ChainConfig {
	return b.backend.ChainConfig()
}

func (b *EthAPIBackend) Engine() consensus.Engine {
	return b.backend.Engine()
}

// -----------------------------------------------------------------------------
// Chain / headers / state
// -----------------------------------------------------------------------------

func (b *EthAPIBackend) SetHead(number uint64) {
	b.backend.SetHead(number)
}

func (b *EthAPIBackend) CurrentHeader() *types.Header {
	return b.backend.CurrentHeader()
}

func (b *EthAPIBackend) CurrentBlock() *types.Block {
	return b.backend.CurrentBlock()
}

func (b *EthAPIBackend) HeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Header, error) {
	return b.backend.HeaderByNumber(ctx, number)
}

func (b *EthAPIBackend) HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error) {
	return b.backend.HeaderByHash(ctx, hash)
}

func (b *EthAPIBackend) HeaderByNumberOrHash(ctx context.Context, bh rpc.BlockNumberOrHash) (*types.Header, error) {
	return b.backend.HeaderByNumberOrHash(ctx, bh)
}

func (b *EthAPIBackend) BlockByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Block, error) {
	return b.backend.BlockByNumber(ctx, number)
}

func (b *EthAPIBackend) BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error) {
	return b.backend.BlockByHash(ctx, hash)
}

func (b *EthAPIBackend) BlockByNumberOrHash(ctx context.Context, bh rpc.BlockNumberOrHash) (*types.Block, error) {
	return b.backend.BlockByNumberOrHash(ctx, bh)
}

func (b *EthAPIBackend) StateAndHeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*state.StateDB, *types.Header, error) {
	return b.backend.StateAndHeaderByNumber(ctx, number)
}

func (b *EthAPIBackend) StateAndHeaderByNumberOrHash(ctx context.Context, bh rpc.BlockNumberOrHash) (*state.StateDB, *types.Header, error) {
	return b.backend.StateAndHeaderByNumberOrHash(ctx, bh)
}

func (b *EthAPIBackend) GetReceipts(ctx context.Context, hash common.Hash) (types.Receipts, error) {
	return b.backend.GetReceipts(ctx, hash)
}

func (b *EthAPIBackend) GetLogs(ctx context.Context, hash common.Hash) ([][]*types.Log, error) {
	return b.backend.GetLogs(ctx, hash)
}

func (b *EthAPIBackend) GetTd(ctx context.Context, hash common.Hash) *big.Int {
	return b.backend.GetTd(ctx, hash)
}

func (b *EthAPIBackend) GetEVM(ctx context.Context, msg core.Message, st *state.StateDB, header *types.Header) (*vm.EVM, func() error, error) {
	return b.backend.GetEVM(ctx, msg, st, header)
}

// -----------------------------------------------------------------------------
// Chain subscriptions
// -----------------------------------------------------------------------------

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

// -----------------------------------------------------------------------------
// Tx pool
// -----------------------------------------------------------------------------

func (b *EthAPIBackend) SendTx(ctx context.Context, signedTx *types.Transaction) error {
	return b.backend.SendTx(ctx, signedTx)
}

func (b *EthAPIBackend) GetTransaction(ctx context.Context, txHash common.Hash) (*types.Transaction, common.Hash, uint64, uint64, error) {
	return b.backend.GetTransaction(ctx, txHash)
}

func (b *EthAPIBackend) GetPoolTransactions() (types.Transactions, error) {
	return b.backend.GetPoolTransactions()
}

func (b *EthAPIBackend) GetPoolTransaction(txHash common.Hash) *types.Transaction {
	return b.backend.GetPoolTransaction(txHash)
}

func (b *EthAPIBackend) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return b.backend.GetPoolNonce(ctx, addr)
}

func (b *EthAPIBackend) Stats() (pending int, queued int) {
	return b.backend.Stats()
}

func (b *EthAPIBackend) TxPoolContent() (map[common.Address]types.Transactions, map[common.Address]types.Transactions) {
	return b.backend.TxPoolContent()
}

func (b *EthAPIBackend) SubscribeNewTxsEvent(ch chan<- core.NewTxsEvent) event.Subscription {
	return b.backend.SubscribeNewTxsEvent(ch)
}

// -----------------------------------------------------------------------------
// Bloom / filter service
// -----------------------------------------------------------------------------

func (b *EthAPIBackend) BloomStatus() (uint64, uint64) {
	return b.backend.BloomStatus()
}

func (b *EthAPIBackend) ServiceFilter(ctx context.Context, session *bloombits.MatcherSession) {
	b.backend.ServiceFilter(ctx, session)
}

// -----------------------------------------------------------------------------
// Mining / engine
// -----------------------------------------------------------------------------

func (b *EthAPIBackend) Miner() *miner.Miner {
	return b.backend.Miner()
}

func (b *EthAPIBackend) StartMining(threads int) error {
	return b.backend.StartMining(threads)
}

// -----------------------------------------------------------------------------
// PRECONF (mini-block) helpers
// -----------------------------------------------------------------------------

// StorePreconfReceipt is called by the miner/worker when it attaches a
// preconfirmation to a transaction.
func (b *EthAPIBackend) StorePreconfReceipt(hash common.Hash, r *preconf.PreconfReceipt) {
	b.preconfMu.Lock()
	b.preconfReceipts[hash] = r
	b.preconfMu.Unlock()
	b.preconfFeed.Send(r)
}

// LoadPreconfReceipt is used by the preconf RPC to fetch the receipt.
func (b *EthAPIBackend) LoadPreconfReceipt(hash common.Hash) *preconf.PreconfReceipt {
	b.preconfMu.RLock()
	r := b.preconfReceipts[hash]
	b.preconfMu.RUnlock()
	return r
}

// PreconfSubscribe lets RPC layer subscribe to streaming preconf receipts.
func (b *EthAPIBackend) PreconfSubscribe(ch chan *preconf.PreconfReceipt) event.Subscription {
	return b.preconfFeed.Subscribe(ch)
}
