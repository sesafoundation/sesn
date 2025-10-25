package eth

import (
	"math/big"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/ethereum/go-ethereum/eth/gasprice"
	"github.com/ethereum/go-ethereum/miner"
	"github.com/ethereum/go-ethereum/params"

	// --- added imports ---
	"github.com/ethereum/go-ethereum/consensus/sonium"
	"github.com/ethereum/go-ethereum/consensus/sonium/finality/hotstuff"
	"github.com/ethereum/go-ethereum/p2p/mfproto"

	// BLS12-381 for real aggregation
	bls12381 "github.com/kilic/bls12-381"
)

// DefaultFullGPOConfig contains default gasprice oracle settings for full node.
var DefaultFullGPOConfig = gasprice.Config{
	Blocks:     20,
	Percentile: 60,
	MaxPrice:   gasprice.DefaultMaxPrice,
}

// DefaultLightGPOConfig contains default gasprice oracle settings for light client.
var DefaultLightGPOConfig = gasprice.Config{
	Blocks:     2,
	Percentile: 60,
	MaxPrice:   gasprice.DefaultMaxPrice,
}

// DefaultConfig contains default settings for use on the Ethereum main net.
var DefaultConfig = Config{
	SyncMode: downloader.FastSync,
	Ethash: ethash.Config{
		CacheDir:         "ethash",
		CachesInMem:      2,
		CachesOnDisk:     3,
		CachesLockMmap:   false,
		DatasetsInMem:    1,
		DatasetsOnDisk:   2,
		DatasetsLockMmap: false,
	},
	NetworkId:               2250,
	LightPeers:              100,
	UltraLightFraction:      75,
	DatabaseCache:           512,
	TrieCleanCache:          154,
	TrieCleanCacheJournal:   "triecache",
	TrieCleanCacheRejournal: 60 * time.Minute,
	TrieDirtyCache:          256,
	TrieTimeout:             60 * time.Minute,
	SnapshotCache:           102,
	Miner: miner.Config{
		GasFloor: 30000000,
		GasCeil:  42000000,
		GasPrice: big.NewInt(500 * params.GWei),
		Recommit: 3 * time.Second,
	},
	TxPool:      core.DefaultTxPoolConfig,
	RPCGasCap:   25000000,
	GPO:         DefaultFullGPOConfig,
	RPCTxFeeCap: 1, // 1 ether
}

var DefaultTestnetConfig = Config{
	SyncMode: downloader.FastSync,
	Ethash: ethash.Config{
		CacheDir:         "ethash",
		CachesInMem:      2,
		CachesOnDisk:     3,
		CachesLockMmap:   false,
		DatasetsInMem:    1,
		DatasetsOnDisk:   2,
		DatasetsLockMmap: false,
	},
	NetworkId:               2249,
	LightPeers:              100,
	UltraLightFraction:      75,
	DatabaseCache:           512,
	TrieCleanCache:          154,
	TrieCleanCacheJournal:   "triecache",
	TrieCleanCacheRejournal: 60 * time.Minute,
	TrieDirtyCache:          256,
	TrieTimeout:             60 * time.Minute,
	SnapshotCache:           102,
	Miner: miner.Config{
		GasFloor: 30000000,
		GasCeil:  42000000,
		GasPrice: big.NewInt(500 * params.GWei),
		Recommit: 3 * time.Second,
	},
	TxPool:      core.DefaultTxPoolConfig,
	RPCGasCap:   25000000,
	GPO:         DefaultFullGPOConfig,
	RPCTxFeeCap: 1, // 1 ether
}

func init() {
	home := os.Getenv("HOME")
	if home == "" {
		if user, err := user.Current(); err == nil {
			home = user.HomeDir
		}
	}
	if runtime.GOOS == "darwin" {
		DefaultConfig.Ethash.DatasetDir = filepath.Join(home, "Library", "Ethash")
	} else if runtime.GOOS == "windows" {
		localappdata := os.Getenv("LOCALAPPDATA")
		if localappdata != "" {
			DefaultConfig.Ethash.DatasetDir = filepath.Join(localappdata, "Ethash")
		} else {
			DefaultConfig.Ethash.DatasetDir = filepath.Join(home, "AppData", "Local", "Ethash")
		}
	} else {
		DefaultConfig.Ethash.DatasetDir = filepath.Join(home, ".ethash")
	}
}

