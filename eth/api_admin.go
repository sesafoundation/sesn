// eth/api_admin.go
package eth

import (
	"github.com/sesafoundation/sesn/common"
	"github.com/sesafoundation/sesn/p2p/enode"
)

// PrivateAdminAPI implements RPC methods for node/admin operations.
type PrivateAdminAPI struct {
	eth *Ethereum
}

// NewPrivateAdminAPI creates a new admin API instance.
func NewPrivateAdminAPI(eth *Ethereum) *PrivateAdminAPI {
	return &PrivateAdminAPI{eth: eth}
}

// NodeInfo returns metadata of this node.
func (api *PrivateAdminAPI) NodeInfo() *enode.Node {
	return api.eth.p2pServer.Self()
}

// SetEtherbase sets the mining address.
func (api *PrivateAdminAPI) SetEtherbase(addr common.Address) {
	api.eth.SetEtherbase(addr)
}

// AddPeer requests P2P connection to a node.
func (api *PrivateAdminAPI) AddPeer(url string) (bool, error) {
	return api.eth.p2pServer.AddPeer(url)
}

// RemovePeer disconnects from the specified peer.
func (api *PrivateAdminAPI) RemovePeer(url string) (bool, error) {
	return api.eth.p2pServer.RemovePeer(url)
}

// Peers lists connected peers.
func (api *PrivateAdminAPI) Peers() []*p2p.PeerInfo {
	return api.eth.p2pServer.PeersInfo()
}
