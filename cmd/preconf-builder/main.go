package main

import (
	"context"
	"crypto/ecdsa"
	"log"
	"flag"
	"net/http"
	"os"
	"time"
	"sync"

	gethrpc "github.com/sesafoundation/sesn/rpc"
	//"github.com/sesafoundation/sesn/log"
	"github.com/sesafoundation/sesn/common"
)

var configPath = flag.String("config", "", "TOML config file for preconf-builder")

// BuilderConfig holds runtime configuration.
//type BuilderConfig struct {
//	IPCPath       string
//	Cadence       time.Duration
//	MaxTxPerSlice int
//	GasSlice      uint64
//	WSListen      string
//	HTTPListen    string
//	NetworkID     uint64
//}

// loadConfig loads from file or uses defaultBuilderConfig (default_config.go).
func loadConfig(path string) (*BuilderConfig, error) {
	cfg := &BuilderConfig{}
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if err := toml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
		log.Info("Loaded config from file", "path", path)
		return cfg, nil
	}
	// fallback to default TOML string
	var def BuilderConfig
	if err := loadDefaultConfig(&def); err != nil {
		return nil, err
	}
	log.Info("Loaded default preconf-builder config")
	return &def, nil
}

func main() {
	var configPath = flag.String("config", "", "Path to TOML config for preconf-builder")
	flag.Parse()

	// Load config
	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Crit("Failed to load config", "err", err)
	}

	// Connect to Geth IPC
	ipc, err := gethrpc.Dial(cfg.IPCPath)
	if err != nil {
		log.Crit("Attach IPC failed", "err", err)
	}
	defer ipc.Close()

	// Load proposer key (you can replace with HSM/keystore)
	var propKey *ecdsa.PrivateKey = loadProposerKey()
	propAddr := deriveAddress(propKey)

	// Initialize builder + websocket hub
	builder := NewBuilder(ipc, propAddr, propKey, *cfg)
	hub := NewWSHub(builder)
	builder.subs = hub

	// WebSocket server (health endpoint included)
	go func() {
		log.Info("preconf WS", "url", "ws://"+cfg.WSListen+"/ws", "healthz", "/healthz")
		if err := http.ListenAndServe(cfg.WSListen, nil); err != nil {
			log.Crit("WS server failed", "err", err)
		}
	}()

	// HTTP JSON-RPC (preconf_getReceipt)
	go ServeHTTPJSON(builder, cfg.HTTPListen)

	// Start builder loop
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	builder.Run(ctx)
}

// ------------ Builder impl (from earlier) ------------
type Builder struct {
	rpc      *gethrpc.Client
	addr     common.Address
	key      *ecdsa.PrivateKey
	cfg      BuilderConfig

	mu       sync.RWMutex
	lastMini  *MiniBlock
	receipts  map[common.Hash]*PreconfReceipt
	subs      *WSHub
	mbCounter uint64
}

func NewBuilder(rpc *gethrpc.Client, addr common.Address, key *ecdsa.PrivateKey, cfg BuilderConfig) *Builder {
	return &Builder{rpc: rpc, addr: addr, key: key, cfg: cfg, receipts: make(map[common.Hash]*PreconfReceipt)}
}

func (b *Builder) Run(ctx context.Context) {
	t := time.NewTicker(b.cfg.Cadence)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			b.emitMiniBlock(ctx)
		}
	}
}

func (b *Builder) emitMiniBlock(ctx context.Context) {
	pending := b.pickPendingTXs(ctx, b.cfg.MaxTxPerSlice, b.cfg.GasSlice)
	b.mbCounter++
	mb := &MiniBlock{
		ID:          b.mbCounter,
		ParentBlock: b.currentHead(ctx),
		TimestampMs: time.Now().UnixMilli(),
		TxHashes:    pending,
		GasPlanned:  b.cfg.GasSlice,
		Signer:      b.addr,
	}
	mb.Signature = signMiniBlock(b.key, mb)

	for _, h := range pending {
		b.receipts[h] = &PreconfReceipt{TxHash: h, MiniBlockID: mb.ID, Signer: b.addr, Signature: mb.Signature}
	}
	b.lastMini = mb
	if b.subs != nil { b.subs.Broadcast(mb) }
}

func (b *Builder) GetReceipt(tx common.Hash) *PreconfReceipt {
	return b.receipts[tx]
}
