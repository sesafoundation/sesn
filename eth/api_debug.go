// eth/api_debug.go
package eth

import (
	"context"
	"fmt"

	"github.com/sesafoundation/sesn/common/hexutil"
	"github.com/sesafoundation/sesn/internal/ethapi"
	"github.com/sesafoundation/sesn/rlp"
	"github.com/sesafoundation/sesn/rpc"
)

// PublicDebugAPI is the collection of debug APIs exposed on the public endpoint.
type PublicDebugAPI struct {
	backend ethapi.Backend
}

// PrivateDebugAPI is the collection of debug APIs exposed on the private endpoint.
type PrivateDebugAPI struct {
	backend ethapi.Backend
}

// NewPublicDebugAPI creates a new instance of PublicDebugAPI using the shared ethapi.Backend.
func NewPublicDebugAPI(backend ethapi.Backend) *PublicDebugAPI {
	return &PublicDebugAPI{backend: backend}
}

// NewPrivateDebugAPI creates a new instance of PrivateDebugAPI using the shared ethapi.Backend.
func NewPrivateDebugAPI(backend ethapi.Backend) *PrivateDebugAPI {
	return &PrivateDebugAPI{backend: backend}
}

// GetBlockRlp returns the RLP-encoded form of the given block number.
func (api *PublicDebugAPI) GetBlockRlp(ctx context.Context, number uint64) (string, error) {
	block, err := api.backend.BlockByNumber(ctx, rpc.BlockNumber(number))
	if err != nil {
		return "", err
	}
	if block == nil {
		return "", fmt.Errorf("block #%d not found", number)
	}
	data, err := rlp.EncodeToBytes(block)
	if err != nil {
		return "", err
	}
	return hexutil.Encode(data), nil
}

// SetHead moves the canonical chain head to the given block number.
func (api *PrivateDebugAPI) SetHead(number hexutil.Uint64) {
	api.backend.SetHead(uint64(number))
}
