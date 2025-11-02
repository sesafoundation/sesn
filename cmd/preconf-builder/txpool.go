package main

import (
	"context"
	"math/big"
	"sort"
	"time"

	"github.com/sesafoundation/sesn/common"
	"github.com/sesafoundation/sesn/core/types"
	gethrpc "github.com/sesafoundation/sesn/rpc"
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

// get current head
func (b *Builder) currentHead(ctx context.Context) common.Hash {
	var head *types.Header
	err := b.rpc.CallContext(ctx, &head, "eth_getBlockByNumber", "latest", false)
	if err != nil || head == nil {
		return common.Hash{}
	}
	return head.Hash()
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
	// Pull txpool content
	var content txpoolContentResponse
	_ = b.rpc.CallContext(ctx, &content, "txpool_content")

	// Flatten pending (ignore queued for preconf)
	list := make([]txMeta, 0, 4096)
	var arrival uint64
	for from, nonces := range content.Pending {
		_ = from
		for _, t := range nonces {
			arrival++
			// tip = maxPriorityFeePerGas if present else gasPrice
			tip := new(big.Int)
			if t.MaxPriorityFeePerGas != nil && t.MaxPriorityFeePerGas.Sign() > 0 {
				tip.Set(t.MaxPriorityFeePerGas)
			} else if t.GasPrice != nil {
				tip.Set(t.GasPrice)
			} else {
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

	// Order: arrival-time first; tie-break higher tip, then hash
	sort.Slice(list, func(i, j int) bool {
		if list[i].ArrivalRank != list[j].ArrivalRank {
			return list[i].ArrivalRank < list[j].ArrivalRank
		}
		c := list[j].FeeTip.Cmp(list[i].FeeTip) // descending tip
		if c != 0 { return c < 0 }
		return list[i].Hash.Hex() < list[j].Hash.Hex()
	})

	// Stateless eligibility checks (fast):
	// - (Optional) you can call eth_getTransactionCount(from, "pending") to avoid nonce gaps.
	// - Gas budget: stop when exceeding slice budget.

	out := make([]common.Hash, 0, maxCount)
	var usedGas uint64
	for _, m := range list {
		if len(out) >= maxCount { break }
		if m.Gas == 0 || m.Gas > gasBudget { continue }
		if usedGas + m.Gas > gasBudget { break }
		out = append(out, m.Hash)
		usedGas += m.Gas
	}
	return out
}

// small helper for timeouts
func ctxTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}
