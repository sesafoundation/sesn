package preconf

import "github.com/sesafoundation/sesn/common"

type MiniBlock struct {
    ID          uint64
    TxHashes    []common.Hash
    TimestampMs int64
    SignerAddr  common.Address
}
