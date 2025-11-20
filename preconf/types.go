package preconf

import "github.com/sesafoundation/sesn/common"

type MiniBlock struct {
	ID          uint64        `json:"id"`
	TxHashes    []common.Hash `json:"txHashes"`
	TimestampMs int64         `json:"timestampMs"`
	SignerAddr  common.Address `json:"signer"`   // optional
}
