package main

import (
	"context"
	"crypto/ecdsa"
	//"log"
	"flag"
	"net/http"
	"os"
	"time"
	"sync"

	gethrpc "github.com/sesafoundation/sesn/rpc"
	"github.com/sesafoundation/sesn/log"
	"github.com/sesafoundation/sesn/common"
	"github.com/naoina/toml"
)

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
		log.Info("Loaded preconf-builder config from file", "path", path)
		return cfg, nil
	}

	if err := loadDefaultConfig(cfg); err != nil {
		return nil, err
	}
	log.Info("Loaded embedded default preconf-builder config")
	return cfg, nil
}

func main() {
	var configPath = flag.String("config", "", "Path to TOML config for preconf-builder")
	flag.Parse()

	// Load config (from file or defaults)
	var cfg BuilderConfig
	if *configPath != "" {
		data, err := os.ReadFile(*configPath)
		if err != nil {
			log.Crit("Failed to read config", "err", err)
		}
		if err := toml.Unmarshal(data, &cfg); err != nil {
			log.Crit("Invalid TOML config", "err", err)
		}
		log.Info("Loaded preconf-builder config from file", "path", *configPath)
	} else {
		if err := loadDefaultConfig(&cfg); err != nil {
			log.Crit("Failed to load default config", "err", err)
		}
		log.Info("Loaded default preconf-builder config")
	}

	ipc, err := gethrpc.Dial(cfg.IPCPath)
	if err != nil {
		log.Crit("Attach IPC failed", "err", err)
	}
	defer ipc.Close()

	propKey := loadProposerKey()
	propAddr := deriveAddress(propKey)

	builder := NewBuilder(ipc, propAddr, propKey, cfg)
	hub := NewWSHub(builder)
	builder.subs = hub

	go func() {
		log.Info("preconf WS", "url", "ws://"+cfg.WSListen+"/ws", "healthz", "/healthz")
		if err := http.ListenAndServe(cfg.WSListen, nil); err != nil {
			log.Crit("WS server failed", "err", err)
		}
	}()

	go ServeHTTPJSON(builder, cfg.HTTPListen)

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
