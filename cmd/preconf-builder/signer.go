package main

import (
	"crypto/ecdsa"
	"encoding/binary"
	"github.com/sesafoundation/sesn/log"

	 "os"
   // "os/signal"
	//"golang.org/x/crypto/sha3"
	//"github.com/sesafoundation/sesn/preconf/builder"


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
	//hasher := crypto.NewKeccakState()
//	hasher := crypto.NewKeccakState()

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

func oldsignMiniBlock(priv *ecdsa.PrivateKey, mb *MiniBlock) []byte {
	d := miniBlockDigest(mb)
	sig, err := crypto.Sign(d, priv)
	if err != nil {
		panic(err)
	}
	return sig // 65 bytes (R||S||V)
}

func signMiniBlock(key *ecdsa.PrivateKey, mb *MiniBlock) []byte {
    data := append(mb.ParentBlock[:], byte(mb.ID))
    hash := crypto.Keccak256Hash(data)
    sig, err := crypto.Sign(hash.Bytes(), key)
    if err != nil {
        log.Warn("Failed to sign mini-block", "err", err)
        return nil
    }
    return sig
}

func loadProposerKey(keystorePath, passwordFile string) *ecdsa.PrivateKey {
    // Read password from file
    pwBytes, err := os.ReadFile(passwordFile)
    if err != nil {
        log.Crit("Cannot read password file", "err", err)
    }
    password := strings.TrimSpace(string(pwBytes)) // remove newline

    // Read keystore JSON file
    jsonData, err := os.ReadFile(keystorePath)
    if err != nil {
        log.Crit("Cannot read keystore JSON", "err", err)
    }

    // Create keystore instance only for decrypt helper
    ks := keystore.NewKeyStore(filepath.Dir(keystorePath), keystore.StandardScryptN, keystore.StandardScryptP)

    // Decrypt
    keyObj, err := ks.DecryptKey(jsonData, password)
    if err != nil {
        log.Crit("Invalid keystore password", "err", err)
    }

    return keyObj.PrivateKey
}