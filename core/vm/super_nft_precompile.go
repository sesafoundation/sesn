package vm

import (
	"errors"
	"math/big"

	"github.com/sesafoundation/sesn/common"
	"github.com/sesafoundation/sesn/core/types"
	"github.com/sesafoundation/sesn/crypto"
	"github.com/sesafoundation/sesn/params"
)

//
// ──────────────────────────────────────────────────────────────
// FUNCTION SELECTORS (keccak256(sig)[0:4])
// ──────────────────────────────────────────────────────────────
var (
	// ERC165
	superSigSupportsInterface = [4]byte{0x01, 0xff, 0xc9, 0xa7} // supportsInterface(bytes4)

	// ERC721 metadata
	superSigName   = [4]byte{0x06, 0xfd, 0xde, 0x03} // name()
	superSigSymbol = [4]byte{0x95, 0xd8, 0x9b, 0x41} // symbol()

	// ERC721 core
	superSigBalanceOf = [4]byte{0x70, 0xa0, 0x82, 0x31} // balanceOf(address)
	superSigOwnerOf   = [4]byte{0x63, 0x52, 0x21, 0x1e} // ownerOf(uint256)

	// Mint (owner-only)
	superSigMint = [4]byte{0x40, 0xc1, 0x0f, 0x19} // mint(address,uint256)
)

//
// ──────────────────────────────────────────────────────────────
// EVENT TOPICS
// ──────────────────────────────────────────────────────────────
var (
	// keccak256("Transfer(address,address,uint256)")
	superTopicTransfer = common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")
)

//
// ──────────────────────────────────────────────────────────────
// STORAGE LAYOUT (all inside params.SuperNFTPrecompileAddress)
// ──────────────────────────────────────────────────────────────
//
// slot 0: unused
// slot 1: owners[tokenId]   mapping(uint256 => address)
// slot 2: balances[owner]   mapping(address => uint256)
//

func superOwnerKey(tokenId *big.Int) common.Hash {
	// keccak256(pad(tokenId) . pad(slot=1))
	var slot [32]byte
	slot[31] = 1
	return crypto.Keccak256Hash(padBigTo32(tokenId), slot[:])
}

func superBalanceKey(owner common.Address) common.Hash {
	// keccak256(pad(owner) . pad(slot=2))
	var slot [32]byte
	slot[31] = 2
	var padded [32]byte
	copy(padded[12:], owner[:])
	return crypto.Keccak256Hash(padded[:], slot[:])
}

type SuperNFTPrecompile struct{}

func (c *SuperNFTPrecompile) RequiredGas(input []byte) uint64 {
	// Flat cheap rate. Gas-free privilege is enforced in state_transition, not here.
	return 20_000
}

func (c *SuperNFTPrecompile) Run(input []byte, contract *Contract, evm *EVM) ([]byte, error) {
	if len(input) < 4 {
		return nil, errors.New("SUPERNFT: missing selector")
	}
	var sel [4]byte
	copy(sel[:], input[:4])

	switch sel {

	case superSigSupportsInterface:
		// supportsInterface(bytes4)
		if len(input) < 4+32 {
			return nil, errors.New("SUPERNFT: bad calldata")
		}
		// Support ERC165, ERC721, ERC721Metadata minimal
		id := input[4+28 : 4+32] // last 4 bytes
		erc165 := []byte{0x01, 0xff, 0xc9, 0xa7}
		erc721 := []byte{0x80, 0xac, 0x58, 0xcd}
		erc721Meta := []byte{0x5b, 0x5e, 0x13, 0x9f}
		ok := equal4(id, erc165) || equal4(id, erc721) || equal4(id, erc721Meta)
		return packBool(ok), nil

	case superSigName:
		return packString("SUPERNFT"), nil

	case superSigSymbol:
		return packString("SUPERNFT"), nil

	case superSigBalanceOf:
		if len(input) < 4+32 {
			return nil, errors.New("SUPERNFT: bad calldata")
		}
		owner := common.BytesToAddress(input[4+12 : 4+32])
		return packUint256(c.balanceOf(evm, owner)), nil

	case superSigOwnerOf:
		if len(input) < 4+32 {
			return nil, errors.New("SUPERNFT: bad calldata")
		}
		tokenId := bytesToBig(input[4 : 4+32])
		owner, ok := c.ownerOf(evm, tokenId)
		if !ok {
			return nil, errors.New("SUPERNFT: non-existent token")
		}
		return packAddress(owner), nil

	case superSigMint:
		// mint(address,uint256) owner-only
		if contract.Caller() != params.SuperNFTOwnerAddress {
			return nil, errors.New("SUPERNFT: only owner can mint")
		}
		if len(input) < 4+64 {
			return nil, errors.New("SUPERNFT: bad calldata")
		}
		to := common.BytesToAddress(input[4+12 : 4+32])
		tokenId := bytesToBig(input[4+32 : 4+64])
		return packBool(true), c.mint(evm, to, tokenId)

	default:
		return nil, errors.New("SUPERNFT: unknown selector")
	}
}

