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
	premSigSupportsInterface = [4]byte{0x01, 0xff, 0xc9, 0xa7} // supportsInterface(bytes4)

	// ERC721 metadata
	premSigName   = [4]byte{0x06, 0xfd, 0xde, 0x03} // name()
	premSigSymbol = [4]byte{0x95, 0xd8, 0x9b, 0x41} // symbol()

	// ERC721 core
	premSigBalanceOf  = [4]byte{0x70, 0xa0, 0x82, 0x31} // balanceOf(address)
	premSigOwnerOf    = [4]byte{0x63, 0x52, 0x21, 0x1e} // ownerOf(uint256)
	premSigApprove    = [4]byte{0x09, 0x5e, 0xa7, 0xb3} // approve(address,uint256)
	premSigGetApproved = [4]byte{0x08, 0x18, 0x12, 0xfc} // getApproved(uint256)
	premSigSetApprovalForAll = [4]byte{0xa2, 0x2c, 0xb4, 0x65} // setApprovalForAll(address,bool)
	premSigIsApprovedForAll  = [4]byte{0xe9, 0x85, 0xe9, 0xc5} // isApprovedForAll(address,address)
	premSigTransferFrom      = [4]byte{0x23, 0xb8, 0x72, 0xdd} // transferFrom(address,address,uint256)

	// Mint (owner-only)
	premSigMint = [4]byte{0x40, 0xc1, 0x0f, 0x19} // mint(address,uint256)
)

//
// ──────────────────────────────────────────────────────────────
// EVENT TOPICS
// ──────────────────────────────────────────────────────────────
var (
	// keccak256("Transfer(address,address,uint256)")
	premTopicTransfer = common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")
	// keccak256("Approval(address,address,uint256)")
	premTopicApproval = common.HexToHash("0x8c5be1e5ebec7d5bd14f714f6f8f1a9d8a89b9a4d8120c8d7e92443c7f5d9e8e")
	// keccak256("ApprovalForAll(address,address,bool)")
	premTopicApprovalForAll = common.HexToHash("0x17307eab39ab6107e8899845ad3d59bd9653f200f220920489ca2b5937696c31")
)

//
// ──────────────────────────────────────────────────────────────
// STORAGE LAYOUT (all inside params.PremiumNFTPrecompileAddress)
// ──────────────────────────────────────────────────────────────
//
// slot 0: unused (can be used later)
// slot 1: owners[tokenId]             mapping(uint256 => address)
// slot 2: balances[owner]             mapping(address => uint256)
// slot 3: tokenApprovals[tokenId]     mapping(uint256 => address)
// slot 4: operatorApprovals[owner][op] mapping(address => mapping(address => bool))
//

func premOwnerKey(tokenId *big.Int) common.Hash {
	// keccak256(pad(tokenId) . pad(slot=1))
	var slot [32]byte
	slot[31] = 1
	return crypto.Keccak256Hash(padBigTo32(tokenId), slot[:])
}

func premBalanceKey(owner common.Address) common.Hash {
	// keccak256(pad(owner) . pad(slot=2))
	var slot [32]byte
	slot[31] = 2
	var padded [32]byte
	copy(padded[12:], owner[:])
	return crypto.Keccak256Hash(padded[:], slot[:])
}

func premApprovalKey(tokenId *big.Int) common.Hash {
	// keccak256(pad(tokenId) . pad(slot=3))
	var slot [32]byte
	slot[31] = 3
	return crypto.Keccak256Hash(padBigTo32(tokenId), slot[:])
}

func premOperatorApprovalKey(owner, operator common.Address) common.Hash {
	// inner = keccak256(pad(operator) . pad(slot=4))
	// key = keccak256(pad(owner) . inner)
	var slot [32]byte
	slot[31] = 4

	var opPadded [32]byte
	copy(opPadded[12:], operator[:])
	inner := crypto.Keccak256Hash(opPadded[:], slot[:])

	var ownerPadded [32]byte
	copy(ownerPadded[12:], owner[:])
	return crypto.Keccak256Hash(ownerPadded[:], inner[:])
}

type PremiumNFTPrecompile struct{}

