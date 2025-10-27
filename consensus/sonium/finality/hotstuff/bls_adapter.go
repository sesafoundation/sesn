package hotstuff

import (
	"crypto/rand"
	"fmt"

	bls "github.com/kilic/bls12-381"
)

// BLSAdapter provides a simple BLS key pair for HotStuff finality.
type BLSAdapter struct {
	PrivKey *bls.Fr
	PubKey  *bls.PointG1
}

// NewBLSAdapter creates a random BLS key pair.
func NewBLSAdapter() *BLSAdapter {
	g1 := bls.NewG1()
	fr := bls.NewFr()

	// 1. Random 32-byte seed
	skBytes := make([]byte, 32)
	if _, err := rand.Read(skBytes); err != nil {
		panic(fmt.Sprintf("failed to read randomness: %v", err))
	}

	// 2. Convert bytes → scalar (Fr element)
	sk := fr.FromBytes(skBytes)
	if sk == nil {
		// fallback if bytes are invalid
		sk = fr.One()
	}

	// 3. Compute pk = G1 generator * sk
	pk := g1.New()
	g1.MulScalar(pk, g1.One(), sk)

	return &BLSAdapter{
		PrivKey: sk,
		PubKey:  pk,
	}
}


