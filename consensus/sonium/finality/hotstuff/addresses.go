package hotstuff

import "github.com/sesafoundation/sesn/common"

var (
	// DPoS validator registry (active set, pubkeys, stakes)
	ValidatorRegistryAddr = common.HexToAddress("0x0000000000000000000000000000000000001000")

	// Slashing / penalties (equivocation, missed votes, etc.)
	SlashContractAddr     = common.HexToAddress("0x0000000000000000000000000000000000001001")
)