// ──────────────────────────────────────────────────────────────
// Storage ops
// ──────────────────────────────────────────────────────────────

func (c *SuperNFTPrecompile) balanceOf(evm *EVM, owner common.Address) *big.Int {
	key := superBalanceKey(owner)
	h := evm.StateDB.GetState(params.SuperNFTPrecompileAddress, key)
	if h == (common.Hash{}) {
		return big.NewInt(0)
	}
	return h.Big()
}

func (c *SuperNFTPrecompile) setBalance(evm *EVM, owner common.Address, v *big.Int) {
	evm.StateDB.SetState(params.SuperNFTPrecompileAddress, superBalanceKey(owner), common.BigToHash(v))
}

func (c *SuperNFTPrecompile) ownerOf(evm *EVM, tokenId *big.Int) (common.Address, bool) {
	h := evm.StateDB.GetState(params.SuperNFTPrecompileAddress, superOwnerKey(tokenId))
	if h == (common.Hash{}) {
		return common.Address{}, false
	}
	owner := common.BytesToAddress(h.Bytes()[12:])
	if owner == (common.Address{}) {
		return common.Address{}, false
	}
	return owner, true
}

func (c *SuperNFTPrecompile) setOwner(evm *EVM, tokenId *big.Int, owner common.Address) {
	var b [32]byte
	copy(b[12:], owner[:])
	evm.StateDB.SetState(params.SuperNFTPrecompileAddress, superOwnerKey(tokenId), common.BytesToHash(b[:]))
}

// ──────────────────────────────────────────────────────────────
// Actions
// ──────────────────────────────────────────────────────────────

func (c *SuperNFTPrecompile) mint(evm *EVM, to common.Address, tokenId *big.Int) error {
	if to == (common.Address{}) {
		return errors.New("SUPERNFT: mint to zero address")
	}
	_, exists := c.ownerOf(evm, tokenId)
	if exists {
		return errors.New("SUPERNFT: token already minted")
	}

	// Soulbound policy (recommended): 1 address = 1 tokenId (optional)
	// If you want to enforce strictly one SuperNFT per address, uncomment:
	// if c.balanceOf(evm, to).Sign() != 0 {
	//     return errors.New("SUPERNFT: already holds SuperNFT")
	// }

	c.setOwner(evm, tokenId, to)

	bal := c.balanceOf(evm, to)
	c.setBalance(evm, to, new(big.Int).Add(bal, big.NewInt(1)))

	// Transfer(0,to,tokenId)
	evm.StateDB.AddLog(&types.Log{
		Address: params.SuperNFTPrecompileAddress,
		Topics: []common.Hash{
			superTopicTransfer,
			common.Hash{}, // from = 0x0
			common.BytesToHash(to.Bytes()),
			common.BigToHash(tokenId),
		},
		Data: nil,
	})
	return nil
}

