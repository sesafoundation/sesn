package main

import (
	"context"
	"fmt"
	"math/big"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/sesafoundation/sesn/common"
	"github.com/sesafoundation/sesn/core/types"

	// Your finality gadget imports
	"github.com/sesafoundation/sesn/consensus/sonium/finality/hotstuff"
	"github.com/sesafoundation/sesn/p2p/mfproto"

	//"github.com/sesafoundation/sesn/consensus/sonium/finality/hotstuff"
	//"github.com/sesafoundation/sesn/p2p/mfproto"

	bls "github.com/kilic/bls12-381"
)

// ─────────────────────────────────────────────────────────────────────────────
// Simple ANSI helpers
// ─────────────────────────────────────────────────────────────────────────────
const (
	clr    = "\033[0m"
	bold   = "\033[1m"
	grey   = "\033[38;5;240m"
	green  = "\033[38;5;46m"
	yellow = "\033[38;5;220m"
	cyan   = "\033[38;5;45m"
	red    = "\033[38;5;203m"
)
func clearScreen()         { fmt.Print("\033[2J") }
func moveHome()            { fmt.Print("\033[H") }
func hideCursor()          { fmt.Print("\033[?25l") }
func showCursor()          { fmt.Print("\033[?25h") }
func padRight(s string, w int) string {
	if len(s) >= w { return s[:w] }
	return s + strings.Repeat(" ", w-len(s))
}

// ─────────────────────────────────────────────────────────────────────────────
// Demo types
// ─────────────────────────────────────────────────────────────────────────────
type DemoValidator struct {
	ID      int
	Addr    common.Address
	Engine  *hotstuff.Engine
	PrivKey *bls.Fr
	PubKey  *bls.G1
	RecvCh  chan interface{}
	Peers   []*DemoValidator
}

type GossipTransport struct {
	v *DemoValidator
}
func (t *GossipTransport) BroadcastPropose(m *hotstuff.ProposeMsg) error {
	for _, p := range t.v.Peers {
		go func(peer *DemoValidator) {
			delay := time.Duration(rand.Intn(10)+5) * time.Millisecond
			time.Sleep(delay)
			peer.RecvCh <- *m
		}(p)
	}
	return nil
}
func (t *GossipTransport) BroadcastVote(m *hotstuff.VoteMsg) error {
	for _, p := range t.v.Peers {
		go func(peer *DemoValidator) {
			delay := time.Duration(rand.Intn(10)+5) * time.Millisecond
			time.Sleep(delay)
			peer.RecvCh <- *m
		}(p)
	}
	return nil
}
func (t *GossipTransport) BroadcastCommit(m *hotstuff.CommitMsg) error {
	for _, p := range t.v.Peers {
		go func(peer *DemoValidator) {
			delay := time.Duration(rand.Intn(10)+5) * time.Millisecond
			time.Sleep(delay)
			peer.RecvCh <- *m
		}(p)
	}
	return nil
}
func (t *GossipTransport) RegisterHandler(h hotstuff.Handler) {}

// Validator set (static for demo)
type LocalValidatorSet struct{ Vals []*DemoValidator }
func (vs *LocalValidatorSet) Active() ([]common.Address, [][]byte) {
	addrs := make([]common.Address, len(vs.Vals))
	pubs := make([][]byte, len(vs.Vals))
	for i, v := range vs.Vals { addrs[i]=v.Addr; pubs[i]=v.PubKey.ToCompressed() }
	return addrs, pubs
}
func (vs *LocalValidatorSet) IndexOf(a common.Address) (int, bool) {
	for i, v := range vs.Vals { if v.Addr==a { return i, true } }
	return -1, false
}
func (vs *LocalValidatorSet) SelfCoinbase() common.Address { return vs.Vals[0].Addr }

// ─────────────────────────────────────────────────────────────────────────────
// Dashboard model
// ─────────────────────────────────────────────────────────────────────────────
type RoundStatus struct {
	Round        uint64
	ProposerID   int
	StartedAt    time.Time
	FinalizedAt  time.Time
	Final        bool
	Quorum       int
	Total        int
	Voted        map[int]time.Duration // validatorID -> vote latency
	mu           sync.RWMutex
}

