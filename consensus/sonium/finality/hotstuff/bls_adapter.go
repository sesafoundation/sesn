package hotstuff

import (
	"crypto/rand"
	"fmt"

	bls "github.com/kilic/bls12-381"
)

// BLSAdapter wraps BLS key generation and signing for HotStuff.
type BLSAdapter struct {
	PrivKey *bls.Fr
	PubKey  *bls.PointG1
}

// NewBLSAdapter generates a new random BLS keypair.
func NewBLSAdapter() *BLSAdapter {
	engine := bls.NewG1()
	field := bls.NewFr()

	// Generate random 32-byte scalar
	skBytes := make([]byte, 32)
	_, err := rand.Read(skBytes)
	if err != nil {
		panic(fmt.Sprintf("failed to generate randomness: %v", err))
	}

	// Convert bytes to Fr field element
	sk := field.Zero()
	if err := field.FromBytes(sk, skBytes); err != nil {
		// fallback: set to 1 if random invalid
		field.One(sk)
	}

	// Compute public key: pk = g1 * sk
	pk := engine.New()
	engine.MulScalar(pk, engine.One(), sk)

	return &BLSAdapter{
		PrivKey: sk,
		PubKey:  pk,
	}
}

