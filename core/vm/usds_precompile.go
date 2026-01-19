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
// ─────────────────────────────────────────────
// ERC20 FUNCTION SELECTORS
// ─────────────────────────────────────────────
//

var usdsSigTotalSupply = [4]byte{0x18, 0x16, 0x0d, 0xdd} // totalSupply()
var usdsSigBalanceOf = [4]byte{0x70, 0xa0, 0x82, 0x31}   // balanceOf(address)
var usdsSigTransfer = [4]byte{0xa9, 0x05, 0x9c, 0xbb}    // transfer(address,uint256)
var usdsSigApprove = [4]byte{0x09, 0x5e, 0xa7, 0xb3}     // approve(address,uint256)
var usdsSigAllowance = [4]byte{0xdd, 0x62, 0xed, 0x3e}   // allowance(address,address)
var usdsSigTransferFrom = [4]byte{0x23, 0xb8, 0x72, 0xdd} // transferFrom(address,address,uint256)
var usdsSigMint = [4]byte{0x40, 0xc1, 0x0f, 0x19}        // mint(address,uint256)

//
// ─────────────────────────────────────────────
// ERC20 EVENT TOPICS
// ─────────────────────────────────────────────
//

var usdsTopicTransfer = common.HexToHash(
	"0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef",
)

var usdsTopicApproval = common.HexToHash(
	"0x8c5be1e5ebec7d5bd14f714f6f8f1a9d8a89b9a4d8120c8d7e92443c7f5d9e8e",
)

//
// ─────────────────────────────────────────────
// STORAGE LAYOUT
// ─────────────────────────────────────────────
//
// slot 0 -> totalSupply
// slot 1 -> balances mapping
// slot 2 -> allowances mapping
//

var usdsSlotTotalSupply = common.BigToHash(big.NewInt(0))

func usdsBalanceKey(addr common.Address) common.Hash {
	var slot [32]byte
	slot[31] = 1

	var padded [32]byte
	copy(padded[12:], addr.Bytes())

	return crypto.Keccak256Hash(padded[:], slot[:])
}

func usdsAllowanceKey(owner, spender common.Address) common.Hash {
	var slot [32]byte
	slot[31] = 2

	var spenderPad [32]byte
	copy(spenderPad[12:], spender.Bytes())

	inner := crypto.Keccak256Hash(spenderPad[:], slot[:])

	var ownerPad [32]byte
	copy(ownerPad[12:], owner.Bytes())

	return crypto.Keccak256Hash(ownerPad[:], inner[:])
}

//
// ─────────────────────────────────────────────
// PRECOMPILE
// ─────────────────────────────────────────────
//

type USDSPrecompile struct{}

func (c *USDSPrecompile) RequiredGas(input []byte) uint64 {
	// Cheap fixed cost — real fee exemption done in StateTransition
	return 20_000
}

func (c *USDSPrecompile) Run(input []byte, contract *Contract, evm *EVM) ([]byte, error) {

	if len(input) < 4 {
		return nil, errors.New("USDS: missing selector")
	}

	var sel [4]byte
	copy(sel[:], input[:4])

	switch sel {

	case usdsSigTotalSupply:
		return packUint256(c.totalSupply(evm)), nil

	case usdsSigBalanceOf:
		if err := requireLen(input, 4+32); err != nil { return nil, err }
		addr := common.BytesToAddress(input[16:36])
		return packUint256(c.balanceOf(evm, addr)), nil

	case usdsSigAllowance:
		if err := requireLen(input, 4+64); err != nil { return nil, err }
		owner := common.BytesToAddress(input[16:36])
		spender := common.BytesToAddress(input[48:68])
		return packUint256(c.allowance(evm, owner, spender)), nil

	case usdsSigApprove:
		if err := requireLen(input, 4+64); err != nil { return nil, err }
		spender := common.BytesToAddress(input[16:36])
		amount := bytesToBig(input[36:68])
		return packBool(true), c.approve(evm, contract.Caller(), spender, amount)

	case usdsSigTransfer:
		if err := requireLen(input, 4+64); err != nil { return nil, err }
		to := common.BytesToAddress(input[16:36])
		amount := bytesToBig(input[36:68])
		return packBool(true), c.transfer(evm, contract.Caller(), to, amount)

	case usdsSigTransferFrom:
		if err := requireLen(input, 4+96); err != nil { return nil, err }
		from := common.BytesToAddress(input[16:36])
		to := common.BytesToAddress(input[48:68])
		amount := bytesToBig(input[68:100])
		return packBool(true), c.transferFrom(evm, contract.Caller(), from, to, amount)

	case usdsSigMint:
		if contract.Caller() != params.USDSOwnerAddress {
			return nil, errors.New("USDS: only owner")
		}
		if err := requireLen(input, 4+64); err != nil { return nil, err }
		to := common.BytesToAddress(input[16:36])
		amount := bytesToBig(input[36:68])
		return packBool(true), c.mint(evm, to, amount)

	default:
		return nil, errors.New("USDS: unknown selector")
	}
}

//
// ─────────────────────────────────────────────
// STATE LOGIC
// ─────────────────────────────────────────────
//

func (c *USDSPrecompile) totalSupply(evm *EVM) *big.Int {
	h := evm.StateDB.GetState(params.USDSPrecompileAddress, usdsSlotTotalSupply)
	if h == (common.Hash{}) {
		return big.NewInt(0)
	}
	return h.Big()
}

