// eth/quantblocks.go
package eth

import (
	"sync"
	"time"

	"github.com/sesafoundation/sesn/common"
	"github.com/sesafoundation/sesn/core/types"
	"github.com/sesafoundation/sesn/core/rawdb"
	"github.com/sesafoundation/sesn/log"
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
        log.Info("QuantBlocks: starting 100ms soft-confirmation engine")

        ticker := time.NewTicker(QuantConfirmInterval)
        defer ticker.Stop()

        for {
            <-ticker.C
            eth.confirmQuantTxs()
        }
    }()
}


// confirmQuantTxs snapshots the txpool and marks all pending txs as Quant-Confirmed.
// Latency is roughly QuantConfirmInterval (~100 ms) from pool admission.
func (eth *Ethereum) confirmQuantTxs() {
    if eth.txPool == nil {
        return
    }

    pending, _ := eth.txPool.Pending()
    if len(pending) == 0 {
        return
    }

    now := time.Now()
    added := 0

    quantTxPool.Lock()
    for _, txs := range pending {
        for _, tx := range txs {
            h := tx.Hash()
            if _, exists := quantTxPool.confirmed[h]; !exists {
                quantTxPool.confirmed[h] = &QuantTx{
                    Tx:   tx,
                    Time: now,
                }
                added++
            }
        }
    }
    quantTxPool.Unlock()

    if added > 0 {
		for _, txs := range pending {
    		for _, tx := range txs {
        	log.Debug("QuantBlocks: tx quant-confirmed", "hash", tx.Hash().Hex())
    		}
		}
        log.Info("QuantBlocks: soft-confirmed transactions",
            "count", added,
            "timestamp", now.UnixMilli(),
        )
    }
}


// quantStatus computes the status for a given tx hash:
//   - "executed"         → found in canonical chain
//   - "quant-confirmed"  → in quantTxPool map
//   - "pending"          → in txpool but not quant-confirmed yet (rare race)
//   - "not-found"        → nowhere
func (eth *Ethereum) quantStatus(hash common.Hash) (string, error) {
	//log.Trace("QuantBlocks: tx status check", "hash", hash.Hex(), "status", status)
    log.Trace("QuantBlocks: checking tx status", "hash", hash.Hex())

    // 1) Check LevelDB → check if tx is included in any block
    if tx, blockHash, _, _ := rawdb.ReadTransaction(eth.chainDb, hash); tx != nil {
        // blockHash != zero means it is part of a canonical block
        if blockHash != (common.Hash{}) {
            return "executed", nil
        }
    }

    // 2) Check QuantBlocks soft-confirmation list
    quantTxPool.RLock()
    _, ok := quantTxPool.confirmed[hash]
    quantTxPool.RUnlock()
    if ok {
        return "quant-confirmed", nil
    }

    // 3) Check txpool -> pending
    if eth.txPool != nil {
        if tx := eth.txPool.Get(hash); tx != nil {
            return "pending", nil
        }
    }

    // 4) Not found anywhere
    return "not-found", nil
}




