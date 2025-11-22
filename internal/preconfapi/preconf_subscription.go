package preconfapi

import (
    "context"

    "github.com/ethereum/go-ethereum/rpc"
    "github.com/sesafoundation/sesn/preconf"
    "github.com/sesafoundation/sesn/internal/ethapi"
)

type PublicPreconfSubscriptionAPI struct {
    backend *ethapi.EthAPIBackend
}

func NewPublicPreconfSubscriptionAPI(b *ethapi.EthAPIBackend) *PublicPreconfSubscriptionAPI {
    return &PublicPreconfSubscriptionAPI{backend: b}
}

// RPC: eth_subscribe "preconfReceipts"
func (api *PublicPreconfSubscriptionAPI) SubscribePreconf(ctx context.Context) (*rpc.Subscription, error) {
    if api.backend == nil {
        return nil, rpc.ErrNotificationsUnsupported
    }

    notifier, ok := rpc.NotifierFromContext(ctx)
    if !ok {
        return nil, rpc.ErrNotificationsUnsupported
    }
    sub := notifier.CreateSubscription()

    ch := make(chan *preconf.PreconfReceipt, 128)
    api.backend.PreconfFeed.Subscribe(ch)

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
