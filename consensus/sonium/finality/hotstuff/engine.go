package hotstuff

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sesafoundation/sesn/common"
	"github.com/sesafoundation/sesn/core/types"
)

// ──────────────────────────────────────────────────────────────
// Engine configuration
// ──────────────────────────────────────────────────────────────
//type Config struct {
//	BaseTimeout time.Duration
//}

// ──────────────────────────────────────────────────────────────
// Engine represents a validator's local HotStuff logic
// ──────────────────────────────────────────────────────────────
//type Engine struct {
//	cfg       Config
//	vset      ValidatorSet
//	transport Transport
//	bls       BLS // ✅ interface, not *BLS
//	mu        sync.Mutex
//	round     uint64
//}

// engine.go
type Engine struct {
    cfg        Config
    vset       ValidatorSet   // contract-backed
    tr         Transport      // devp2p/QUIC
    bls        BLS            // real BLS
    store      Store          // on-disk QC/vote storage
    leaderSel  LeaderSelector // proposer for (height, view)
    clock      Clock          // mockable time
    log        Logger
    metrics    *Metrics
    mu         sync.Mutex

    height     uint64
    view       uint64
    lastQC     *QuorumCert
    finalized  uint64 // last finalized height
}

type Config struct {
    BaseTimeout     time.Duration // e.g. 100ms
    MaxTimeout      time.Duration // safety upper bound
    ActivateAt      uint64        // from genesis FinalityConfig
    CommitteeSize   int           // 0 = all
    GossipTopic     string
    Domain          []byte        // BLS domain separation
    StoragePath     string
}


// ──────────────────────────────────────────────────────────────
// Constructor
// ──────────────────────────────────────────────────────────────
//func New(cfg Config, vset ValidatorSet, transport Transport, blsImpl BLS) *Engine {
//	return &Engine{
//		cfg:       cfg,
//		vset:      vset,
//		transport: transport,
//		bls:       blsImpl, // ✅ store interface directly
//	}
//}

// engine.go (constructor)
func New(cfg Config, vset ValidatorSet, tr Transport, bls BLS, st Store) *Engine {
    e := &Engine{
        cfg: cfg, vset: vset, tr: tr, bls: bls, store: st,
        leaderSel: DefaultLeaderSelector{},
        clock:     RealClock{},
        metrics:   NewMetrics(),
    }
    tr.RegisterHandler(e)
    // Restore state
    if fh, err := st.GetFinalizedHeight(); err == nil { e.finalized = fh }
    if qc, err := st.GetQC(e.finalized); err == nil { e.lastQC = qc }
    return e
}

func (e *Engine) ShouldActivateAt(height uint64) bool {
    return height >= e.cfg.ActivateAt
}


// ──────────────────────────────────────────────────────────────
// Message types (simplified for demo)
// ──────────────────────────────────────────────────────────────
type ProposeMsg struct {
	Round   uint64
	Proposer common.Address
	Header  *types.Header
}

type VoteMsg struct {
	Round       uint64
	Voter       common.Address
	VoterIndex  uint32
	Signature   []byte
}

type CommitMsg struct {
	Round     uint64
	Aggregate []byte
}

// ──────────────────────────────────────────────────────────────
// Core logic
// ──────────────────────────────────────────────────────────────

// Propose is called by the proposer to start a new round.
func (e *Engine) Propose(ctx context.Context, header *types.Header, round uint64) (*CommitMsg, error) {
	e.mu.Lock()
	e.round = round
	e.mu.Unlock()

	msg := []byte(fmt.Sprintf("round-%d-%x", round, header.Coinbase))
	sig := e.bls.Sign(msg) // ✅ works correctly now

	vote := &VoteMsg{
		Round:      round,
		Voter:      header.Coinbase,
		VoterIndex: 0,
		Signature:  sig,
	}
	// Broadcast to peers
	if err := e.transport.BroadcastVote(vote); err != nil {
		return nil, err
	}

	// Wait briefly for responses (simulation)
	time.Sleep(e.cfg.BaseTimeout / 2)

	// Collect votes (in a real system, gather from peers)
	sigs := [][]byte{sig}
	agg := e.bls.Aggregate(sigs)

	return &CommitMsg{Round: round, Aggregate: agg}, nil
}

// OnPropose handles a received proposal.
func (e *Engine) OnPropose(m *ProposeMsg) {
	msg := []byte(fmt.Sprintf("round-%d-%x", m.Round, m.Header.Coinbase))
	sig := e.bls.Sign(msg)
	vote := &VoteMsg{
		Round:      m.Round,
		Voter:      e.vset.SelfCoinbase(),
		VoterIndex: 0,
		Signature:  sig,
	}
	_ = e.transport.BroadcastVote(vote)
}

// OnVote handles a received vote.
//func (e *Engine) OnVote(m *VoteMsg) {
//	// In real HotStuff, collect and aggregate here
//	_ = m
//}

func (e *Engine) OnVote(m *VoteMsg) {
    if !e.ShouldActivateAt(m.Height) { return }
    if e.store.HasVote(m.Height, m.View, m.Voter) { return } // dedupe
    if !e.verifyVote(m) { return }
    _ = e.store.PutVote(m.Height, m.View, m.Voter, m.Signature)

    if e.hasQuorum(m.Height, m.View) {
        qc := e.buildQC(m.Height, m.View)
        if qc != nil && e.bls.VerifyAggregate(qc.PubKeys, e.msgBytes(qc), qc.AggSig) {
            _ = e.store.PutQC(qc)
            e.finalize(qc.Height)
            e.tr.BroadcastCommit(&CommitMsg{Round: qc.Height, Aggregate: qc.AggSig})
        }
    }
}


// OnCommit handles a final commit message.
func (e *Engine) OnCommit(m *CommitMsg) {
	// In real HotStuff, verify aggregate signature
	fmt.Printf("✅ Round %d finalized with aggregate signature %x\n", m.Round, m.Aggregate[:8])
}

// engine.go (core loop sketch)
func (e *Engine) step(height, view uint64) {
    leader := e.leaderSel.Proposer(height, view, e.vset)
    if leader == e.vset.SelfCoinbase() {
        e.propose(height, view)
    }
    e.armTimer(e.cfg.BaseTimeout) // adaptive backoff on timeout
}

func (e *Engine) onTimeout(height, view uint64) {
    // Start new view: view++
    e.view++
    e.metrics.ViewChange.Inc()
    e.step(e.height, e.view)
}

func (e *Engine) OnVote(m *VoteMsg) {
    if !e.ShouldActivateAt(m.Height) { return }
    if e.store.HasVote(m.Height, m.View, m.Voter) { return } // dedupe
    if !e.verifyVote(m) { return }
    _ = e.store.PutVote(m.Height, m.View, m.Voter, m.Signature)

    if e.hasQuorum(m.Height, m.View) {
        qc := e.buildQC(m.Height, m.View)
        if qc != nil && e.bls.VerifyAggregate(qc.PubKeys, e.msgBytes(qc), qc.AggSig) {
            _ = e.store.PutQC(qc)
            e.finalize(qc.Height)
            e.tr.BroadcastCommit(&CommitMsg{Round: qc.Height, Aggregate: qc.AggSig})
        }
    }
}




