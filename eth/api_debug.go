package eth

import (
	"context"
	"encoding/hex"

	"github.com/sesafoundation/sesn/common/hexutil"
	//"github.com/sesafoundation/sesn/core/types"
	"github.com/sesafoundation/sesn/rlp"
	"github.com/sesafoundation/sesn/internal/ethapi"

)

// ----------------------------------------------------------------------
// PublicDebugAPI – exposed as "debug" namespace, read-only helpers
// ----------------------------------------------------------------------

type PublicDebugAPI struct {
	eth *Ethereum
}

func NewPublicDebugAPI(eth *Ethereum) *PublicDebugAPI {
	return &PublicDebugAPI{eth: eth}
}

type PrivateDebugAPI struct {
    backend ethapi.Backend
}

func NewPrivateDebugAPI(backend ethapi.Backend) *PrivateDebugAPI {
    return &PrivateDebugAPI{backend: backend}
}

// GetBlockRlp returns the RLP encoding of the given block number.
func (api *PublicDebugAPI) GetBlockRlp(_ context.Context, number uint64) (string, error) {
	//block := api.eth.blockchain.GetBlockByNumber(number)
	block := api.backend.BlockChain().GetBlockByNumber(number)

	if block == nil {
		return "", nil
	}
	data, err := rlp.EncodeToBytes(block)
	if err != nil {
		return "", err
	}
	return "0x" + hex.EncodeToString(data), nil
}

// DumpBlock returns a minimal dump of the block (header + tx list).
func (api *PublicDebugAPI) DumpBlock(_ context.Context, number uint64) (map[string]interface{}, error) {
	//block := api.eth.blockchain.GetBlockByNumber(number)
	block := api.backend.BlockChain().GetBlockByNumber(number)

	if block == nil {
		return nil, nil
	}
	out := map[string]interface{}{
		"number":     block.NumberU64(),
		"hash":       block.Hash(),
		"parentHash": block.ParentHash(),
		"txs":        block.Transactions(), // []types.Transaction
	}
	return out, nil
}



// SetHead rewinds/forwards the canonical chain head to the given block number.

func (api *PrivateDebugAPI) SetHead(number hexutil.Uint64) {
    api.backend.SetHead(uint64(number))
}
// NOTE: All trace* methods (TraceCall, TraceTransaction, etc.) are implemented
// in api_tracer.go as methods on *PrivateDebugAPI. This file only defines the
// struct and SetHead. As long as api_tracer.go also uses `type PrivateDebugAPI`
// with field `eth *Ethereum`, everything will compile.