type Dashboard struct {
	Title       string
	Rounds      []*RoundStatus
	RoundsMu    sync.RWMutex
	RenderTick  *time.Ticker
	Quit        chan struct{}
}

func NewDashboard(title string) *Dashboard {
	return &Dashboard{
		Title:      title,
		RenderTick: time.NewTicker(50 * time.Millisecond),
		Quit:       make(chan struct{}),
	}
}

func (d *Dashboard) AddRound(rs *RoundStatus) {
	d.RoundsMu.Lock(); defer d.RoundsMu.Unlock()
	d.Rounds = append(d.Rounds, rs)
}

func (d *Dashboard) UpdateVote(round uint64, validatorID int, at time.Time) {
	d.RoundsMu.RLock()
	var r *RoundStatus
	for _, rr := range d.Rounds { if rr.Round==round { r=rr; break } }
	d.RoundsMu.RUnlock()
	if r==nil { return }
	r.mu.Lock()
	if _, ok := r.Voted[validatorID]; !ok {
		r.Voted[validatorID] = at.Sub(r.StartedAt)
	}
	r.mu.Unlock()
}

func (d *Dashboard) Finalize(round uint64, at time.Time) {
	d.RoundsMu.RLock()
	var r *RoundStatus
	for _, rr := range d.Rounds { if rr.Round==round { r=rr; break } }
	d.RoundsMu.RUnlock()
	if r==nil { return }
	r.mu.Lock()
	r.Final = true
	r.FinalizedAt = at
	r.mu.Unlock()
}

func (d *Dashboard) StartRender() {
	hideCursor()
	go func() {
		for {
			select {
			case <-d.RenderTick.C:
				d.render()
			case <-d.Quit:
				showCursor()
				return
			}
		}
	}()
}

func (d *Dashboard) render() {
	moveHome()
	clearScreen()
	fmt.Printf("%sSesa Instant Finality – Live Monitor%s\n", bold, clr)
	fmt.Printf("%s%s%s\n\n", grey, padRight(d.Title, 80), clr)

	d.RoundsMu.RLock()
	defer d.RoundsMu.RUnlock()

	for _, r := range d.Rounds {
		r.mu.RLock()
		elapsed := time.Since(r.StartedAt)
		voted := len(r.Voted)

		// Header line per round
		state := fmt.Sprintf("%sP%d%s", cyan, r.ProposerID, clr)
		if r.Final {
			state = fmt.Sprintf("%sFINAL%s", green, clr)
			elapsed = r.FinalizedAt.Sub(r.StartedAt)
		}
		fmt.Printf("%sRound %-2d%s  | Proposer: %s | Quorum: %d/%d | Elapsed: %s\n",
			bold, r.Round, clr, state, r.Quorum, voted, fmt.Sprintf("%v", elapsed).PadRight(10, ' '))

		// Progress bar
		barW := 50
		fill := int(float64(voted) / float64(r.Quorum) * float64(barW))
		if fill > barW { fill = barW }
		bar := strings.Repeat("█", fill) + strings.Repeat("░", barW-fill)
		col := yellow
		if r.Final { col = green }
		fmt.Printf("   %s[%s]%s  %d/%d\n", col, bar, clr, voted, r.Quorum)

		// Validator vote table (first line shows IDs; second shows ✓ / · and ms)
		ids := make([]string, r.Total)
		marks := make([]string, r.Total)
		for i := 0; i < r.Total; i++ {
			ids[i] = fmt.Sprintf("v%-2d", i)
			if lat, ok := r.Voted[i]; ok {
				ms := int(lat / time.Millisecond)
				marks[i] = fmt.Sprintf("%s✓%s(%dms)", green, clr, ms)
			} else {
				marks[i] = fmt.Sprintf("%s·%s", grey, clr)
			}
		}
		fmt.Printf("   %s\n", strings.Join(ids, "  "))
		fmt.Printf("   %s\n\n", strings.Join(marks, "  "))

		r.mu.RUnlock()
	}
}

// helper for formatting
func (s string) PadRight(w int, pad rune) string {
	if len(s) >= w { return s }
	return s + strings.Repeat(string(pad), w-len(s))
}

