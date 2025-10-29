package wire

import (
	"bytes"
	"fmt"

	"google.golang.org/protobuf/proto"
)

type Propose struct {
	Round   uint64
	Height  uint64
	Hash    [32]byte
	Payload []byte
}

type Vote struct {
	Round       uint64
	Height      uint64
	VoterIndex  uint32
	Signature   []byte
}

type Commit struct {
	Round  uint64
	Height uint64
	QC     []byte
}

type QuorumCert struct {
	Height uint64
	Round  uint64
	Signers []uint32
	SigAgg  []byte
}

// ----------------------------------------------------------------------
// Generic GOB codec (used before protobuf integration)
// ----------------------------------------------------------------------

func Encode(msg proto.Message) ([]byte, error) {
	return proto.Marshal(msg)
}

func DecodePropose(b []byte) (*Propose, error) {
	var m Propose
	if err := proto.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("decode propose: %w", err)
	}
	return &m, nil
}

func DecodeVote(b []byte) (*Vote, error) {
	var m Vote
	if err := proto.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("decode vote: %w", err)
	}
	return &m, nil
}

func DecodeCommit(b []byte) (*Commit, error) {
	var m Commit
	if err := proto.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("decode commit: %w", err)
	}
	return &m, nil
}

func DecodeQC(b []byte) (*QuorumCert, error) {
	var m QuorumCert
	if err := proto.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("decode qc: %w", err)
	}
	return &m, nil
}

// Optional convenience wrappers
func MustEncode(msg proto.Message) []byte {
	data, err := Encode(msg)
	if err != nil {
		panic(err)
	}
	return data
}

func CloneMessage(msg proto.Message) proto.Message {
	data, _ := Encode(msg)
	cl := proto.Clone(msg)
	proto.Unmarshal(data, cl)
	return cl
}

// Pretty print (for logs)
func Dump(msg proto.Message) string {
	data, _ := proto.MarshalOptions{Multiline: true}.Marshal(msg)
	return string(bytes.TrimSpace(data))
}