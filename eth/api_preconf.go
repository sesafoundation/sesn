package eth

import (
    "context"

    "github.com/ethereum/go-ethereum/rpc"
    "github.com/sesafoundation/sesn/preconf"
)

type PreconfSubscriptionAPI struct {
    backend *EthAPIBackend
}

func NewPreconfSubscriptionAPI(b *EthAPIBackend) *PreconfSubscriptionAPI {
    return &PreconfSubscriptionAPI{backend: b}
}

// RPC: `preconf_subscribe`
func (api *PreconfSubscriptionAPI) SubscribePreconf(ctx context.Context) (*rpc.Subscription, error) {
    notifier, supported := rpc.NotifierFromContext(ctx)
    if !supported {
        return nil, rpc.ErrNotificationsUnsupported
    }
    sub := notifier.CreateSubscription()

    ch := make(chan *preconf.PreconfReceipt, 256)
    subErr := api.backend.preconfFeed.Subscribe(ch)

    go func() {
        for {
            select {
            case r := <-ch:
                notifier.Notify(sub.ID, r)
            case <-subErr:
                return
            case <-sub.Err():
                return
            }
        }
    }()
    return sub, nil
}
