package finality

import (
	"bytes"
	"github.com/ethereum/go-ethereum/rlp"
)

type Payload struct {
	Round  uint64 // e.g., header.Number or view
	AggSig []byte // aggregated BLS signature
	Bitmap []byte // which validators signed
}

var tag = []byte{0xFA, 0x11} // sentinel

func Append(extra []byte, p Payload) []byte {
	var enc bytes.Buffer
	_ = rlp.Encode(&enc, p)
	return append(append(extra, tag...), enc.Bytes()...)
}

func Extract(extra []byte) (Payload, bool) {
	i := bytes.LastIndex(extra, tag)
	if i < 0 { return Payload{}, false }
	var p Payload
	if rlp.DecodeBytes(extra[i+len(tag):], &p) != nil { return Payload{}, false }
	return p, true
}
