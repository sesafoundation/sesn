package params

import "github.com/sesafoundation/sesn/common"

// Address of the native PREMIUM NFT precompile.
// NOTE: Must NEVER change after mainnet launch.
var PremiumNFTPrecompileAddress = common.HexToAddress("0x2000000000000000000000000000000000000001")

// Owner / minter address for PREMIUM minting.
// TODO: set to treasury / governance before mainnet.
var PremiumNFTOwnerAddress = common.HexToAddress("0x00000000000000000000000000000000000000AB")
