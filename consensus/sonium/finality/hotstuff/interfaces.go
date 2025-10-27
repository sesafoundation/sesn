package hotstuff

import "github.com/sesafoundation/sesn/common"

// ValidatorSet exposes the active validator metadata needed by HotStuff.
type ValidatorSet interface {
	// Active returns the active validator coinbase addresses and their
	// compressed public keys (G1, []byte per validator) in the same order.
	Active() ([]common.Address, [][]byte)
	// IndexOf returns the index of a validator address in the active set.
	IndexOf(a common.Address) (int, bool)
	// SelfCoinbase returns this node’s validator coinbase (local identity).
	SelfCoinbase() common.Address
}

// Transport is the network layer used by the engine to gossip messages.
type Transport interface {
	BroadcastPropose(m *ProposeMsg) error
	BroadcastVote(m *VoteMsg) error
	BroadcastCommit(m *CommitMsg) error
	// RegisterHandler lets the transport deliver inbound messages to the engine.
	RegisterHandler(h Handler)
}

// Handler is implemented by the Engine to receive inbound messages.
type Handler interface {
	OnPropose(m *ProposeMsg)
	OnVote(m *VoteMsg)
	OnCommit(m *CommitMsg)
}
