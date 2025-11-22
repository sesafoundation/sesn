package ethapi

import (
    "context"

    "github.com/ethereum/go-ethereum/rpc"
    "github.com/sesafoundation/sesn/preconf"
)

//type PublicPreconfSubscriptionAPI struct {
//    backend Backend
//}

type PublicPreconfSubscriptionAPI struct {
    b Backend
}

func NewPublicPreconfSubscriptionAPI(b Backend) *PublicPreconfSubscriptionAPI {
    return &PublicPreconfSubscriptionAPI{backend: b}
}

func (api *PublicPreconfSubscriptionAPI) SubscribePreconf(ctx context.Context) (*rpc.Subscription, error) {
    eb, ok := api.b.(*EthAPIBackend)
    if !ok {
        return nil, rpc.ErrNotificationsUnsupported
    }

    notifier, ok := rpc.NotifierFromContext(ctx)
    if !ok {
        return nil, rpc.ErrNotificationsUnsupported
    }
    sub := notifier.CreateSubscription()

    ch := make(chan *preconf.PreconfReceipt, 128)
    eb.preconfFeed.Subscribe(ch)

    go func() {
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
