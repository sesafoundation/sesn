package params

import "github.com/sesafoundation/sesn/common"

// Address of the native SuperNFT precompile.
//
// NOTE: This must NEVER change after mainnet launch.
var SuperNFTPrecompileAddress = common.HexToAddress("0x2000000000000000000000000000000000000000")

// Hardcoded SuperNFT owner / minter address.
//
// ⚠️ TODO: CHANGE THIS to your treasury / owner address
// before launching the network.
var SuperNFTOwnerAddress = common.HexToAddress("0x00000000000000000000000000000000000000AD")
