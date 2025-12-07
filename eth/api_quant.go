// eth/api_quant.go
package eth

import (
	"context"

	"github.com/sesafoundation/sesn/common"
)

// PublicQuantAPI is the public RPC API for QuantBlocks.
type PublicQuantAPI struct {
	eth *Ethereum
}

// NewPublicQuantAPI creates a new RPC service for QuantBlocks.
func NewPublicQuantAPI(eth *Ethereum) *PublicQuantAPI {
	return &PublicQuantAPI{eth: eth}
}

// GetQuantStatus returns the soft-confirmation status of a transaction.
//
// Possible values:
//   - "pending"
//   - "quant-confirmed"
//   - "executed"
//   - "not-found"
func (api *PublicQuantAPI) GetQuantStatus(ctx context.Context, hash common.Hash) (string, error) {
	return api.eth.quantStatus(hash)
}