// ─────────────────────────────────────────────────────────────────────────────
// Utilities
// ─────────────────────────────────────────────────────────────────────────────
func bigFromUint64(v uint64) *big.Int { b := new(big.Int); b.SetUint64(v); return b }

// ─────────────────────────────────────────────────────────────────────────────
// Main
// ─────────────────────────────────────────────────────────────────────────────
func main() {
	title := "Concurrent Gossip with Adaptive Quorum (target ~100ms)"
	dbg := NewDashboard(title)
	dbg.StartRender()
	defer func() { close(dbg.Quit); showCursor() }()

	const validatorCount = 10   // try 5, 10, 30…
	const blockRounds    = 6

	rand.Seed(time.Now().UnixNano())

	// Create validators + keys
	validators := make([]*DemoValidator, 0, validatorCount)
	for i := 0; i < validatorCount; i++ {
		sk := new(bls.Fr).SetUint64(uint64(rand.Intn(1e9)))
		pk := new(bls.G1).ScalarBaseMult(sk)
		addr := common.BigToAddress(bigFromUint64(uint64(1000 + i)))
		v := &DemoValidator{
			ID:      i,
			Addr:    addr,
			PrivKey: sk,
			PubKey:  pk,
			RecvCh:  make(chan interface{}, 256),
		}
		validators = append(validators, v)
	}

	// Full mesh peers
	for _, v := range validators {
		for _, p := range validators {
			if v != p { v.Peers = append(v.Peers, p) }
		}
	}

	// Shared validator set
	vset := &LocalValidatorSet{Vals: validators}

	// Engines + listeners
	for _, v := range validators {
		tr := &GossipTransport{v: v}
		blsAdapter := &hotstuff.BLSAdapter{PrivKey: v.PrivKey, PubKey: v.PubKey}
		cfg := hotstuff.Config{BaseTimeout: 100 * time.Millisecond}
		v.Engine = hotstuff.New(cfg, vset, tr, blsAdapter)
		go func(v *DemoValidator) {
			for msg := range v.RecvCh {
				switch m := msg.(type) {
				case hotstuff.ProposeMsg:
					v.Engine.OnPropose(&m)
				case hotstuff.VoteMsg:
					// record vote arrival for dashboard
					dbg.UpdateVote(m.Round, int(m.VoterIndex), time.Now())
					v.Engine.OnVote(&m)
				case hotstuff.CommitMsg:
					v.Engine.OnCommit(&m)
				}
			}
		}(v)
	}

	// Round loop
	for round := uint64(1); round <= uint64(blockRounds); round++ {
		proposer := validators[int(round-1)%validatorCount]

		// compute quorum from active set (same as gadget will do)
		total := validatorCount
		var quorum int
		switch {
		case total <= 3:
			quorum = total
		case total <= 10:
			quorum = int((3*total + 3) / 4) // ceil(0.75*n)
		default:
			quorum = int((2*total + 2) / 3) // ceil(2/3*n)
		}

		rs := &RoundStatus{
			Round:      round,
			ProposerID: proposer.ID,
			StartedAt:  time.Now(),
			Quorum:     quorum,
			Total:      total,
			Voted:      make(map[int]time.Duration),
		}
		dbg.AddRound(rs)

		header := &types.Header{
			Number:     bigFromUint64(round),
			Coinbase:   proposer.Addr,
			Time:       uint64(time.Now().Unix()),
			ParentHash: common.HexToHash(fmt.Sprintf("%064x", round-1)),
		}

		start := time.Now()
		cc, err := proposer.Engine.Propose(context.Background(), header, round)
		if err != nil {
			fmt.Printf("%sRound %d finality failed: %v%s\n", red, round, err, clr)
		} else {
			dbg.Finalize(round, time.Now())
			_ = cc // not used in this view; kept for completeness
			_ = start
		}
		time.Sleep(450 * time.Millisecond) // spacing between rounds
	}

	time.Sleep(2 * time.Second)
	fmt.Printf("%s\nDone. Press Ctrl+C to exit.%s\n", grey, clr)
}