func (c *PremiumNFTPrecompile) RequiredGas(input []byte) uint64 {
	// Cheap flat rate. Actual fee logic for USDS is enforced in state_transition.
	return 20_000
}

func (c *PremiumNFTPrecompile) Run(input []byte, contract *Contract, evm *EVM) ([]byte, error) {
	if len(input) < 4 {
		return nil, errors.New("PREMIUM: missing selector")
	}
	var sel [4]byte
	copy(sel[:], input[:4])

	switch sel {

	case premSigSupportsInterface:
		// supportsInterface(bytes4)
		if len(input) < 4+32 {
			return nil, errors.New("PREMIUM: bad calldata")
		}
		// We support ERC165, ERC721, ERC721Metadata (minimal)
		id := input[4+28 : 4+32] // last 4 bytes of the 32-byte word
		erc165 := []byte{0x01, 0xff, 0xc9, 0xa7}
		erc721 := []byte{0x80, 0xac, 0x58, 0xcd}
		erc721Meta := []byte{0x5b, 0x5e, 0x13, 0x9f}
		ok := equal4(id, erc165) || equal4(id, erc721) || equal4(id, erc721Meta)
		return packBool(ok), nil

	case premSigName:
		return packString("PREMIUM"), nil

	case premSigSymbol:
		return packString("PREMIUM"), nil

	case premSigBalanceOf:
		owner := common.BytesToAddress(input[4+12 : 4+32])
		return packUint256(c.balanceOf(evm, owner)), nil

	case premSigOwnerOf:
		tokenId := bytesToBig(input[4 : 4+32])
		owner, ok := c.ownerOf(evm, tokenId)
		if !ok {
			return nil, errors.New("PREMIUM: non-existent token")
		}
		return packAddress(owner), nil

	case premSigGetApproved:
		tokenId := bytesToBig(input[4 : 4+32])
		ap := c.getApproved(evm, tokenId)
		return packAddress(ap), nil

	case premSigIsApprovedForAll:
		owner := common.BytesToAddress(input[4+12 : 4+32])
		op := common.BytesToAddress(input[4+32+12 : 4+64])
		ok := c.isApprovedForAll(evm, owner, op)
		return packBool(ok), nil

	case premSigApprove:
		to := common.BytesToAddress(input[4+12 : 4+32])
		tokenId := bytesToBig(input[4+32 : 4+64])
		caller := contract.Caller()
		return packBool(true), c.approve(evm, caller, to, tokenId)

	case premSigSetApprovalForAll:
		op := common.BytesToAddress(input[4+12 : 4+32])
		val := bytesToBig(input[4+32 : 4+64]).Sign() != 0
		caller := contract.Caller()
		return packBool(true), c.setApprovalForAll(evm, caller, op, val)

	case premSigTransferFrom:
		from := common.BytesToAddress(input[4+12 : 4+32])
		to := common.BytesToAddress(input[4+32+12 : 4+64])
		tokenId := bytesToBig(input[4+64 : 4+96])
		caller := contract.Caller()
		return packBool(true), c.transferFrom(evm, caller, from, to, tokenId)

	case premSigMint:
		// mint(address,uint256) owner-only
		if contract.Caller() != params.PremiumNFTOwnerAddress {
			return nil, errors.New("PREMIUM: only owner can mint")
		}
		to := common.BytesToAddress(input[4+12 : 4+32])
		tokenId := bytesToBig(input[4+32 : 4+64])
		return packBool(true), c.mint(evm, to, tokenId)

	default:
		return nil, errors.New("PREMIUM: unknown selector")
	}
}

// ──────────────────────────────────────────────────────────────
// ERC721 storage ops
// ──────────────────────────────────────────────────────────────

func (c *PremiumNFTPrecompile) balanceOf(evm *EVM, owner common.Address) *big.Int {
	key := premBalanceKey(owner)
	h := evm.StateDB.GetState(params.PremiumNFTPrecompileAddress, key)
	if h == (common.Hash{}) {
		return big.NewInt(0)
	}
	return h.Big()
}

func (c *PremiumNFTPrecompile) setBalance(evm *EVM, owner common.Address, v *big.Int) {
	evm.StateDB.SetState(params.PremiumNFTPrecompileAddress, premBalanceKey(owner), common.BigToHash(v))
}

