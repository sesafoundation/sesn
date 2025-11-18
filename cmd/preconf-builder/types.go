package main

import (
	"crypto/ecdsa"
	"time"

	"github.com/sesafoundation/sesn/common"
)

type MiniBlock struct {
	ID           uint64         `json:"id"`
	ParentBlock  common.Hash    `json:"parentBlock"`
	TimestampMs  int64          `json:"timestampMs"`
	TxHashes     []common.Hash  `json:"txHashes"`
	GasPlanned   uint64         `json:"gasPlanned"`
	Signer       common.Address `json:"signer"`
	Signature    []byte         `json:"signature"`       // secp256k1 (R||S||V)
	AggregateBLS []byte         `json:"aggregateBls,omitempty"`
}

type PreconfReceipt struct {
	TxHash      common.Hash    `json:"txHash"`
	MiniBlockID uint64         `json:"miniBlockId"`
	Signer      common.Address `json:"signer"`
	Signature   []byte         `json:"signature"`
}

type ProposerKey struct {
	Address common.Address
	ECDSA   *ecdsa.PrivateKey
}

type RawConfig struct {
    Builder struct {
        IPCPath       string
        Cadence       string
        MaxTxPerSlice int
        GasSlice      uint64
        WSListen      string
        HTTPListen    string
        NetworkID     uint64
        KeystorePath  string
        KeyPassword   string
    }
}

// runtime config struct used by builder
type BuilderConfig struct {
    IPCPath       string
    Cadence       time.Duration
    MaxTxPerSlice int
    GasSlice      uint64
    WSListen      string
    HTTPListen    string
    NetworkID     uint64
    KeystorePath  string
    KeyPassword   string
}