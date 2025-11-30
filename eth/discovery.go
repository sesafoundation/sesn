// Copyright 2019 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package eth

import (
	"github.com/sesafoundation/sesn/core/forkid"
	"github.com/sesafoundation/sesn/p2p/dnsdisc"
	"github.com/sesafoundation/sesn/p2p/enode"
	"github.com/sesafoundation/sesn/rlp"
)

// ethEntry is the "eth" ENR entry that advertises the eth protocol
// on the discovery network. Fields other than ForkID are ignored
// for forward compatibility.
type ethEntry struct {
	ForkID forkid.ID         // Fork identifier per EIP-2124
	Rest   []rlp.RawValue `rlp:"tail"`
}

// ENRKey implements enr.Entry.
func (e ethEntry) ENRKey() string {
	return "eth"
}

// setupDiscovery creates the discovery iterator for the eth protocol.
func (eth *Ethereum) setupDiscovery() (enode.Iterator, error) {
	if len(eth.config.DiscoveryURLs) == 0 {
		return nil, nil
	}
	client := dnsdisc.NewClient(dnsdisc.Config{})
	return client.NewIterator(eth.config.DiscoveryURLs...)
}