func (c *PremiumNFTPrecompile) ownerOf(evm *EVM, tokenId *big.Int) (common.Address, bool) {
	h := evm.StateDB.GetState(params.PremiumNFTPrecompileAddress, premOwnerKey(tokenId))
	if h == (common.Hash{}) {
		return common.Address{}, false
	}
	owner := common.BytesToAddress(h.Bytes()[12:])
	if owner == (common.Address{}) {
		return common.Address{}, false
	}
	return owner, true
}

func (c *PremiumNFTPrecompile) setOwner(evm *EVM, tokenId *big.Int, owner common.Address) {
	// store as address in low 20 bytes
	var b [32]byte
	copy(b[12:], owner[:])
	evm.StateDB.SetState(params.PremiumNFTPrecompileAddress, premOwnerKey(tokenId), common.BytesToHash(b[:]))
}

func (c *PremiumNFTPrecompile) getApproved(evm *EVM, tokenId *big.Int) common.Address {
	h := evm.StateDB.GetState(params.PremiumNFTPrecompileAddress, premApprovalKey(tokenId))
	if h == (common.Hash{}) {
		return common.Address{}
	}
	return common.BytesToAddress(h.Bytes()[12:])
}

func (c *PremiumNFTPrecompile) setApproved(evm *EVM, tokenId *big.Int, spender common.Address) {
	var b [32]byte
	copy(b[12:], spender[:])
	evm.StateDB.SetState(params.PremiumNFTPrecompileAddress, premApprovalKey(tokenId), common.BytesToHash(b[:]))
}

func (c *PremiumNFTPrecompile) isApprovedForAll(evm *EVM, owner, op common.Address) bool {
	h := evm.StateDB.GetState(params.PremiumNFTPrecompileAddress, premOperatorApprovalKey(owner, op))
	return h.Big().Sign() != 0
}

func (c *PremiumNFTPrecompile) setOperatorApproved(evm *EVM, owner, op common.Address, val bool) {
	v := big.NewInt(0)
	if val {
		v = big.NewInt(1)
	}
	evm.StateDB.SetState(params.PremiumNFTPrecompileAddress, premOperatorApprovalKey(owner, op), common.BigToHash(v))
}

// ──────────────────────────────────────────────────────────────
// ERC721 actions
// ──────────────────────────────────────────────────────────────

func (c *PremiumNFTPrecompile) approve(evm *EVM, caller, to common.Address, tokenId *big.Int) error {
	owner, ok := c.ownerOf(evm, tokenId)
	if !ok {
		return errors.New("PREMIUM: non-existent token")
	}
	if to == owner {
		return errors.New("PREMIUM: approve to current owner")
	}
	if caller != owner && !c.isApprovedForAll(evm, owner, caller) {
		return errors.New("PREMIUM: not owner nor approved for all")
	}
	c.setApproved(evm, tokenId, to)

	// Approval(owner, to, tokenId)
	evm.StateDB.AddLog(&types.Log{
		Address: params.PremiumNFTPrecompileAddress,
		Topics: []common.Hash{
			premTopicApproval,
			common.BytesToHash(owner.Bytes()),
			common.BytesToHash(to.Bytes()),
			common.BigToHash(tokenId),
		},
		Data: nil,
	})
	return nil
}

func (c *PremiumNFTPrecompile) setApprovalForAll(evm *EVM, caller, op common.Address, val bool) error {
	if op == caller {
		return errors.New("PREMIUM: approve self")
	}
	c.setOperatorApproved(evm, caller, op, val)

	// ApprovalForAll(owner, operator, approved)
	approvedWord := make([]byte, 32)
	if val {
		approvedWord[31] = 1
	}
	evm.StateDB.AddLog(&types.Log{
		Address: params.PremiumNFTPrecompileAddress,
		Topics: []common.Hash{
			premTopicApprovalForAll,
			common.BytesToHash(caller.Bytes()),
			common.BytesToHash(op.Bytes()),
		},
		Data: approvedWord,
	})
	return nil
}

