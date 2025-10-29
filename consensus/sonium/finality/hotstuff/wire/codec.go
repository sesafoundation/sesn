package wire

import (
	"fmt"

	"google.golang.org/protobuf/proto"
	"github.com/sesafoundation/sesn/consensus/sonium/finality/hotstuff"
)

type (
    Propose    = hotstuff.ProposeMsg
    Vote       = hotstuff.VoteMsg
    Commit     = hotstuff.CommitMsg
    QuorumCert = hotstuff.QuorumCert
)
// Topics (versioned)
const (
	TopicBase       = "sesa/hotstuff/1"
	TopicPropose    = TopicBase + "/propose"
	TopicVote       = TopicBase + "/vote"
	TopicCommit     = TopicBase + "/commit"
	TopicQuorumCert = TopicBase + "/qc"
)

func MustMarshal(m proto.Message) []byte {
	b, err := proto.Marshal(m)
	if err != nil {
		panic(err)
	}
	return b
}

func Marshal(m proto.Message) ([]byte, error) {
	return proto.Marshal(m)
}

func UnmarshalPropose(b []byte) (*Propose, error) {
	var m Propose
	if err := proto.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("decode propose: %w", err)
	}
	return &m, nil
}

func UnmarshalVote(b []byte) (*Vote, error) {
	var m Vote
	if err := proto.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("decode vote: %w", err)
	}
	return &m, nil
}

func UnmarshalCommit(b []byte) (*Commit, error) {
	var m Commit
	if err := proto.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("decode commit: %w", err)
	}
	return &m, nil
}

func UnmarshalQC(b []byte) (*QuorumCert, error) {
	var m QuorumCert
	if err := proto.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("decode qc: %w", err)
	}
	return &m, nil
}
