package hotstuff

// BLSStub is a dummy BLS implementation used only for testing or fallback.
// It satisfies the BLS interface in bls_adapter.go but does no real crypto.
// Always returns "true" for verification.
type BLSStub struct{}

// Sign just returns the first 16 bytes of the message (not secure!)
func (b *BLSStub) Sign(msg []byte) []byte {
	if len(msg) < 16 {
		return append([]byte{}, msg...)
	}
	return append([]byte{}, msg[:16]...)
}

// Aggregate concatenates all signatures.
func (b *BLSStub) Aggregate(sigs [][]byte) []byte {
	var out []byte
	for _, s := range sigs {
		out = append(out, s...)
	}
	return out
}

// Verify always returns true (no real signature checking).
func (b *BLSStub) Verify(pub []byte, msg []byte, sig []byte) bool {
	return true
}

// VerifyAggregate always returns true (no real signature checking).
func (b *BLSStub) VerifyAggregate(pubs [][]byte, msg []byte, aggSig []byte) bool {
	return true
}
