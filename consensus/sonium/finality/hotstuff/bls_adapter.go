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


func (a *BLSAdapter) Sign(msg []byte) []byte {
	h, err := a.g2.HashToCurve(msg, []byte("sesa-domain"))
	if err != nil {
		return nil
	}
	sig := a.g2.New()
	a.g2.MulScalar(sig, h, a.PrivKey)
	return a.g2.ToCompressed(sig)
}

// ──────────────────────────────────────────────────────────────
// Aggregate multiple signatures into one
// ──────────────────────────────────────────────────────────────
func (a *BLSAdapter) Aggregate(sigs [][]byte) []byte {
	if len(sigs) == 0 {
		return nil
	}
	sum := a.g1.New()
	for _, sb := range sigs {
		tmp, err := a.g1.FromCompressed(sb)
		if err != nil {
			continue
		}
		a.g1.Add(sum, sum, tmp)
	}
	return a.g1.ToCompressed(sum)
}

// Verify a single BLS signature (PK in G1, SIG in G2)
func (a *BLSAdapter) Verify(pubBytes []byte, msg []byte, sigBytes []byte) bool {
	pk, err := a.g1.FromCompressed(pubBytes)
	if err != nil {
		return false
	}
	sig, err := a.g2.FromCompressed(sigBytes)
	if err != nil {
		return false
	}
	h, err := a.g2.HashToCurve(msg, []byte("sesa-domain"))
	if err != nil {
		return false
	}
	engine := bls.NewEngine()
	engine.AddPair(pk, h)
	engine.AddPairInv(a.g1.One(), sig)
	return engine.Check()
}

// Verify aggregate signature (same message for all)
func (a *BLSAdapter) VerifyAggregate(pubKeys [][]byte, msg []byte, aggSig []byte) bool {
	if len(pubKeys) == 0 {
		return false
	}

	aggPK := a.g1.New()
	for _, pb := range pubKeys {
		tmp, err := a.g1.FromCompressed(pb)
		if err != nil {
			continue
		}
		a.g1.Add(aggPK, aggPK, tmp)
	}

	sig, err := a.g2.FromCompressed(aggSig)
	if err != nil {
		return false
	}
	h, err := a.g2.HashToCurve(msg, []byte("sesa-domain"))
	if err != nil {
		return false
	}

	engine := bls.NewEngine()
	engine.AddPair(aggPK, h)
	engine.AddPairInv(a.g1.One(), sig)
	return engine.Check()
}
