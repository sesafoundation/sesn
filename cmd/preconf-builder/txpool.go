package main

import (
	"context"
	"math/big"
	"sort"
	"time"

	"github.com/sesafoundation/sesn/common"
	//"github.com/sesafoundation/sesn/core/types"
	//gethrpc "github.com/sesafoundation/sesn/rpc"
	"github.com/sesafoundation/sesn/log"
)

// minimal interfaces to call geth over IPC
type txpoolContentResponse struct {
	Pending map[string]map[string]*rpcTx `json:"pending"`
	Queued  map[string]map[string]*rpcTx `json:"queued"`
}
type rpcTx struct {
	Hash     common.Hash `json:"hash"`
	Nonce    uint64      `json:"nonce"`
	To       *common.Address `json:"to"`
	Value    *big.Int    `json:"value"`
	Gas      uint64      `json:"gas"`
	GasPrice *big.Int    `json:"gasPrice"`
	MaxFeePerGas *big.Int `json:"maxFeePerGas"`
	MaxPriorityFeePerGas *big.Int `json:"maxPriorityFeePerGas"`
	Input    []byte      `json:"input"`
	From     common.Address `json:"from"`
}

type RPCTransaction struct {
	Hash common.Hash
	Gas  uint64
}


// rpcTx mirrors fields returned by txpool_content RPC.


// get current head
//func (b *Builder) currentHead(ctx context.Context) common.Hash {
//	var head *types.Header
//	err := b.rpc.CallContext(ctx, &head, "eth_getBlockByNumber", "latest", false)
//	if err != nil || head == nil {
//		return common.Hash{}
//	}
//	return head.Hash()
//}
func (b *Builder) currentHead(ctx context.Context) common.Hash {
	var block struct {
		Hash common.Hash `json:"hash"`
	}
	if err := b.rpc.CallContext(ctx, &block, "eth_getBlockByNumber", "latest", false); err != nil {
		log.Warn("eth_getBlockByNumber RPC failed", "err", err)
		return common.Hash{}
	}
	return block.Hash
}

type txMeta struct {
	Hash        common.Hash
	FeeTip      *big.Int
	ArrivalRank uint64 // proxy for arrival order
	Gas         uint64
	From        common.Address
	Nonce       uint64
}

func (b *Builder) pickPendingTXs(ctx context.Context, maxCount int, gasBudget uint64) []common.Hash {
	// --- Query txpool_content via IPC ---
	var content txpoolContentResponse
	if err := b.rpc.CallContext(ctx, &content, "txpool_content"); err != nil {
		log.Warn("Failed to query txpool_content", "err", err)
		return nil
	}

	// --- Flatten pending transactions ---
	list := make([]txMeta, 0, 4096)
	var arrival uint64

	for from, nonces := range content.Pending {
		_ = from // key ignored, already present in txMeta
		for _, t := range nonces {
			arrival++

			// Determine tip: prefer MaxPriorityFeePerGas > GasPrice
			tip := new(big.Int)
			switch {
			case t.MaxPriorityFeePerGas != nil && t.MaxPriorityFeePerGas.Sign() > 0:
				tip.Set(t.MaxPriorityFeePerGas)
			case t.GasPrice != nil:
				tip.Set(t.GasPrice)
			default:
				tip.SetUint64(0)
			}

			list = append(list, txMeta{
				Hash:        t.Hash,
				FeeTip:      tip,
				ArrivalRank: arrival,
				Gas:         t.Gas,
				From:        t.From,
				Nonce:       t.Nonce,
			})
		}
	}

	if len(list) == 0 {
		return nil
	}

	// --- Sort: earliest arrival first, then highest fee tip, then hash lexicographically ---
	sort.Slice(list, func(i, j int) bool {
		if list[i].ArrivalRank != list[j].ArrivalRank {
			return list[i].ArrivalRank < list[j].ArrivalRank
		}
		if cmp := list[j].FeeTip.Cmp(list[i].FeeTip); cmp != 0 {
			return cmp < 0 // descending fee tip
		}
		return list[i].Hash.Hex() < list[j].Hash.Hex()
	})

	// --- Apply gas + count limits ---
	out := make([]common.Hash, 0, maxCount)
	var usedGas uint64

	for _, m := range list {
		if len(out) >= maxCount {
			break
		}
		if m.Gas == 0 || m.Gas > gasBudget {
			continue
		}
		if usedGas+m.Gas > gasBudget {
			break
		}
		out = append(out, m.Hash)
		usedGas += m.Gas
	}

	log.Trace("Selected txs for mini-block", "count", len(out), "gas", usedGas)
	return out
}

// small helper for timeouts
func ctxTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}
