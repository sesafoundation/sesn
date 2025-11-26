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

// Package ethapi implements the general Ethereum API functions.
package ethapi

import (
	"context"
	"math/big"

	"github.com/sesafoundation/sesn/accounts"
	"github.com/sesafoundation/sesn/common"
	"github.com/sesafoundation/sesn/consensus"
	"github.com/sesafoundation/sesn/core"
	"github.com/sesafoundation/sesn/core/bloombits"
	"github.com/sesafoundation/sesn/core/state"
	"github.com/sesafoundation/sesn/core/types"
	"github.com/sesafoundation/sesn/core/vm"
	"github.com/sesafoundation/sesn/eth/downloader"
	"github.com/sesafoundation/sesn/ethdb"
	"github.com/sesafoundation/sesn/event"
	"github.com/sesafoundation/sesn/params"
	"github.com/sesafoundation/sesn/rpc"
	// preconfapi "github.com/sesafoundation/sesn/internal/preconfapi"
	 "github.com/sesafoundation/sesn/preconf"
	// "github.com/sesafoundation/sesn/internal/preconfapi"
	 preconfapi "github.com/sesafoundation/sesn/internal/preconfapi"
	 "github.com/sesafoundation/sesn/miner"
	  //ethtypes "github.com/sesafoundation/sesn/eth/protocols/eth"
)

// Backend interface provides the common API services (that are provided by
// both full and light clients) with access to necessary functions.
type Backend interface {
    // Downloader / Chain Management
   // Downloader() *downloader.Downloader
   // ProtocolVersion() int
    //SuggestPrice(ctx context.Context) (*big.Int, error)
    //ChainDb() ethdb.Database
   // AccountManager() *accounts.Manager
    ExtRPCEnabled() bool
    //RPCGasCap() uint64
    //RPCTxFeeCap() float64

    // Blockchain Queries
   // SetHead(number uint64)
    HeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Header, error)
    HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error)
    HeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Header, error)
    CurrentHeader() *types.Header
   // CurrentBlock() *types.Block
    BlockByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Block, error)
    BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error)
    BlockByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Block, error)
    StateAndHeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*state.StateDB, *types.Header, error)
    StateAndHeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*state.StateDB, *types.Header, error)
    GetReceipts(ctx context.Context, hash common.Hash) (types.Receipts, error)
    GetTd(ctx context.Context, hash common.Hash) *big.Int
    GetEVM(ctx context.Context, msg core.Message, state *state.StateDB, header *types.Header) (*vm.EVM, func() error, error)
    SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription
    SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription
    SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription

    // TxPool
    //SendTx(ctx context.Context, signedTx *types.Transaction) error
    GetTransaction(ctx context.Context, txHash common.Hash) (*types.Transaction, common.Hash, uint64, uint64, error)
    GetPoolTransactions() (types.Transactions, error)
    GetPoolTransaction(txHash common.Hash) *types.Transaction
    GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error)
    Stats() (pending int, queued int)
    TxPoolContent() (map[common.Address]types.Transactions, map[common.Address]types.Transactions)
    SubscribeNewTxsEvent(chan<- core.NewTxsEvent) event.Subscription

    // Logs / Filters
    BloomStatus() (uint64, uint64)
    GetLogs(ctx context.Context, hash common.Hash) ([][]*types.Log, error)
    ServiceFilter(ctx context.Context, session *bloombits.MatcherSession)
    SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription
    SubscribePendingLogsEvent(ch chan<- []*types.Log) event.Subscription
    SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription

   // ChainConfig() *params.ChainConfig
   // Engine() consensus.Engine

    // ⭐ PRECONF EXTENSION (required)
    StorePreconfReceipt(common.Hash, *preconf.PreconfReceipt)
    LoadPreconfReceipt(common.Hash) *preconf.PreconfReceipt
    PreconfSubscribe(chan *preconf.PreconfReceipt) event.Subscription

	//BlockChain() *core.BlockChain
	//Miner() *miner.Miner
	//ProtocolManager() *ProtocolManager
	//ProtocolManager() *ethtypes.ProtocolManager
	//NetVersion() int
	//NodeInfo() interface{}

	    // blockchain
   // BlockChain() *core.BlockChain
    CurrentBlock() *types.Block
   // CurrentHeader() *types.Header
    SetHead(uint64)

    // mining
   // Miner() *miner.Miner
    //StartMining(int) error

    // txpool
    //TxPool() *core.TxPool
    SendTx(context.Context, *types.Transaction) error

    // sync / networking
   // Downloader() *downloader.Downloader
 //   ProtocolVersion() int
   // NetVersion() uint64
   // NodeInfo() interface{}

    // config + database
   // ChainDb() ethdb.Database
   // ChainConfig() *params.ChainConfig
    //AccountManager() *accounts.Manager
   // Engine() consensus.Engine

	//Miner() *miner.Miner
	//StartMining(int) error

	EthVersion() int 

    // RPC safety
    //RPCGasCap() uint64
    //RPCTxFeeCap() float64
/////////////new
	BlockChain() *core.BlockChain
    ChainConfig() *params.ChainConfig
    Engine() consensus.Engine

    // mining
    Miner() *miner.Miner
    StartMining(int) error

    // tx pool
    TxPool() *core.TxPool

    // sync / networking
    Downloader() *downloader.Downloader
    ProtocolVersion() int
    NetVersion() uint64

    // database + accounts
    ChainDb() ethdb.Database
    AccountManager() *accounts.Manager
    EventMux() *event.TypeMux      // 🔥 required by api_backend.go

    // RPC safety
    RPCGasCap() uint64
    RPCTxFeeCap() float64

    // gas oracle
    SuggestPrice(context.Context) (*big.Int, error)

    // misc
    NodeInfo() interface{}
}


func (b *EthAPIBackend) BlockChain() *core.BlockChain {
    return b.backend.BlockChain()
}



func GetAPIs(apiBackend Backend) []rpc.API {
	nonceLock := new(AddrLocker)

	apis := []rpc.API{
		{
			Namespace: "eth",
			Version:   "1.0",
			Service:   NewPublicEthereumAPI(apiBackend),
			Public:    true,
		},
		{
			Namespace: "eth",
			Version:   "1.0",
			Service:   NewPublicBlockChainAPI(apiBackend),
			Public:    true,
		},
		{
			Namespace: "eth",
			Version:   "1.0",
			Service:   NewPublicTransactionPoolAPI(apiBackend, nonceLock),
			Public:    true,
		},
		{
			Namespace: "txpool",
			Version:   "1.0",
			Service:   NewPublicTxPoolAPI(apiBackend),
			Public:    true,
		},
		{
			Namespace: "eth",
			Version:   "1.0",
			Service:   NewPublicAccountAPI(apiBackend.AccountManager()),
			Public:    true,
		},
		{
			Namespace: "personal",
			Version:   "1.0",
			Service:   NewPrivateAccountAPI(apiBackend, nonceLock),
			Public:    false,
		},
	}

	// 🔥 PRECONF EXTENSIONS
	if pb, ok := apiBackend.(preconfapi.PreconfBackend); ok {
		apis = append(
			apis,
			rpc.API{
				Namespace: "preconf",
				Version:   "1.0",
				Service:   preconfapi.NewPublicPreconfAPI(pb),
				Public:    true,
			},
			rpc.API{
				Namespace: "preconf",
				Version:   "1.0",
				Service:   preconfapi.NewPublicPreconfSubscriptionAPI(pb),
				Public:    true,
			},
		)
	}

	return apis
}



