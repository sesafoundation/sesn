package preconfapi

import (
    "github.com/sesafoundation/sesn/common"
    "github.com/sesafoundation/sesn/preconf"
)

type PreconfBackend interface {
    GetPreconfReceipt(hash common.Hash) (*preconf.PreconfReceipt, bool)
    PreconfSubscribe(chan *preconf.PreconfReceipt) event.Subscription
}
