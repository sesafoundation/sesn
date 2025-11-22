package miner

import (
    "github.com/sesafoundation/sesn/common"
    "github.com/sesafoundation/sesn/preconf"
)

type PreconfBackend interface {
    StorePreconfReceipt(hash common.Hash, r *preconf.PreconfReceipt)
}
