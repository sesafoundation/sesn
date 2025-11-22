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
