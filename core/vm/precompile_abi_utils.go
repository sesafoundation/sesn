package vm

import (
	"math/big"

	"github.com/sesafoundation/sesn/common"
)

// ------------------------------------------------------------
// Padding helpers
// ------------------------------------------------------------

// padBigTo32 left-pads a big.Int into 32 bytes
func padBigTo32(v *big.Int) []byte {
	out := make([]byte, 32)
	if v != nil && v.Sign() > 0 {
		b := v.Bytes()
		copy(out[32-len(b):], b)
	}
	return out
}

// bytesToBig converts bytes to big.Int
func bytesToBig(b []byte) *big.Int {
	if len(b) == 0 {
		return big.NewInt(0)
	}
	return new(big.Int).SetBytes(b)
}

// ------------------------------------------------------------
// ABI return encoders (minimal EVM ABI encoding)
// ------------------------------------------------------------

// packUint256 encodes uint256 return value
func packUint256(v *big.Int) []byte {
	out := make([]byte, 32)
	if v != nil && v.Sign() > 0 {
		b := v.Bytes()
		copy(out[32-len(b):], b)
	}
	return out
}

// packBool encodes bool return value
func packBool(b bool) []byte {
	out := make([]byte, 32)
	if b {
		out[31] = 1
	}
	return out
}

// packAddress encodes address return value
func packAddress(a common.Address) []byte {
	out := make([]byte, 32)
	copy(out[12:], a[:])
	return out
}

// packString encodes dynamic ABI string return
//
// ABI format:
// offset(32) | length(32) | data(padded)
func packString(s string) []byte {
	b := []byte(s)

	// head: offset = 32
	head := make([]byte, 32)
	head[31] = 32

	// length word
	lw := make([]byte, 32)
	l := big.NewInt(int64(len(b))).Bytes()
	copy(lw[32-len(l):], l)

	// padded string data
	padLen := ((len(b) + 31) / 32) * 32
	data := make([]byte, padLen)
	copy(data, b)

	return append(append(head, lw...), data...)
}

// ------------------------------------------------------------
// Selector helpers
// ------------------------------------------------------------

// equal4 compares 4-byte selectors
func equal4(x []byte, y []byte) bool {
	if len(x) != 4 || len(y) != 4 {
		return false
	}
	for i := 0; i < 4; i++ {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}