func (c *PremiumNFTPrecompile) transferFrom(evm *EVM, caller, from, to common.Address, tokenId *big.Int) error {
	if to == (common.Address{}) {
		return errors.New("PREMIUM: transfer to zero address")
	}
	owner, ok := c.ownerOf(evm, tokenId)
	if !ok {
		return errors.New("PREMIUM: non-existent token")
	}
	if owner != from {
		return errors.New("PREMIUM: from is not owner")
	}

	approved := c.getApproved(evm, tokenId)
	if caller != owner && caller != approved && !c.isApprovedForAll(evm, owner, caller) {
		return errors.New("PREMIUM: not approved")
	}

	// clear token approval
	c.setApproved(evm, tokenId, common.Address{})

	// balances
	fromBal := c.balanceOf(evm, from)
	toBal := c.balanceOf(evm, to)

	c.setBalance(evm, from, new(big.Int).Sub(fromBal, big.NewInt(1)))
	c.setBalance(evm, to, new(big.Int).Add(toBal, big.NewInt(1)))

	// owner
	c.setOwner(evm, tokenId, to)

	// Transfer(from,to,tokenId)
	evm.StateDB.AddLog(&types.Log{
		Address: params.PremiumNFTPrecompileAddress,
		Topics: []common.Hash{
			premTopicTransfer,
			common.BytesToHash(from.Bytes()),
			common.BytesToHash(to.Bytes()),
			common.BigToHash(tokenId),
		},
		Data: nil,
	})
	return nil
}

func (c *PremiumNFTPrecompile) mint(evm *EVM, to common.Address, tokenId *big.Int) error {
	if to == (common.Address{}) {
		return errors.New("PREMIUM: mint to zero address")
	}
	_, exists := c.ownerOf(evm, tokenId)
	if exists {
		return errors.New("PREMIUM: token already minted")
	}

	// set owner
	c.setOwner(evm, tokenId, to)

	// inc balance
	bal := c.balanceOf(evm, to)
	c.setBalance(evm, to, new(big.Int).Add(bal, big.NewInt(1)))

	// Transfer(0,to,tokenId)
	evm.StateDB.AddLog(&types.Log{
		Address: params.PremiumNFTPrecompileAddress,
		Topics: []common.Hash{
			premTopicTransfer,
			common.Hash{}, // from = 0x0
			common.BytesToHash(to.Bytes()),
			common.BigToHash(tokenId),
		},
		Data: nil,
	})
	return nil
}

// ──────────────────────────────────────────────────────────────
// utils
// ──────────────────────────────────────────────────────────────

func padBigTo32(v *big.Int) []byte {
	out := make([]byte, 32)
	if v != nil && v.Sign() > 0 {
		b := v.Bytes()
		copy(out[32-len(b):], b)
	}
	return out
}

func bytesToBig(b []byte) *big.Int {
	if len(b) == 0 {
		return big.NewInt(0)
	}
	return new(big.Int).SetBytes(b)
}

func packUint256(v *big.Int) []byte {
	out := make([]byte, 32)
	if v != nil && v.Sign() > 0 {
		b := v.Bytes()
		copy(out[32-len(b):], b)
	}
	return out
}

func packBool(b bool) []byte {
	out := make([]byte, 32)
	if b {
		out[31] = 1
	}
	return out
}

func packAddress(a common.Address) []byte {
	out := make([]byte, 32)
	copy(out[12:], a[:])
	return out
}

// minimal ABI encoding for string (dynamic):
// return: offset(32) + len + data(padded)
func packString(s string) []byte {
	b := []byte(s)
	// head: offset=32
	head := make([]byte, 32)
	head[31] = 32
	// len
	lw := make([]byte, 32)
	lwBig := big.NewInt(int64(len(b)))
	copy(lw[32-len(lwBig.Bytes()):], lwBig.Bytes())
	// data padded
	pad := ((len(b) + 31) / 32) * 32
	data := make([]byte, pad)
	copy(data, b)
	return append(append(head, lw...), data...)
}

func equal4(x []byte, y []byte) bool {
	if len(x) != 4 || len(y) != 4 {
		return false
	}
	for i := 0; i < 4; i++ {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}
