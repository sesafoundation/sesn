package eth

import (
	"github.com/sesafoundation/sesn/common"
	"github.com/sesafoundation/sesn/p2p/enode"
)

// PrivateAdminAPI exposes node admin operations over RPC.
type PrivateAdminAPI struct {
	eth *Ethereum
}

func NewPrivateAdminAPI(eth *Ethereum) *PrivateAdminAPI {
	return &PrivateAdminAPI{eth: eth}
}

// SetEtherbase sets the mining reward address.
func (api *PrivateAdminAPI) SetEtherbase(addr common.Address) {
	api.eth.SetEtherbase(addr)
}

// AddPeer connects to an enode:// URL.


// Peers returns information about connected peers.
func (api *PrivateAdminAPI) Peers() interface{} {
	return api.eth.p2pServer.PeersInfo()
}


func (api *PrivateAdminAPI) AddPeer(url string) error {
    n, err := enode.Parse(enode.ValidSchemes, url)
    if err != nil {
        return err
    }
    api.eth.p2pServer.AddPeer(n) // no return value
    return nil
}

func (api *PrivateAdminAPI) RemovePeer(url string) error {
    n, err := enode.Parse(enode.ValidSchemes, url)
    if err != nil {
        return err
    }
    api.eth.p2pServer.RemovePeer(n) // no return value
    return nil
}