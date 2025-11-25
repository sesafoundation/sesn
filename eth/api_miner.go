// eth/api_miner.go
package eth

import (
	"github.com/sesafoundation/sesn/common"
)

// PrivateMinerAPI exposes mining control methods.
type PrivateMinerAPI struct {
	eth *Ethereum
}

// NewPrivateMinerAPI creates a new miner control API instance.
func NewPrivateMinerAPI(eth *Ethereum) *PrivateMinerAPI {
	return &PrivateMinerAPI{eth: eth}
}

// Start starts mining with the given thread count.
func (api *PrivateMinerAPI) Start(threads int) error {
	return api.eth.StartMining(threads)
}

// Stop stops the miner completely.
func (api *PrivateMinerAPI) Stop() {
	api.eth.StopMining()
}

// SetEtherbase changes the mining reward address.
func (api *PrivateMinerAPI) SetEtherbase(addr common.Address) {
	api.eth.SetEtherbase(addr)
}

// SetGasPrice configures the minimum gas price for tx inclusion.
func (api *PrivateMinerAPI) SetGasPrice(price uint64) {
	api.eth.lock.Lock()
	api.eth.gasPrice = new(big.Int).SetUint64(price)
	api.eth.lock.Unlock()
	api.eth.txPool.SetGasPrice(api.eth.gasPrice)
}
