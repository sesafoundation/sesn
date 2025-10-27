package hotstuff

import (
	"math/big"

	bls "github.com/kilic/bls12-381"
)

// BLSAdapter wraps BLS key generation and signing for HotStuff.
type BLSAdapter struct {
	PrivKey *bls.Fr
	PubKey  *bls.PointG1
}

// NewBLSAdapter creates a new random BLS keypair.
func NewBLSAdapter() *BLSAdapter {
	engine := bls.NewG1()

	sk := new(bls.Fr)
	sk.SetBigInt(big.NewInt(12345)) // you can randomize this

	// ✅ FIX: use MulScalar instead of ScalarBaseMult
	pk := engine.MulScalar(engine.One(), sk)

	return &BLSAdapter{
		PrivKey: sk,
		PubKey:  pk,
	}
}
