package hotstuff

import (
	"crypto/rand"
	"fmt"

	bls "github.com/kilic/bls12-381"
)

// ──────────────────────────────────────────────────────────────
// BLS interface expected by HotStuff engine
// ──────────────────────────────────────────────────────────────
type BLS interface {
	Sign(msg []byte) []byte
	Verify(pub []byte, msg []byte, sig []byte) bool
	Aggregate(sigs [][]byte) []byte
	VerifyAggregate(pubs [][]byte, msg []byte, aggSig []byte) bool
}

// ──────────────────────────────────────────────────────────────
// BLSAdapter implements the BLS interface for HotStuff finality
// ──────────────────────────────────────────────────────────────
type BLSAdapter struct {
	PrivKey *bls.Fr
	PubKey  *bls.PointG1
	g1      *bls.G1
	g2      *bls.G2
}

// NewBLSAdapter generates a new random BLS keypair.
func NewBLSAdapter() *BLSAdapter {
	g1 := bls.NewG1()
	fr := bls.NewFr()

	skBytes := make([]byte, 32)
	if _, err := rand.Read(skBytes); err != nil {
		panic(fmt.Sprintf("failed to generate randomness: %v", err))
	}
	sk := fr.FromBytes(skBytes)
	if sk == nil {
		sk = fr.One()
	}

	pk := g1.New()
	g1.MulScalar(pk, g1.One(), sk)

	return &BLSAdapter{
		PrivKey: sk,
		PubKey:  pk,
		g1:      g1,
		g2:      bls.NewG2(),
	}
}

// ──────────────────────────────────────────────────────────────
// Sign a message (BLS signature over G1)
// ──────────────────────────────────────────────────────────────
func (a *BLSAdapter) Sign(msg []byte) []byte {
	if a.g1 == nil {
		a.g1 = bls.NewG1()
	}
	// Hash message to curve, multiply by private key
	h := a.g1.HashToCurve(msg, []byte("sesa-domain"))
	sig := a.g1.New()
	a.g1.MulScalar(sig, h, a.PrivKey)
	return a.g1.ToCompressed(sig)
}

// ──────────────────────────────────────────────────────────────
// Verify a single BLS signature
// ──────────────────────────────────────────────────────────────
func (a *BLSAdapter) Verify(pubBytes []byte, msg []byte, sigBytes []byte) bool {
	if a.g1 == nil {
		a.g1 = bls.NewG1()
		a.g2 = bls.NewG2()
	}

	pk := a.g1.New()
	sig := a.g1.New()
	if err := a.g1.FromCompressed(pk, pubBytes); err != nil {
		return false
	}
	if err := a.g1.FromCompressed(sig, sigBytes); err != nil {
		return false
	}

	h := a.g1.HashToCurve(msg, []byte("sesa-domain"))

	engine := bls.NewEngine()
	engine.AddPair(sig, a.g2.

