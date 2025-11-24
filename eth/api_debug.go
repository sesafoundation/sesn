package eth

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/sesafoundation/sesn/accounts"
	"github.com/sesafoundation/sesn/accounts/abi"
	"github.com/sesafoundation/sesn/accounts/keystore"
	"github.com/sesafoundation/sesn/accounts/scwallet"
	"github.com/sesafoundation/sesn/common"
	"github.com/sesafoundation/sesn/common/hexutil"
	"github.com/sesafoundation/sesn/common/math"
	"github.com/sesafoundation/sesn/consensus/clique"
	"github.com/sesafoundation/sesn/consensus/ethash"
	"github.com/sesafoundation/sesn/core"
	"github.com/sesafoundation/sesn/core/types"
	"github.com/sesafoundation/sesn/core/vm"
	"github.com/sesafoundation/sesn/crypto"
	"github.com/sesafoundation/sesn/log"
	"github.com/sesafoundation/sesn/p2p"
	"github.com/sesafoundation/sesn/params"
	"github.com/sesafoundation/sesn/rlp"
	"github.com/sesafoundation/sesn/rpc"
	"github.com/tyler-smith/go-bip39"
)

// PrivateDebugAPI is the collection of Ethereum APIs exposed over the private
// debugging endpoint.
type PrivateDebugAPI struct {
   eth *Ethereum
}

// NewPrivateDebugAPI creates a new API definition for the private debug methods
// of the Ethereum service.
func NewPrivateDebugAPI(e *Ethereum) *PrivateDebugAPI {
    return &PrivateDebugAPI{eth: e}
}

// ChaindbProperty returns leveldb properties of the key-value database.
func (api *PrivateDebugAPI) ChaindbProperty(property string) (string, error) {
	if property == "" {
		property = "leveldb.stats"
	} else if !strings.HasPrefix(property, "leveldb.") {
		property = "leveldb." + property
	}
	return api.eth.ChainDb().Stat(property)
}

// ChaindbCompact flattens the entire key-value database into a single level,
// removing all unused slots and merging all keys.
func (api *PrivateDebugAPI) ChaindbCompact() error {
	for b := byte(0); b < 255; b++ {
		log.Info("Compacting chain database", "range", fmt.Sprintf("0x%0.2X-0x%0.2X", b, b+1))
		if err := api.eth.ChainDb().Compact([]byte{b}, []byte{b + 1}); err != nil {
			log.Error("Database compaction failed", "err", err)
			return err
		}
	}
	return nil
}

// SetHead rewinds the head of the blockchain to a previous block.
func (api *PrivateDebugAPI) SetHead(number hexutil.Uint64) {
	api.eth.SetHead(uint64(number))
}



//public debug

type PublicDebugAPI struct {
	eth *Ethereum
}

// NewPublicDebugAPI creates a new API definition for the public debug methods
// of the Ethereum service.
func NewPublicDebugAPI(eth *Ethereum) *PublicDebugAPI {
	return &PublicDebugAPI{eth: eth}
}

// GetBlockRlp retrieves the RLP encoded for of a single block.
func (api *PublicDebugAPI) GetBlockRlp(ctx context.Context, number uint64) (string, error) {
	    block, _ := api.eth.BlockByNumber(ctx, rpc.BlockNumber(number))
	if block == nil {
		return "", fmt.Errorf("block #%d not found", number)
	}
	encoded, err := rlp.EncodeToBytes(block)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", encoded), nil
}

// TestSignCliqueBlock fetches the given block number, and attempts to sign it as a clique header with the
// given address, returning the address of the recovered signature
//
// This is a temporary method to debug the externalsigner integration,
// TODO: Remove this method when the integration is mature
func (api *PublicDebugAPI) TestSignCliqueBlock(ctx context.Context, address common.Address, number uint64) (common.Address, error) {
	block, _ := api.eth.BlockByNumber(ctx, rpc.BlockNumber(number))
	if block == nil {
		return common.Address{}, fmt.Errorf("block #%d not found", number)
	}
	header := block.Header()
	header.Extra = make([]byte, 32+65)
	encoded := clique.CliqueRLP(header)

	// Look up the wallet containing the requested signer
	account := accounts.Account{Address: address}
	wallet, err := api.eth.AccountManager().Find(account)
	if err != nil {
		return common.Address{}, err
	}

	signature, err := wallet.SignData(account, accounts.MimetypeClique, encoded)
	if err != nil {
		return common.Address{}, err
	}
	sealHash := clique.SealHash(header).Bytes()
	log.Info("test signing of clique block",
		"Sealhash", fmt.Sprintf("%x", sealHash),
		"signature", fmt.Sprintf("%x", signature))
	pubkey, err := crypto.Ecrecover(sealHash, signature)
	if err != nil {
		return common.Address{}, err
	}
	var signer common.Address
	copy(signer[:], crypto.Keccak256(pubkey[1:])[12:])

	return signer, nil
}

// PrintBlock retrieves a block and returns its pretty printed form.
func (api *PublicDebugAPI) PrintBlock(ctx context.Context, number uint64) (string, error) {
	block, _ := api.eth.BlockByNumber(ctx, rpc.BlockNumber(number))
	if block == nil {
		return "", fmt.Errorf("block #%d not found", number)
	}
	return spew.Sdump(block), nil
}

// SeedHash retrieves the seed hash of a block.
func (api *PublicDebugAPI) SeedHash(ctx context.Context, number uint64) (string, error) {
	block, _ := api.eth.BlockByNumber(ctx, rpc.BlockNumber(number))
	if block == nil {
		return "", fmt.Errorf("block #%d not found", number)
	}
	return fmt.Sprintf("0x%x", ethash.SeedHash(number)), nil
}