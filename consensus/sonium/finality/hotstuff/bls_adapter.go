package hotstuff

import (
	"crypto/rand"
	"math/big"

	bls "github.com/kilic/bls12-381"
)

// BLSAdapter implements basic BLS key generation and signing.
type BLSAdapter struct {
	PrivKey *bls.Fr
	PubKey  *bls.PointG1
}

// NewBLSAdapter creates a new random BLS keypair.
func NewBLSAdapter() *BLSAdapter {
	engine := bls.NewG1()

	// Generate a random scalar (private key)
	skBytes := make([]byte, 32)
	_, err := rand.Read(skBytes)
	if err != nil {
		panic(err)
	}

	// Convert bytes to field element
	var sk bls.Fr
	if err := sk.FromBytes(skBytes); err != nil {
		// fallback if the random bytes are out of range
		sk.SetUint64(uint64(new(big.Int).SetBytes(skBytes).Uint64()))
	}

	// ✅ FIX: use MulScalar(dst, base, scalar)
	pk := new(bls.PointG1)
	engine.MulScalar(pk, engine.One(), &sk)

	return &BLSAdapter{
		PrivKey: &sk,
		PubKey:  pk,
	}
}
