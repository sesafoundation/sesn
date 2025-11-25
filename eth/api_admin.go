package eth

import (
	"github.com/sesafoundation/sesn/common"
)

// PrivateAdminAPI exposes node admin operations.
type PrivateAdminAPI struct {
	eth *Ethereum
}

func NewPrivateAdminAPI(eth *Ethereum) *PrivateAdminAPI {
	return &PrivateAdminAPI{eth: eth}
}

// SetEtherbase changes the mining reward address.
func (api *PrivateAdminAPI) SetEtherbase(addr common.Address) {
	api.eth.SetEtherbase(addr)
}

// AddPeer connects to the node URL (enode://...).
func (api *PrivateAdminAPI) AddPeer(url string) error {
	return api.eth.p2pServer.AddPeer(url)
}

// RemovePeer disconnects from the given peer URL.
func (api *PrivateAdminAPI) RemovePeer(url string) error {
	return api.eth.p2pServer.RemovePeer(url)
}

// Peers returns list of connected peers.
func (api *PrivateAdminAPI) Peers() interface{} {
	return api.eth.p2pServer.PeersInfo()
}
