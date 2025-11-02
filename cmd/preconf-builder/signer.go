package main

import (
	"crypto/ecdsa"
	"encoding/binary"

	"github.com/sesafoundation/sesn/common"
	"github.com/sesafoundation/sesn/crypto"
)

func deriveAddress(k *ecdsa.PrivateKey) common.Address {
	return crypto.PubkeyToAddress(k.PublicKey)
}

// hash over a compact, deterministic encoding of MiniBlock header+tx list
func miniBlockDigest(mb *MiniBlock) []byte {
	// keccak256( 0x01 || parentHash || id || ts || gasPlanned || signer || keccak256(txHashes...) )
	txh := make([]byte, 0, 32*len(mb.TxHashes))
	hasher := crypto.NewKeccakState()
	for _, h := range mb.TxHashes {
		txh = append(txh, h.Bytes()...)
	}
	txHashesDigest := crypto.Keccak256(txh)

	buf := make([]byte, 0, 1+32+8+8+8+20+32)
	buf = append(buf, 0x01)
	buf = append(buf, mb.ParentBlock.Bytes()...)
	tmp := make([]byte, 8)
	binary.BigEndian.PutUint64(tmp, mb.ID)
	buf = append(buf, tmp...)
	binary.BigEndian.PutUint64(tmp, uint64(mb.TimestampMs))
	buf = append(buf, tmp...)
	binary.BigEndian.PutUint64(tmp, mb.GasPlanned)
	buf = append(buf, tmp...)
	buf = append(buf, mb.Signer.Bytes()...)
	buf = append(buf, txHashesDigest...)
	sum := crypto.Keccak256(buf)
	return sum
}

func signMiniBlock(priv *ecdsa.PrivateKey, mb *MiniBlock) []byte {
	d := miniBlockDigest(mb)
	sig, err := crypto.Sign(d, priv)
	if err != nil {
		panic(err)
	}
	return sig // 65 bytes (R||S||V)
}

// ---- demo key loader (replace with keystore/HSM) ----
func loadProposerKey() *ecdsa.PrivateKey {
	// For POC ONLY: use env var PRIVATE_KEY or similar in your codebase.
	// Here, panic to force you to plug in your own loader.
	panic("implement loadProposerKey(): load from keystore/HSM/env")
}
