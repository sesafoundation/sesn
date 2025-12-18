// eth/quantblocks.go
package eth

import (
	"sync"
	"time"
    "fmt"

	"github.com/sesafoundation/sesn/common"
	"github.com/sesafoundation/sesn/core/types"
	"github.com/sesafoundation/sesn/core/rawdb"
	"github.com/sesafoundation/sesn/log"
)

// QuantConfirmInterval defines the target latency for QuantBlocks quant-confirmation.
const QuantConfirmInterval = 100 * time.Millisecond

// QuantTx stores basic metadata for a Quant-Confirmed transaction.
type QuantTx struct {
	Tx   *types.Transaction
	Time time.Time // time when it was first Quant-confirmed
}

// quantTxPool is a global in-memory map for Quant-Confirmed txs.
// This is *not* consensus state; it's a UX/quant-confirmation layer.
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
        log.Info("QuantBlocks: starting 100ms QuantBlocks-Confirmation Engine")

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
    newTxs := make([]*types.Transaction, 0, 1024)

    quantTxPool.Lock()
    for _, txs := range pending {
        for _, tx := range txs {
            h := tx.Hash()
            if _, exists := quantTxPool.confirmed[h]; !exists {
                quantTxPool.confirmed[h] = &QuantTx{
                    Tx:   tx,
                    Time: now,
                }
                newTxs = append(newTxs, tx)
                added++
            }
        }
    }
    quantTxPool.Unlock()

    if added > 0 {
        // 100ms interval → QuantTPS = tx_count * 10
        quantTPS := float64(added) * 10.0

        // Debug log for only newly confirmed transactions
        for _, tx := range newTxs {
            log.Debug("QuantBlocks: tx quant-confirmed ⚡",
                "hash", tx.Hash().Hex(),
                "time", now.UnixMilli())
        }

        // Main info log
        log.Info("QuantBlocks: QuantConfirmed Batch ⚡",
            "new_tx", added,
            "quant_tps", fmt.Sprintf("%.2f", quantTPS),
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

    // 2) Check QuantBlocks QuantBlocks-confirmation list
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




