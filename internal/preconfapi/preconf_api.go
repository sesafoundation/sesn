package preconfapi

import (
    "context"

    "github.com/sesafoundation/sesn/common"
    "github.com/sesafoundation/sesn/preconf"
)

// This interface is implemented by EthAPIBackend (added in backend.go)
//type PreconfBackend interface {
//    GetPreconfReceipt(hash common.Hash) (*preconf.PreconfReceipt, bool)
//}

type PublicPreconfAPI struct {
    backend PreconfBackend
}

func NewPublicPreconfAPI(b PreconfBackend) *PublicPreconfAPI {
    return &PublicPreconfAPI{backend: b}
}

func (api *PublicPreconfAPI) GetPreconfReceipt(ctx context.Context, txHash common.Hash) (*preconf.PreconfReceipt, error) {
    if api.backend == nil {
        return nil, nil
    }
    r, _ := api.backend.GetPreconfReceipt(txHash)
    return r, nil
}
