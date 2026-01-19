package params

import "github.com/sesafoundation/sesn/common"

// Address of the native USDS precompile.
//
// NOTE: This must NEVER change after mainnet launch.
var USDSPrecompileAddress = common.HexToAddress("0x1000000000000000000000000000000000000001")

// Hardcoded USDS owner / minter address.
//
// ⚠️ TODO: CHANGE THIS to your treasury / owner address
// before launching the network.
var USDSOwnerAddress = common.HexToAddress("0x00000000000000000000000000000000000000AB")