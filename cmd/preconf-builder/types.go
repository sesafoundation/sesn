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

type BuilderConfig struct {
	Cadence       time.Duration // e.g. 150ms
	MaxTxPerSlice int
	GasSlice      uint64
	IPCPath       string        // e.g. "~/.ethereum/geth.ipc"
	WSListen      string        // e.g. ":8556"
	HTTPListen    string        // e.g. ":8557" (for getReceipt HTTP JSON-RPC)
	NetworkID     uint64        // chainId
}
