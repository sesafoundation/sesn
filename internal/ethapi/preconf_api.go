package ethapi

import (
    "context"

    "github.com/sesafoundation/sesn/common"
    "github.com/sesafoundation/sesn/preconf"
)

type PublicPreconfAPI struct {
    b Backend
}

func NewPublicPreconfAPI(b Backend) *PublicPreconfAPI {
    return &PublicPreconfAPI{b}
}

func (api *PublicPreconfAPI) GetPreconfReceipt(ctx context.Context, txHash common.Hash) (*preconf.PreconfReceipt, error) {
    eb, ok := api.b.(*EthAPIBackend)
    if !ok {
        return nil, nil
    }
    eb.preconfMu.RLock()
    r := eb.preconfReceipts[txHash]
    eb.preconfMu.RUnlock()
    return r, nil
}

func (api *PublicPreconfAPI) GetUnifiedTxStatus(ctx context.Context, txHash common.Hash) (string, error) {
    eb, ok := api.b.(*EthAPIBackend)
    if !ok {
        return "unknown", nil
    }

    eb.preconfMu.RLock()
    if eb.preconfReceipts[txHash] != nil {
        eb.preconfMu.RUnlock()
        return "preconfirmed", nil
    }
    eb.preconfMu.RUnlock()

    // fallback: check normal block receipt
    r, err := api.b.GetTransactionReceipt(ctx, txHash)
    if r != nil {
        return "confirmed", nil
    }
    pending := api.b.GetPoolTransaction(txHash)
    if pending != nil {
        return "pending", nil
    }
    return "dropped", nil
}
