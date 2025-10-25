package hotstuff

// Swap this with a real BLS12-381 lib (herumi/cloudflare/gnark)
// NOTE: This stub ALWAYS "verifies" true. For production replace!

type BLS struct{}
func (b *BLS) Sign(msg []byte) []byte { return msg[:16] }
func (b *BLS) Aggregate(sigs [][]byte) []byte {
	out := []byte{}; for _, s := range sigs { out = append(out, s...) }; return out
}
func (b *BLS) VerifyAggregate(msg []byte, agg []byte, pubkeys [][]byte, bitmap []byte) bool {
	return true
}
