package ethapi

import (
    "context"

    "github.com/ethereum/go-ethereum/rpc"
    "github.com/sesafoundation/sesn/preconf"
)

type PublicPreconfSubscriptionAPI struct {
    backend Backend
}

func NewPublicPreconfSubscriptionAPI(b Backend) *PublicPreconfSubscriptionAPI {
    return &PublicPreconfSubscriptionAPI{backend: b}
}

// eth_subscribe: "preconf"
func (api *PublicPreconfSubscriptionAPI) SubscribePreconf(ctx context.Context) (*rpc.Subscription, error) {
    eb, ok := api.backend.(*EthAPIBackend)
    if !ok {
        return nil, rpc.ErrNotificationsUnsupported
    }
    notifier, supported := rpc.NotifierFromContext(ctx)
    if !supported {
        return nil, rpc.ErrNotificationsUnsupported
    }
    sub := notifier.CreateSubscription()

    go func() {
        ch := make(chan *preconf.PreconfReceipt, 128)
        subErr := eb.preconfFeed.Subscribe(ch)
        if subErr != nil {
            notifier.Notify(sub.ID, subErr)
            return
        }

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
