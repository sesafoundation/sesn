package hotstuff

import (
	"context"
	"math"
	"time"

	"github.com/sesafoundation/sesn/common"
	"github.com/sesafoundation/sesn/core/types"
)

type Config struct {
	BaseTimeout  time.Duration // floor (e.g., 80–120ms)
	UseCommittee bool
	CommitteeSz  int
}

type ValidatorSet interface {
	Active() ([]common.Address, [][]byte)
	IndexOf(common.Address) (int, bool)
	SelfCoinbase() common.Address
}

type Transport interface {
	BroadcastPropose(*ProposeMsg) error
	BroadcastVote(*VoteMsg) error
	BroadcastCommit(*CommitMsg) error
	RegisterHandler(Handler)
}

type Handler interface {
	OnPropose(*ProposeMsg)
	OnVote(*VoteMsg)
	OnCommit(*CommitMsg)
}

type ProposeMsg struct {
	BlockHash common.Hash
	Round     uint64
	Proposer  common.Address
}

type CommitMsg struct {
	BlockHash common.Hash
	Round     uint64
	AggSig    []byte
	Bitmap    []byte
}

type Engine struct {
	cfg  Config
	vs   ValidatorSet
	net  Transport
	bls  *BLS
	qc   *QCPool
	met  *Metrics
}

func New(cfg Config, vs ValidatorSet, net Transport, bls *BLS) *Engine {
	e := &Engine{cfg: cfg, vs: vs, net: net, bls: bls, qc: NewQCPool(), met: NewMetrics()}
	net.RegisterHandler(e)
	return e
}

func (e *Engine) adaptiveQuorum(n int) int {
	switch {
	case n <= 3:
		return n // all
	case n <= 10:
		return int(math.Ceil(0.75 * float64(n)))
	default:
		return int(math.Ceil(0.67 * float64(n)))
	}
}

func (e *Engine) timeout() time.Duration {
	//  ~4*RTT, bounded below by BaseTimeout
	rt := e.met.P95RTT()
	t := 4*rt
	if t < e.cfg.BaseTimeout { t = e.cfg.BaseTimeout }
	return t
}

// Called by proposer during sealing: broadcast proposal, self-vote, wait for CC.
func (e *Engine) Propose(ctx context.Context, hdr *types.Header, round uint64) (*CommitCert, error) {
	pm := &ProposeMsg{BlockHash: hdr.Hash(), Round: round, Proposer: hdr.Coinbase}
	_ = e.net.BroadcastPropose(pm)

	// self vote
	self, _ := e.vs.IndexOf(e.vs.SelfCoinbase())
	sig := e.bls.Sign(pm.BlockHash[:])
	_ = e.net.BroadcastVote(&VoteMsg{BlockHash: pm.BlockHash, Round: round, VoterIndex: uint16(self), Sig: sig})
	e.qc.AddVote(&VoteMsg{BlockHash: pm.BlockHash, Round: round, VoterIndex: uint16(self), Sig: sig})

	addrs, _ := e.vs.Active()
	quorum := e.adaptiveQuorum(len(addrs))
	cc, ok := e.qc.WaitForQuorum(pm.BlockHash, round, quorum, e.timeout(), e.bls)
	if ok {
		_ = e.net.BroadcastCommit(&CommitMsg{BlockHash: pm.BlockHash, Round: round, AggSig: cc.AggSig, Bitmap: cc.Bitmap})
		return cc, nil
	}
	return nil, context.DeadlineExceeded
}

// Net callbacks
func (e *Engine) OnPropose(p *ProposeMsg) {
	idx, _ := e.vs.IndexOf(e.vs.SelfCoinbase())
	sig := e.bls.Sign(p.BlockHash[:])
	vm := &VoteMsg{BlockHash: p.BlockHash, Round: p.Round, VoterIndex: uint16(idx), Sig: sig}
	_ = e.net.BroadcastVote(vm)
	e.qc.AddVote(vm)
}
func (e *Engine) OnVote(v *VoteMsg)   { e.qc.AddVote(v) }
func (e *Engine) OnCommit(c *CommitMsg) { /* optional fast-forward */ }

// Verify commit (used on import)
func (e *Engine) VerifyCommit(msg []byte, round uint64, aggSig, bitmap []byte) bool {
	_, pubs := e.vs.Active()
	return e.bls.VerifyAggregate(msg, aggSig, pubs, bitmap)
}
