// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.

package core

import (
	"fmt"
	"math"
	"math/big"

	"github.com/sesafoundation/sesn/common"
	"github.com/sesafoundation/sesn/consensus"
	"github.com/sesafoundation/sesn/core/vm"
	"github.com/sesafoundation/sesn/params"
)

/*
State Transition Model
*/

type StateTransition struct {
	gp         *GasPool
	msg        Message
	gas        uint64
	gasPrice   *big.Int
	initialGas uint64
	value      *big.Int
	data       []byte
	state      vm.StateDB
	evm        *vm.EVM
}

// Message interface

type Message interface {
	From() common.Address
	To() *common.Address

	GasPrice() *big.Int
	Gas() uint64
	Value() *big.Int

	Nonce() uint64
	CheckNonce() bool
	Data() []byte
}

type ExecutionResult struct {
	UsedGas    uint64
	Err        error
	ReturnData []byte
}

func (result *ExecutionResult) Unwrap() error {
	return result.Err
}

func (result *ExecutionResult) Failed() bool { return result.Err != nil }

func (result *ExecutionResult) Return() []byte {
	if result.Err != nil {
		return nil
	}
	return common.CopyBytes(result.ReturnData)
}

func (result *ExecutionResult) Revert() []byte {
	if result.Err != vm.ErrExecutionReverted {
		return nil
	}
	return common.CopyBytes(result.ReturnData)
}

// Intrinsic gas calculation

func IntrinsicGas(data []byte, contractCreation, isHomestead bool, isEIP2028 bool) (uint64, error) {
	var gas uint64
	if contractCreation && isHomestead {
		gas = params.TxGasContractCreation
	} else {
		gas = params.TxGas
	}

	if len(data) > 0 {
		var nz uint64
		for _, byt := range data {
			if byt != 0 {
				nz++
			}
		}

		nonZeroGas := params.TxDataNonZeroGasFrontier
		if isEIP2028 {
			nonZeroGas = params.TxDataNonZeroGasEIP2028
		}

		if (math.MaxUint64-gas)/nonZeroGas < nz {
			return 0, ErrGasUintOverflow
		}
		gas += nz * nonZeroGas

		z := uint64(len(data)) - nz
		if (math.MaxUint64-gas)/params.TxDataZeroGas < z {
			return 0, ErrGasUintOverflow
		}
		gas += z * params.TxDataZeroGas
	}
	return gas, nil
}

// Constructor

func NewStateTransition(evm *vm.EVM, msg Message, gp *GasPool) *StateTransition {
	return &StateTransition{
		gp:       gp,
		evm:      evm,
		msg:      msg,
		gasPrice: msg.GasPrice(),
		value:    msg.Value(),
		data:     msg.Data(),
		state:    evm.StateDB,
	}
}

// Target address

func (st *StateTransition) to() common.Address {
	if st.msg == nil || st.msg.To() == nil {
		return common.Address{}
	}
	return *st.msg.To()
}

// --------------------------------------
// USDS GASLESS TRANSACTION CHECK
// --------------------------------------

func (st *StateTransition) isUSDSFreeTx() bool {
	if st.msg == nil || st.msg.To() == nil {
		return false
	}
	return *st.msg.To() == params.USDSPrecompileAddress
}

// --------------------------------------
// BUY GAS (PATCHED)
// --------------------------------------

func (st *StateTransition) buyGas() error {

	// Gasless USDS tx:
	if st.isUSDSFreeTx() {

		// Still reserve block gas (anti-spam)
		if err := st.gp.SubGas(st.msg.Gas()); err != nil {
			return err
		}

		st.gas += st.msg.Gas()
		st.initialGas = st.msg.Gas()

		return nil
	}

	// Normal transactions

	mgval := new(big.Int).Mul(new(big.Int).SetUint64(st.msg.Gas()), st.gasPrice)
	if have, want := st.state.GetBalance(st.msg.From()), mgval; have.Cmp(want) < 0 {
		return fmt.Errorf("%w: address %v have %v want %v",
			ErrInsufficientFunds,
			st.msg.From().Hex(),
			have,
			want,
		)
	}

	if err := st.gp.SubGas(st.msg.Gas()); err != nil {
		return err
	}

	st.gas += st.msg.Gas()
	st.initialGas = st.msg.Gas()

	st.state.SubBalance(st.msg.From(), mgval)
	return nil
}

