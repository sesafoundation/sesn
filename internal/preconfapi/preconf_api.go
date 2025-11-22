package preconfapi

import (
    "context"

    "github.com/sesafoundation/sesn/common"
    "github.com/sesafoundation/sesn/preconf"
    "github.com/sesafoundation/sesn/internal/ethapi" // only for type reference, no import cycle because this file is outside ethapi
)

type PublicPreconfAPI struct {
    backend *ethapi.EthAPIBackend
}

// The backend is injected from backend.go
func NewPublicPreconfAPI(b *ethapi.EthAPIBackend) *PublicPreconfAPI {
    return &PublicPreconfAPI{backend: b}
}

func (api *PublicPreconfAPI) GetPreconfReceipt(ctx context.Context, txHash common.Hash) (*preconf.PreconfReceipt, error) {
    if api.backend == nil {
        return nil, nil
    }

    api.backend.PreconfMu.RLock()
    r := api.backend.PreconfReceipts[txHash]
    api.backend.PreconfMu.RUnlock()

    return r, nil
}
