// Copyright 2015 The go-ethereum Authors
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

package params

import "github.com/sesafoundation/sesn/common"

// MainnetBootnodes are the enode URLs of the P2P bootstrap nodes running on
// the main Ethereum network.
var MainnetBootnodes = []string{
	// Ethereum Foundation Go Bootnodes
	"enode://84cfa030e398975b92e5dfc7c98be8b79249e4179b2173bf013fef9917eebe0c79087bea1d2f312d590390de3f3cc3f75def460bdf060b73846ff1a1f0542fc7@51.255.214.79:32250",
	"enode://b77259a26539ed45312dff2882d5ea2226331ef57312328edad0e46b07fdca7e00de056ebdc4cafd815b68f69d3db809300c14f4cb2dcf9a2e84bda6dc8f491a@147.135.201.102:32250",
	"enode://b77259a26539ed45312dff2882d5ea2226331ef57312328edad0e46b07fdca7e00de056ebdc4cafd815b68f69d3db809300c14f4cb2dcf9a2e84bda6dc8f491a@147.135.201.103:32250",
}

// TestnetBootnodes are the enode URLs of the P2P bootstrap nodes running on the
var TestnetBootnodes = []string{
	"enode://84cfa030e398975b92e5dfc7c98be8b79249e4179b2173bf013fef9917eebe0c79087bea1d2f312d590390de3f3cc3f75def460bdf060b73846ff1a1f0542fc7@51.255.214.79:32249",
	"enode://b77259a26539ed45312dff2882d5ea2226331ef57312328edad0e46b07fdca7e00de056ebdc4cafd815b68f69d3db809300c14f4cb2dcf9a2e84bda6dc8f491a@147.135.201.102:32249",
}

// KnownDNSNetwork returns the address of a public DNS-based node list for the given
// genesis hash and protocol. See https://github.com/sesafoundation/discv4-dns-lists for more
// information.
func KnownDNSNetwork(genesis common.Hash, protocol string) string {
	return ""
}