func (c *USDSPrecompile) setTotalSupply(evm *EVM, v *big.Int) {
	evm.StateDB.SetState(params.USDSPrecompileAddress, usdsSlotTotalSupply, common.BigToHash(v))
}

func (c *USDSPrecompile) balanceOf(evm *EVM, a common.Address) *big.Int {
	h := evm.StateDB.GetState(params.USDSPrecompileAddress, usdsBalanceKey(a))
	if h == (common.Hash{}) {
		return big.NewInt(0)
	}
	return h.Big()
}

func (c *USDSPrecompile) setBalance(evm *EVM, a common.Address, v *big.Int) {
	evm.StateDB.SetState(params.USDSPrecompileAddress, usdsBalanceKey(a), common.BigToHash(v))
}

func (c *USDSPrecompile) allowance(evm *EVM, owner, spender common.Address) *big.Int {
	h := evm.StateDB.GetState(params.USDSPrecompileAddress, usdsAllowanceKey(owner, spender))
	if h == (common.Hash{}) {
		return big.NewInt(0)
	}
	return h.Big()
}

func (c *USDSPrecompile) setAllowance(evm *EVM, owner, spender common.Address, v *big.Int) {
	evm.StateDB.SetState(params.USDSPrecompileAddress, usdsAllowanceKey(owner, spender), common.BigToHash(v))
}

//
// ─────────────────────────────────────────────
// ERC20 OPERATIONS
// ─────────────────────────────────────────────
//

func (c *USDSPrecompile) approve(evm *EVM, owner, spender common.Address, amount *big.Int) error {

	if amount.Sign() < 0 {
		return errors.New("USDS: negative allowance")
	}

	c.setAllowance(evm, owner, spender, amount)

	evm.StateDB.AddLog(&types.Log{
		Address: params.USDSPrecompileAddress,
		Topics: []common.Hash{
			usdsTopicApproval,
			topicAddress(owner),
			topicAddress(spender),
		},
		Data: packUint256(amount),
	})

	return nil
}

func (c *USDSPrecompile) transfer(evm *EVM, from, to common.Address, amount *big.Int) error {

	if amount.Sign() <= 0 || from == to {
		return nil
	}

	fromBal := c.balanceOf(evm, from)
	if fromBal.Cmp(amount) < 0 {
		return errors.New("USDS: insufficient balance")
	}

	c.setBalance(evm, from, new(big.Int).Sub(fromBal, amount))
	c.setBalance(evm, to, new(big.Int).Add(c.balanceOf(evm, to), amount))

	evm.StateDB.AddLog(&types.Log{
		Address: params.USDSPrecompileAddress,
		Topics: []common.Hash{
			usdsTopicTransfer,
			topicAddress(from),
			topicAddress(to),
		},
		Data: packUint256(amount),
	})

	return nil
}

func (c *USDSPrecompile) transferFrom(evm *EVM, spender, from, to common.Address, amount *big.Int) error {

	allow := c.allowance(evm, from, spender)
	if allow.Cmp(amount) < 0 {
		return errors.New("USDS: insufficient allowance")
	}

	fromBal := c.balanceOf(evm, from)
	if fromBal.Cmp(amount) < 0 {
		return errors.New("USDS: insufficient balance")
	}

	newAllow := new(big.Int).Sub(allow, amount)
	c.setAllowance(evm, from, spender, newAllow)

	c.setBalance(evm, from, new(big.Int).Sub(fromBal, amount))
	c.setBalance(evm, to, new(big.Int).Add(c.balanceOf(evm, to), amount))

	// Transfer event
	evm.StateDB.AddLog(&types.Log{
		Address: params.USDSPrecompileAddress,
		Topics: []common.Hash{
			usdsTopicTransfer,
			topicAddress(from),
			topicAddress(to),
		},
		Data: packUint256(amount),
	})

	// Approval update event (ERC20 compliance)
	evm.StateDB.AddLog(&types.Log{
		Address: params.USDSPrecompileAddress,
		Topics: []common.Hash{
			usdsTopicApproval,
			topicAddress(from),
			topicAddress(spender),
		},
		Data: packUint256(newAllow),
	})

	return nil
}

func (c *USDSPrecompile) mint(evm *EVM, to common.Address, amount *big.Int) error {

	if amount.Sign() <= 0 {
		return errors.New("USDS: mint amount zero")
	}

	c.setTotalSupply(evm, new(big.Int).Add(c.totalSupply(evm), amount))
	c.setBalance(evm, to, new(big.Int).Add(c.balanceOf(evm, to), amount))

	evm.StateDB.AddLog(&types.Log{
		Address: params.USDSPrecompileAddress,
		Topics: []common.Hash{
			usdsTopicTransfer,
			common.Hash{},
			topicAddress(to),
		},
		Data: packUint256(amount),
	})

	return nil
}

//
// ─────────────────────────────────────────────
// UTILS
// ─────────────────────────────────────────────
//

func requireLen(input []byte, n int) error {
	if len(input) < n {
		return errors.New("USDS: bad calldata length")
	}
	return nil
}

func topicAddress(a common.Address) common.Hash {
	return common.BytesToHash(common.LeftPadBytes(a.Bytes(), 32))
}

func packUint256(v *big.Int) []byte {
	out := make([]byte, 32)
	if v.Sign() > 0 {
		copy(out[32-len(v.Bytes()):], v.Bytes())
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

func bytesToBig(b []byte) *big.Int {
	if len(b) == 0 {
		return big.NewInt(0)
	}
	return new(big.Int).SetBytes(b)
}

