package preconfapi

import (
    "context"

    "github.com/ethereum/go-ethereum/rpc"
    "github.com/sesafoundation/sesn/preconf"
)
 
// WebSocket subscription API → preconf events
type PublicPreconfSubscriptionAPI struct {
    //backend *EthAPIBackend
	backend, ok := api.backend.(*ethapi.EthAPIBackend)
}

// Constructor
func NewPublicPreconfSubscriptionAPI(b *EthAPIBackend) *PublicPreconfSubscriptionAPI {
    return &PublicPreconfSubscriptionAPI{backend: b}
}

// WebSocket subscription handler: preconf_subscribe
func (api *PublicPreconfSubscriptionAPI) SubscribePreconf(ctx context.Context) (*rpc.Subscription, error) {
    if api.backend == nil {
        return nil, rpc.ErrNotificationsUnsupported
    }

    notifier, ok := rpc.NotifierFromContext(ctx)
    if !ok {
        return nil, rpc.ErrNotificationsUnsupported
    }

    sub := notifier.CreateSubscription()

    // Feed channel for mini-block receipts
    ch := make(chan *preconf.PreconfReceipt, 128)
    api.backend.PreconfFeed.Subscribe(ch)

    go func() {
        defer api.backend.PreconfFeed.Unsubscribe(ch)

        for {
            select {
            case <-ctx.Done():
                return
            case r := <-ch:
                notifier.Notify(sub.ID, r)
            }
        }
    }()

    return sub, nil
}
