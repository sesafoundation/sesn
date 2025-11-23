
package preconfapi

import (
    "github.com/ethereum/go-ethereum/event"
    "github.com/sesafoundation/sesn/common"
    "github.com/sesafoundation/sesn/preconf"
)

type PreconfBackend interface {
    StorePreconfReceipt(common.Hash, *preconf.PreconfReceipt)
    LoadPreconfReceipt(common.Hash) *preconf.PreconfReceipt
    PreconfSubscribe(chan *preconf.PreconfReceipt) event.Subscription
}