//go:generate gencodec -type Config -formats toml -out gen_config.go

type Config struct {
	Genesis                 *core.Genesis `toml:",omitempty"`
	NetworkId               uint64
	SyncMode                downloader.SyncMode
	DiscoveryURLs           []string
	NoPruning, NoPrefetch   bool
	TxLookupLimit           uint64 `toml:",omitempty"`
	Whitelist               map[uint64]common.Hash `toml:"-"`
	LightServ, LightIngress int                    `toml:",omitempty"`
	LightEgress, LightPeers int                    `toml:",omitempty"`
	LightNoPrune            bool                   `toml:",omitempty"`
	UltraLightServers       []string               `toml:",omitempty"`
	UltraLightFraction      int                    `toml:",omitempty"`
	UltraLightOnlyAnnounce  bool                   `toml:",omitempty"`
	SkipBcVersionCheck      bool                   `toml:"-"`
	DatabaseHandles         int                    `toml:"-"`
	DatabaseCache           int
	DatabaseFreezer         string
	TrieCleanCache          int
	TrieCleanCacheJournal   string        `toml:",omitempty"`
	TrieCleanCacheRejournal time.Duration `toml:",omitempty"`
	TrieDirtyCache          int
	TrieTimeout             time.Duration
	SnapshotCache           int
	Preimages               bool
	Miner                   miner.Config
	Ethash                  ethash.Config
	TxPool                  core.TxPoolConfig
	GPO                     gasprice.Config
	EnablePreimageRecording bool
	DocRoot                 string `toml:"-"`
	EWASMInterpreter        string
	EVMInterpreter          string
	RPCGasCap               uint64  `toml:",omitempty"`
	RPCTxFeeCap             float64 `toml:",omitempty"`
	Checkpoint              *params.TrustedCheckpoint        `toml:",omitempty"`
	CheckpointOracle        *params.CheckpointOracleConfig   `toml:",omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// 🌐 Sonium + Instant-Finality Engine Initialization
///////////////////////////////////////////////////////////////////////////////

// NewConsensusEngine creates the Sonium DPoS engine with optional HotStuff finality.
func NewConsensusEngine(cfg *params.ChainConfig, backend core.EngineBackend) (consensus.Engine, error) {
	// Base Sonium engine
	base := sonium.New(backend, cfg)

	// If no finality configuration, return base engine
	if cfg.Finality == nil || cfg.Finality.Type != "hotstuff" {
		return base, nil
	}

	// --- Real BLS12-381 key setup ---
	// Each validator should have its own private key; this example just creates a dummy pair
	// for demonstration. In production, derive from your validator keystore.
	sk := bls12381.NewKey()
	pk := new(bls12381.G1).ScalarBaseMult(sk)
	_ = pk // store or publish pk for validator discovery

	// BLS adapter implementing our gadget's interface
	blsAdapter := &hotstuff.BLSAdapter{
		PrivKey: sk,
		PubKey:  pk,
	}

	// Validator set from DPoS contract/state
	vs := sonium.NewValidatorSet(backend)

	// Mini-finality transport (in-proc stub; replace with p2p for real net)
	tr := mfproto.NewLocalTransport(nil)

	// Configure adaptive finality with ~100 ms timeout
	gadget := hotstuff.New(
		hotstuff.Config{BaseTimeout: time.Duration(cfg.Finality.TimeoutMS) * time.Millisecond},
		vs, tr, blsAdapter,
	)

	engine := sonium.WithFinality(base, gadget)

	// Optionally attach P2P protocol for real vote gossip
	if backend.NodeServer() != nil {
		backend.NodeServer().Protocols = append(
			backend.NodeServer().Protocols,
			mfproto.Protocol(gadget),
		)
	}

	return engine, nil
}
