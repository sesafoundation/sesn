// engine_equivocation.go
package hotstuff

import (
	"context"
	"math/big"
	"time"

	"github.com/sesafoundation/sesn/common"
)

func (e *Engine) reportEquivocation(height, view uint64, voter common.Address, blockIdA, blockIdB common.Hash, proofA, proofB []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sl, err := NewSlash(SlashContractAddr, e.rpc) // or backend.TxSender()
	if err != nil {
		e.log.Warn("slash binding", "err", err)
		return
	}
	opts, err := e.txOpts(ctx) // fill nonce/gas/chainID/signer
	if err != nil {
		e.log.Warn("tx opts", "err", err)
		return
	}
	_, err = sl.ReportEquivocation(
		opts,
		height, view,
		blockIdA, blockIdB, voter,
		proofA, proofB,
	)
	if err != nil {
		e.log.Warn("reportEquivocation failed", "err", err, "height", height, "view", view, "voter", voter)
	} else {
		e.log.Info("Equivocation reported", "height", height, "view", view, "voter", voter)
	}
}
