package sonium

import (
	"context"
	"fmt"

	"github.com/sesafoundation/sesn/common"
	"github.com/sesafoundation/sesn/consensus"
	//"github.com/sesafoundation/sesn/consensus/sonium/finality/hotstuff"
	"github.com/sesafoundation/sesn/consensus/sonium/finality/hotstuff"

	//fin "github.com/sesafoundation/sesn/internal/finality"
	fin "github.com/sesafoundation/sesn/internal/finality"

	"github.com/sesafoundation/sesn/core/types"


)

type withFinality struct {
	base   consensus.Engine
	gadget *hotstuff.Engine
}

func WithFinality(base consensus.Engine, gadget *hotstuff.Engine) consensus.Engine {
	return &withFinality{base: base, gadget: gadget}
}

func (w *withFinality) Name() string { return w.base.Name() + "+finality" }
func (w *withFinality) Author(h *types.Header) (common.Address, error) { return w.base.Author(h) }
func (w *withFinality) Prepare(c consensus.ChainHeaderReader, h *types.Header) error { return w.base.Prepare(c, h) }
func (w *withFinality) Finalize(c consensus.ChainHeaderReader, h *types.Header, s consensus.StateDB, txs []*types.Transaction, uncles []*types.Header, r []*types.Receipt) (*types.Block, error) {
	return w.base.Finalize(c, h, s, txs, uncles, r)
}

func (w *withFinality) VerifyHeader(chain consensus.ChainHeaderReader, h *types.Header, seal bool) error {
	if err := w.base.VerifyHeader(chain, h, seal); err != nil { return err }
	if payload, ok := fin.Extract(h.Extra); ok {
		if ok := w.gadget.VerifyCommit(h.Hash().Bytes(), payload.Round, payload.AggSig, payload.Bitmap); !ok {
			return fmt.Errorf("finality: invalid commit cert")
		}
	}
	return nil
}

func (w *withFinality) VerifyHeaders(c consensus.ChainHeaderReader, hs []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	return w.base.VerifyHeaders(c, hs, seals)
}

func (w *withFinality) Seal(c consensus.ChainHeaderReader, b *types.Block, stop <-chan struct{}) (*types.Block, error) {
	h := types.CopyHeader(b.Header())
	round := uint64(h.Number.Uint64())
	if cc, err := w.gadget.Propose(context.Background(), h, round); err == nil && cc != nil {
		h.Extra = fin.Append(h.Extra, fin.Payload{Round: cc.Round, AggSig: cc.AggSig, Bitmap: cc.Bitmap})
	}
	return w.base.Seal(c, types.NewBlockWithHeader(h), stop)
}

func (w *withFinality) SealHash(h *types.Header) (hash common.Hash) { return w.base.SealHash(h) }
func (w *withFinality) APIs(c consensus.ChainHeaderReader) []consensus.API { return w.base.APIs(c) }
func (w *withFinality) Close() error { return w.base.Close() }
