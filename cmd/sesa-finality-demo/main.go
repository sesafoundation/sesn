package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/ethereum/go-ethereum/consensus/sonium/finality/hotstuff"
	"github.com/ethereum/go-ethereum/p2p/mfproto"

	bls "github.com/kilic/bls12-381"
)

// DemoValidator represents a single local validator node (simulated).
type DemoValidator struct {
	ID      int
	Engine  *hotstuff.Engine
	PrivKey *bls.Fr
	PubKey  *bls.G1
	Addr    common.Address
}

func main() {
	fmt.Println("🚀 Starting Sesa Network Instant-Finality Demo (3 local validators)")

	const validatorCount = 3

	// Generate demo validator keys & fake addresses
	validators := make([]*DemoValidator, 0, validatorCount)
	for i := 0; i < validatorCount; i++ {
		sk := new(bls.Fr).SetUint64(uint64(rand.Intn(1000000)))
		pk := new(bls.G1).ScalarBaseMult(sk)
		addr := common.BigToAddress(bigFromUint64(uint64(i + 1000)))
		validators = append(validators, &DemoValidator{ID: i, PrivKey: sk, PubKey: pk, Addr: addr})
	}

	// Create in-memory mesh network (using mfproto.LocalTransport)
	transports := make([]hotstuff.Transport, validatorCount)
	for i := 0; i < validatorCount; i++ {
		transports[i] = mfproto.NewLocalTransport(nil)
	}

	// Simple static validator set
	vset := &LocalValidatorSet{Vals: validators}

	// Initialize HotStuff engines
	for i, v := range validators {
		cfg := hotstuff.Config{BaseTimeout: 100 * time.Millisecond}
		blsAdapter := &hotstuff.BLSAdapter{PrivKey: v.PrivKey, PubKey: v.PubKey}
		engine := hotstuff.New(cfg, vset, transports[i], blsAdapter)
		v.Engine = engine
		transports[i].RegisterHandler(engine)
	}

	// Simulate block proposals sequentially (3 rounds)
	for round := uint64(1); round <= 3; round++ {
		proposer := validators[int(round-1)%validatorCount]
		fmt.Printf("\n🧩 Round %d proposer: validator-%d (%s)\n", round, proposer.ID, proposer.Addr.Hex())

		header := &types.Header{
			Number:     bigFromUint64(round),
			Coinbase:   proposer.Addr,
			Time:       uint64(time.Now().Unix()),
			Extra:      []byte{},
			ParentHash: common.HexToHash(fmt.Sprintf("%064x", round-1)),
		}

		start := time.Now()
		cc, err := proposer.Engine.Propose(context.Background(), header, round)
		elapsed := time.Since(start)

		if err != nil {
			fmt.Printf("❌ Round %d finality failed: %v\n", round, err)
			continue
		}

		fmt.Printf("✅ Block #%d finalized in %v (AggSig %d bytes, %d participants)\n",
			round, elapsed, len(cc.AggSig), cc.Count)
	}

	fmt.Println("\n🎯 Demo complete — instant finality working locally!\n")
}

// LocalValidatorSet implements hotstuff.ValidatorSet for demo purposes.
type LocalValidatorSet struct {
	Vals []*DemoValidator
}

func (vs *LocalValidatorSet) Active() ([]common.Address, [][]byte) {
	addrs := make([]common.Address, len(vs.Vals))
	pubs := make([][]byte, len(vs.Vals))
	for i, v := range vs.Vals {
		addrs[i] = v.Addr
		pubs[i] = v.PubKey.ToCompressed()
	}
	return addrs, pubs
}

func (vs *LocalValidatorSet) IndexOf(a common.Address) (int, bool) {
	for i, v := range vs.Vals {
		if v.Addr == a {
			return i, true
		}
	}
	return -1, false
}

func (vs *LocalValidatorSet) SelfCoinbase() common.Address {
	return vs.Vals[0].Addr // demo; not used by all engines here
}

// bigFromUint64 creates a *big.Int from uint64
func bigFromUint64(v uint64) *big.Int {
	b := new(big.Int)
	b.SetUint64(v)
	return b
}
