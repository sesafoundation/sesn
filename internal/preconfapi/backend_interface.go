package preconfapi

import (
    "github.com/sesafoundation/sesn/common"
    "github.com/sesafoundation/sesn/preconf"
)

type PreconfBackend interface {
    StorePreconfReceipt(common.Hash, *preconf.PreconfReceipt)
}