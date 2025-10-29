package hotstuff

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/sesafoundation/sesn/common"
	"github.com/sesafoundation/sesn/core/types"
	"github.com/sesafoundation/sesn/ethclient"
)

//go:generate abigen --abi ./abi/ValidatorRegistry.json --pkg hotstuff --type ValidatorRegistry --out ./validator_registry_bindings.go
// Expected methods in the ABI:
//   function activeValidators() view returns (address[] addrs, bytes[] blsPubkeys)
//   function selfIndex(address who) view returns (uint256, bool)

// ContractVSet pulls and caches the active validator set from the registry contract.
type ContractVSet struct {
	rpc    *ethclient.Client
	cache  atomic.Value // stores *cacheEntry
}

type cacheEntry struct {
	addrs []common.Address
	pubs  [][]byte
	at    uint64 // block number
}

func NewContractVSet(rpc *ethclient.Client) *ContractVSet {
	vs := &ContractVSet{rpc: rpc}
	vs.cache.Store(&cacheEntry{nil, nil, 0})
	return vs
}

func (vs *ContractVSet) Active() ([]common.Address, [][]byte) {
	ce := vs.cache.Load().(*cacheEntry)
	return ce.addrs, ce.pubs
}

func (vs *ContractVSet) IndexOf(a common.Address) (int, bool) {
	ce := vs.cache.Load().(*cacheEntry)
	for i, x := range ce.addrs {
		if x == a {
			return i, true
		}
	}
	return -1, false
}

// This node’s signer (coinbase).
func (vs *ContractVSet) SelfCoinbase() common.Address {
	// Replace with your existing identity accessor if different.
	// Typically miner/engine coinbase or configured validator address.
	coinbase, _ := vs.rpc.Coinbase(context.Background())
	return coinbase
}

// RefreshAt re-reads the active set at/after a given head (reorg-safe).
func (vs *ContractVSet) RefreshAt(head *types.Header) error {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	reg, err := NewValidatorRegistry(ValidatorRegistryAddr, vs.rpc)
	if err != nil {
		return err
	}
	callOpts := &bind.CallOpts{Context: ctx, BlockNumber: new(big.Int).SetUint64(head.Number.Uint64())}

	addrs, pubs, err := reg.ActiveValidators(callOpts)
	if err != nil {
		return err
	}
	vs.cache.Store(&cacheEntry{addrs: addrs, pubs: pubs, at: head.Number.Uint64()})
	return nil
}
