package preconfapi

import (
    "context"

    "github.com/sesafoundation/sesn/common"
    "github.com/sesafoundation/sesn/preconf"
)

type PublicPreconfAPI struct {
    backend PreconfBackend
}

func NewPublicPreconfAPI(b PreconfBackend) *PublicPreconfAPI {
    return &PublicPreconfAPI{backend: b}
}

func (api *PublicPreconfAPI) GetPreconfReceipt(ctx context.Context, txHash common.Hash) (*preconf.PreconfReceipt, error) {
    return api.backend.LoadPreconfReceipt(txHash), nil
}

