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
type Config struct {
	BaseTimeout time.Duration
}

// ──────────────────────────────────────────────────────────────
// Engine represents a validator's local HotStuff logic
// ──────────────────────────────────────────────────────────────
type Engine struct {
	cfg       Config
	vset      ValidatorSet
	transport Transport
	bls       BLS // ✅ interface, not *BLS
	mu        sync.Mutex
	round     uint64
}

// ──────────────────────────────────────────────────────────────
// Constructor
// ──────────────────────────────────────────────────────────────
func New(cfg Config, vset ValidatorSet, transport Transport, blsImpl BLS) *Engine {
	return &Engine{
		cfg:       cfg,
		vset:      vset,
		transport: transport,
		bls:       blsImpl, // ✅ store interface directly
	}
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
func (e *Engine) OnVote(m *VoteMsg) {
	// In real HotStuff, collect and aggregate here
	_ = m
}

// OnCommit handles a final commit message.
func (e *Engine) OnCommit(m *CommitMsg) {
	// In real HotStuff, verify aggregate signature
	fmt.Printf("✅ Round %d finalized with aggregate signature %x\n", m.Round, m.Aggregate[:8])
}