// --------------------------------------

func (st *StateTransition) preCheck() error {

	if st.msg.CheckNonce() {
		stNonce := st.state.GetNonce(st.msg.From())
		if msgNonce := st.msg.Nonce(); stNonce < msgNonce {
			return fmt.Errorf("%w: address %v, tx: %d state: %d",
				ErrNonceTooHigh,
				st.msg.From().Hex(),
				msgNonce,
				stNonce)
		} else if stNonce > msgNonce {
			return fmt.Errorf("%w: address %v, tx: %d state: %d",
				ErrNonceTooLow,
				st.msg.From().Hex(),
				msgNonce,
				stNonce)
		}
	}

	return st.buyGas()
}

// --------------------------------------
// MAIN STATE TRANSITION (PATCHED)
// --------------------------------------

func (st *StateTransition) TransitionDb() (*ExecutionResult, error) {

	if err := st.preCheck(); err != nil {
		return nil, err
	}

	msg := st.msg
	sender := vm.AccountRef(msg.From())

	homestead := st.evm.ChainConfig().IsHomestead(st.evm.Context.BlockNumber)
	istanbul := st.evm.ChainConfig().IsIstanbul(st.evm.Context.BlockNumber)
	contractCreation := msg.To() == nil

	// Intrinsic gas skip for USDS

	if !st.isUSDSFreeTx() {

		gas, err := IntrinsicGas(st.data, contractCreation, homestead, istanbul)
		if err != nil {
			return nil, err
		}
		if st.gas < gas {
			return nil, fmt.Errorf("%w: have %d, want %d", ErrIntrinsicGas, st.gas, gas)
		}
		st.gas -= gas
	}

	if msg.Value().Sign() > 0 && !st.evm.Context.CanTransfer(st.state, msg.From(), msg.Value()) {
		return nil, fmt.Errorf("%w: address %v", ErrInsufficientFundsForTransfer, msg.From().Hex())
	}

	var (
		ret   []byte
		vmerr error
	)

	if contractCreation {

		ret, _, st.gas, vmerr = st.evm.Create(sender, st.data, st.gas, st.value)

	} else {

		st.state.SetNonce(msg.From(), st.state.GetNonce(sender.Address())+1)

		ret, st.gas, vmerr = st.evm.Call(sender, st.to(), st.data, st.gas, st.value)
	}

	st.refundGas()

	// --------------------------------------
	// Miner reward skip for USDS
	// --------------------------------------

	if !st.isUSDSFreeTx() {

		if st.evm.ChainConfig().Sonium != nil {

			st.state.AddBalance(
				consensus.FeeRecoder,
				new(big.Int).Mul(
					new(big.Int).SetUint64(st.gasUsed()),
					st.gasPrice,
				),
			)

		} else {

			st.state.AddBalance(
				st.evm.Context.Coinbase,
				new(big.Int).Mul(
					new(big.Int).SetUint64(st.gasUsed()),
					st.gasPrice,
				),
			)
		}
	}

	return &ExecutionResult{
		UsedGas:    st.gasUsed(),
		Err:        vmerr,
		ReturnData: ret,
	}, nil
}

// --------------------------------------
// REFUND (PATCHED)
// --------------------------------------

func (st *StateTransition) refundGas() {

	refund := st.gasUsed() / 2
	if refund > st.state.GetRefund() {
		refund = st.state.GetRefund()
	}
	st.gas += refund

	// Only refund SESA if it was prepaid
	if !st.isUSDSFreeTx() {

		remaining := new(big.Int).Mul(
			new(big.Int).SetUint64(st.gas),
			st.gasPrice,
		)

		st.state.AddBalance(st.msg.From(), remaining)
	}

	// Always return gas to block pool
	st.gp.AddGas(st.gas)
}

// --------------------------------------

func (st *StateTransition) gasUsed() uint64 {
	return st.initialGas - st.gas
}
