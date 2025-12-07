// eth/quantblocks.go
package eth

import (
	"sync"
	"time"

	"github.com/sesafoundation/sesn/common"
	"github.com/sesafoundation/sesn/core/types"
)

// QuantConfirmInterval defines the target latency for QuantBlocks soft-confirmation.
const QuantConfirmInterval = 100 * time.Millisecond

// QuantTx stores basic metadata for a Quant-Confirmed transaction.
type QuantTx struct {
	Tx   *types.Transaction
	Time time.Time // time when it was first Quant-confirmed
}

// quantTxPool is a global in-memory map for Quant-Confirmed txs.
// This is *not* consensus state; it's a UX/soft-confirmation layer.
var quantTxPool = struct {
	sync.RWMutex
	confirmed map[common.Hash]*QuantTx
}{
	confirmed: make(map[common.Hash]*QuantTx),
}

// startQuantBlocks kicks off the 100ms QuantBlocks loop.
// It lives on the Ethereum backend so it has access to txpool.
func (eth *Ethereum) startQuantBlocks() {
	go func() {
		ticker := time.NewTicker(QuantConfirmInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				eth.confirmQuantTxs()
			case <-eth.shutdownChan:
				return
			}
		}
	}()
}

// confirmQuantTxs snapshots the txpool and marks all pending txs as Quant-Confirmed.
// Latency is roughly QuantConfirmInterval (~100 ms) from pool admission.
func (eth *Ethereum) confirmQuantTxs() {
	if eth.txPool == nil {
		return
	}

	// Geth-style: Pending() returns map[common.Address]types.Transactions
	pending, _ := eth.txPool.Pending()
	if len(pending) == 0 {
		return
	}

	now := time.Now()

	quantTxPool.Lock()
	defer quantTxPool.Unlock()

	for _, txs := range pending {
		for _, tx := range txs {
			h := tx.Hash()
			if _, exists := quantTxPool.confirmed[h]; !exists {
				quantTxPool.confirmed[h] = &QuantTx{
					Tx:   tx,
					Time: now,
				}
			}
		}
	}
}

// quantStatus computes the status for a given tx hash:
//   - "executed"         → found in canonical chain
//   - "quant-confirmed"  → in quantTxPool map
//   - "pending"          → in txpool but not quant-confirmed yet (rare race)
//   - "not-found"        → nowhere
func (eth *Ethereum) quantStatus(hash common.Hash) (string, error) {
	// 1) Check if transaction is already in the canonical chain
	if eth.blockchain != nil {
		if tx, _, _, _ := eth.blockchain.GetTransaction(hash); tx != nil {
			return "executed", nil
		}
	}

	// 2) Check QuantBlocks pool
	quantTxPool.RLock()
	_, ok := quantTxPool.confirmed[hash]
	quantTxPool.RUnlock()
	if ok {
		return "quant-confirmed", nil
	}

	// 3) Check txpool directly → "pending"
	if eth.txPool != nil {
		if tx := eth.txPool.Get(hash); tx != nil {
			return "pending", nil
		}
	}

	// 4) Nowhere
	return "not-found", nil
}
