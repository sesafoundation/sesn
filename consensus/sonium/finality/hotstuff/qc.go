package hotstuff

import (
	"errors"
)

// QuorumCert is the aggregated vote proof for a round.
type QuorumCert struct {
	Round   uint64   // round this QC attests to
	AggSig  []byte   // aggregated signature (BLS G2 compressed)
	PubKeys [][]byte // compressed G1 pubkeys that participated (optional, can be a bitmap in prod)
}

// BuildQC aggregates signatures and verifies them before returning a QC.
// msg must be the exact bytes every validator signed for this round.
func (e *Engine) BuildQC(round uint64, msg []byte, sigs [][]byte, pubs [][]byte) (*QuorumCert, error) {
	if len(sigs) == 0 {
		return nil, errors.New("no signatures to aggregate")
	}
	if len(pubs) == 0 {
		return nil, errors.New("no pubkeys provided")
	}
	agg := e.bls.Aggregate(sigs)
	if len(agg) == 0 {
		return nil, errors.New("aggregate signature empty")
	}
	// Verify aggregated signature against the same message
	if !e.bls.VerifyAggregate(pubs, msg, agg) {
		return nil, errors.New("aggregate signature verification failed")
	}
	return &QuorumCert{
		Round:   round,
		AggSig:  agg,
		PubKeys: pubs,
	}, nil
}

// VerifyQC checks the QC against the provided message.
func (e *Engine) VerifyQC(qc *QuorumCert, msg []byte) bool {
	if qc == nil || len(qc.AggSig) == 0 || len(qc.PubKeys) == 0 {
		return false
	}
	return e.bls.VerifyAggregate(qc.PubKeys, msg, qc.AggSig)
}
