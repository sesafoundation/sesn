package preconfapi

import (
    "context"

    "github.com/ethereum/go-ethereum/rpc"
    "github.com/sesafoundation/sesn/preconf"
)

type PublicPreconfSubscriptionAPI struct {
    backend PreconfBackend
}

func NewPublicPreconfSubscriptionAPI(b PreconfBackend) *PublicPreconfSubscriptionAPI {
    return &PublicPreconfSubscriptionAPI{backend: b}
}

func (api *PublicPreconfSubscriptionAPI) SubscribePreconf(ctx context.Context) (*rpc.Subscription, error) {
    notifier, ok := rpc.NotifierFromContext(ctx)
    if !ok {
        return nil, rpc.ErrNotificationsUnsupported
    }
    sub := notifier.CreateSubscription()

    ch := make(chan *preconf.PreconfReceipt, 128)
    api.backend.PreconfSubscribe(ch)

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
