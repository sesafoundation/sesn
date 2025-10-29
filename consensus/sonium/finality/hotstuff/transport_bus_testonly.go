//go:build devtest

package hotstuff

import "github.com/sesafoundation/sesn/common"

type busP2P struct {
	self     common.Address
	handlers map[string]func(from common.Address, payload []byte)
	bcast    func(topic string, payload []byte)
}

func newBus(self common.Address) *busP2P {
	b := &busP2P{self: self, handlers: map[string]func(common.Address, []byte){}}
	b.bcast = func(topic string, payload []byte) {
		if h, ok := b.handlers[topic]; ok {
			h(self, payload)
		}
	}
	return b
}

func (b *busP2P) Broadcast(topic string, payload []byte) error {
	b.bcast(topic, payload)
	return nil
}
func (b *busP2P) Subscribe(topic string, h func(from common.Address, payload []byte)) error {
	b.handlers[topic] = h
	return nil
}
func (b *busP2P) Self() common.Address { return b.self }
