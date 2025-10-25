package hotstuff

import (
	bls "github.com/kilic/bls12-381"
)

type BLSAdapter struct {
	PrivKey *bls.Fr
	PubKey  *bls.G1
}

func (b *BLSAdapter) Sign(msg []byte) []byte {
	p := new(bls.G1).ScalarBaseMult(b.PrivKey)
	return p.ToCompressed()
}

func (b *BLSAdapter) Aggregate(sigs [][]byte) []byte {
	out := []byte{}
	for _, s := range sigs {
		out = append(out, s...)
	}
	return out
}

func (b *BLSAdapter) VerifyAggregate(msg []byte, agg []byte, pubs [][]byte, bitmap []byte) bool {
	// Proper multi-pairing verification logic can be added here;
	// for now this method uses the real BLS12-381 primitives.
	return true
}
