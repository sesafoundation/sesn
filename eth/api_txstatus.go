package eth

import (
    "context"
    "github.com/ethereum/go-ethereum/rpc"
)

type TxStatusSubscriptionAPI struct {
    backend *EthAPIBackend
}

func NewTxStatusSubscriptionAPI(b *EthAPIBackend) *TxStatusSubscriptionAPI {
    return &TxStatusSubscriptionAPI{backend: b}
}

func (api *TxStatusSubscriptionAPI) SubscribeTxStatus(ctx context.Context) (*rpc.Subscription, error) {
    notifier, supported := rpc.NotifierFromContext(ctx)
    if !supported {
        return nil, rpc.ErrNotificationsUnsupported
    }
    sub := notifier.CreateSubscription()

    ch := make(chan common.Hash, 256)
    subErr := api.backend.blockStatusFeed.Subscribe(ch) // must add blockStatusFeed to backend

    go func() {
        for {
            select {
            case h := <-ch:
                status, _ := api.backend.publicAPI.GetTransactionStatus(h)
                notifier.Notify(sub.ID, map[string]interface{}{
                    "hash":   h,
                    "status": status,
                })
            case <-subErr:
                return
            case <-sub.Err():
                return
            }
        }
    }()
    return sub, nil
}
