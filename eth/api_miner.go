package eth

import (
	"math/big"
	"github.com/sesafoundation/sesn/common"
	"github.com/sesafoundation/sesn/internal/ethapi"
)

// PrivateMinerAPI exposes mining control over RPC.
type PrivateMinerAPI struct {
	eth *Ethereum
}

type PublicMinerAPI struct {
    backend ethapi.Backend
}

func NewPrivateMinerAPI(eth *Ethereum) *PrivateMinerAPI {
	return &PrivateMinerAPI{eth: eth}
}

// Start begins mining with the specified number of threads.
func (api *PrivateMinerAPI) Start(threads int) error {
	return api.eth.StartMining(threads)
}

// Stop terminates mining.
func (api *PrivateMinerAPI) Stop() {
	api.eth.StopMining()
}

// SetEtherbase changes the mining reward address.
func (api *PrivateMinerAPI) SetEtherbase(addr common.Address) {
	api.eth.SetEtherbase(addr)
}

// SetGasPrice updates the minimum gas price accepted by the txpool.
func (api *PrivateMinerAPI) SetGasPrice(price uint64) {
	api.eth.lock.Lock()
	api.eth.gasPrice = new(big.Int).SetUint64(price)
	api.eth.lock.Unlock()

	api.eth.txPool.SetGasPrice(api.eth.gasPrice)
}

///new 
func NewPublicMinerAPI(backend ethapi.Backend) *PublicMinerAPI {
    return &PublicMinerAPI{backend: backend}
}

func (api *PublicMinerAPI) Mining() bool {
    return api.backend.Miner().Mining()
}
