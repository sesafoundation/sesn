package mfproto

import (
	"io"

	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/consensus/sonium/finality/hotstuff"
)

const (
	ProtoName    = "minifinality"
	ProtoVersion = 1
	CodePropose  = 0x01
	CodeVote     = 0x02
	CodeCommit   = 0x03
)

type transport struct{ h hotstuff.Handler }

func (t *transport) BroadcastPropose(m *hotstuff.ProposeMsg) error { return nil } // TODO send to peers
func (t *transport) BroadcastVote(m *hotstuff.VoteMsg) error       { return nil } // TODO send
func (t *transport) BroadcastCommit(m *hotstuff.CommitMsg) error   { return nil } // TODO send
func (t *transport) RegisterHandler(h hotstuff.Handler)            { t.h = h }

func Protocol(h *hotstuff.Engine) p2p.Protocol {
	tr := &transport{}; tr.RegisterHandler(h)
	return p2p.Protocol{
		Name:    ProtoName, Version: ProtoVersion, Length: 3,
		Run: func(p *p2p.Peer, rw p2p.MsgReadWriter) error {
			for {
				msg, err := rw.ReadMsg()
				if err != nil { if err == io.EOF { return nil }; return err }
				switch msg.Code {
				case CodePropose:
					var pm hotstuff.ProposeMsg; if rlp.Decode(msg.Payload, &pm) == nil { h.OnPropose(&pm) }
				case CodeVote:
					var vm hotstuff.VoteMsg; if rlp.Decode(msg.Payload, &vm) == nil { h.OnVote(&vm) }
				case CodeCommit:
					var cm hotstuff.CommitMsg; if rlp.Decode(msg.Payload, &cm) == nil { h.OnCommit(&cm) }
				}
			}
		},
	}
}

// Expose a Transport for wiring without p2p (in-proc tests)
func NewLocalTransport(h hotstuff.Handler) hotstuff.Transport {
	tr := &transport{}; tr.RegisterHandler(h); return tr
}
