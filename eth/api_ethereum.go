package eth

import (
    "context"
    "github.com/sesafoundation/sesn/internal/ethapi"
)

type PublicEthereumAPI struct {
    backend ethapi.Backend
}

func NewPublicEthereumAPI(backend ethapi.Backend) *PublicEthereumAPI {
    return &PublicEthereumAPI{backend: backend}
}

func (api *PublicEthereumAPI) BlockNumber(ctx context.Context) (hexutil.Uint64, error) {
    return hexutil.Uint64(api.backend.CurrentBlock().NumberU64()), nil
}
