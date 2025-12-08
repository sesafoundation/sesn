package eth

import (
    "context"
    "github.com/sesafoundation/sesn/common"
)

type PublicQuantAPI struct {
    eth *Ethereum
}

func NewPublicQuantAPI(eth *Ethereum) *PublicQuantAPI {
    return &PublicQuantAPI{eth: eth}
}

func (api *PublicQuantAPI) GetQuantStatus(ctx context.Context, hash common.Hash) (string, error) {
    return api.eth.quantStatus(hash)
}
