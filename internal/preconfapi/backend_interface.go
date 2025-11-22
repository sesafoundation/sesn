package preconfapi

import (
    "github.com/ethereum/go-ethereum/event"
    "github.com/sesafoundation/sesn/common"
    "github.com/sesafoundation/sesn/preconf"
)

// The only PreconfBackend interface used by APIs
type PreconfBackend interface {
    // store + fetch
    StorePreconfReceipt(common.Hash, *preconf.PreconfReceipt)
    LoadPreconfReceipt(common.Hash) *preconf.PreconfReceipt

    // subscription
    PreconfSubscribe(chan *preconf.PreconfReceipt) event.Subscription
}
