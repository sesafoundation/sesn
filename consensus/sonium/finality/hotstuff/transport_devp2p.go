package hotstuff

import (
	"fmt"

	"github.com/sesafoundation/sesn/common"
	"github.com/sesafoundation/sesn/consensus/sonium/finality/hotstuff/wire"
)

// P2P is a thin adapter to your real network stack.
type P2P interface {
	// Broadcast payload to topic.
	Broadcast(topic string, payload []byte) error
	// Subscribe handler to topic.
	Subscribe(topic string, handler func(from common.Address, payload []byte)) error
	// Local identity address.
	Self() common.Address
}

type Devp2pTransport struct {
	p2p     P2P
	handler Handler
}

func NewDevp2pTransport(p2p P2P) *Devp2pTransport {
	t := &Devp2pTransport{p2p: p2p}
	// Register inbound handlers
	_ = p2p.Subscribe(wire.TopicPropose, t.onPropose)
	_ = p2p.Subscribe(wire.TopicVote, t.onVote)
	_ = p2p.Subscribe(wire.TopicCommit, t.onCommit)
	return t
}

func (t *Devp2pTransport) RegisterHandler(h Handler) { t.handler = h }

func (t *Devp2pTransport) BroadcastPropose(m *ProposeMsg) error {
	pm := &wire.Propose{
		ChainId:   m.ChainID,
		Height:    m.Height,
		View:      m.View,
		HeaderRlp: m.HeaderRLP,
		JustifyQc: m.JustifyQC, // may be nil
	}
	return t.p2p.Broadcast(wire.TopicPropose, wire.MustMarshal(pm))
}

func (t *Devp2pTransport) BroadcastVote(m *VoteMsg) error {
	vm := &wire.Vote{
		ChainId: m.ChainID,
		Height:  m.Height,
		View:    m.View,
		BlockId: m.BlockID[:],
		VoterIx: m.VoterIndex,
		SigG2:   m.Signature,
	}
	return t.p2p.Broadcast(wire.TopicVote, wire.MustMarshal(vm))
}

func (t *Devp2pTransport) BroadcastCommit(m *CommitMsg) error {
	cm := &wire.Commit{
		ChainId: m.ChainID,
		Height:  m.Height,
		View:    m.View,
		BlockId: m.BlockID[:],
		Qc:      m.QC, // proto of QuorumCert
	}
	return t.p2p.Broadcast(wire.TopicCommit, wire.MustMarshal(cm))
}

// ---------------- inbound ----------------

func (t *Devp2pTransport) onPropose(from common.Address, payload []byte) {
	if t.handler == nil {
		return
	}
	pm, err := wire.UnmarshalPropose(payload)
	if err != nil {
		return
	}
	msg := &ProposeMsg{
		ChainID:   pm.ChainId,
		Height:    pm.Height,
		View:      pm.View,
		HeaderRLP: pm.HeaderRlp,
		JustifyQC: pm.JustifyQc,
		From:      from,
	}
	t.handler.OnPropose(msg)
}

func (t *Devp2pTransport) onVote(from common.Address, payload []byte) {
	if t.handler == nil {
		return
	}
	vm, err := wire.UnmarshalVote(payload)
	if err != nil {
		return
	}
	if len(vm.BlockId) != 32 {
		return
	}
	var bid common.Hash
	copy(bid[:], vm.BlockId)
	msg := &VoteMsg{
		ChainID:    vm.ChainId,
		Height:     vm.Height,
		View:       vm.View,
		BlockID:    bid,
		VoterIndex: vm.VoterIx,
		Signature:  vm.SigG2,
		From:       from,
	}
	t.handler.OnVote(msg)
}

func (t *Devp2pTransport) onCommit(from common.Address, payload []byte) {
	if t.handler == nil {
		return
	}
	cm, err := wire.UnmarshalCommit(payload)
	if err != nil {
		return
	}
	if len(cm.BlockId) != 32 {
		return
	}
	var bid common.Hash
	copy(bid[:], cm.BlockId)
	msg := &CommitMsg{
		ChainID: cm.ChainId,
		Height:  cm.Height,
		View:    cm.View,
		BlockID: bid,
		QC:      cm.Qc,
		From:    from,
	}
	t.handler.OnCommit(msg)
}

func (t *Devp2pTransport) String() string {
	return fmt.Sprintf("Devp2pTransport{self=%s}", t.p2p.Self().Hex())
}
