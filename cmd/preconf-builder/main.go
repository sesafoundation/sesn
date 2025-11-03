package main

import (
	"context"
	"crypto/ecdsa"
	//"log"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"
	"sync"
	"strings"
	"syscall"

	gethrpc "github.com/sesafoundation/sesn/rpc"
	"github.com/sesafoundation/sesn/log"
	"github.com/sesafoundation/sesn/common"
	"github.com/naoina/toml"
)

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

// loadConfig loads from file or uses defaultBuilderConfig (default_config.go).
func expandHome(path string) string {
	if strings.HasPrefix(path, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}

// loadConfig loads builder configuration from a TOML file if provided,
// otherwise it falls back to the embedded defaults from default_config.go.
func loadConfig(path string) (*BuilderConfig, error) {
	cfg := &BuilderConfig{}

	// --- Load from file if specified ---
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			log.Error("Failed to read config file", "path", path, "err", err)
			return nil, err
		}
		if err := toml.Unmarshal(data, cfg); err != nil {
			log.Error("Failed to parse TOML config", "path", path, "err", err)
			return nil, err
		}
		log.Info("Loaded preconf-builder config from file", "path", path)
	} else {
		// --- Load from embedded defaults ---
		if err := loadDefaultConfig(cfg); err != nil {
			log.Error("Failed to load embedded default config", "err", err)
			return nil, err
		}
		log.Info("Loaded embedded default preconf-builder config")
	}

	// --- Normalize and post-process ---
	cfg.IPCPath = expandHome(cfg.IPCPath)
	if cfg.Cadence == 0 {
		//cfg.Cadence = 100_000_000 // fallback: 100ms
		  cfg.Cadence = 100 * time.Millisecond
	}

	return cfg, nil
}


func main() {
	var configPath = flag.String("config", "", "Path to TOML config for preconf-builder")
	flag.Parse()

	// ---- Load config (file or defaults) ----
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
		log.Info("Loaded default preconf-builder config (embedded)")
	}

	// ---- Announce startup ----
	log.Info("Starting preconf-builder",
		"IPCPath", cfg.IPCPath,
		"Cadence", cfg.Cadence,
		"WS", cfg.WSListen,
		"HTTP", cfg.HTTPListen,
	)

	// ---- Connect to Geth IPC ----
	ipc, err := gethrpc.Dial(cfg.IPCPath)
	if err != nil {
		log.Crit("Attach IPC failed", "err", err)
	}
	defer ipc.Close()

	// ---- Load proposer key (ECDSA / HSM / keystore) ----
	var propKey *ecdsa.PrivateKey = loadProposerKey()
	propAddr := deriveAddress(propKey)

	// ---- Initialize builder + websocket hub ----
	builder := NewBuilder(ipc, propAddr, propKey, cfg)
	hub := NewWSHub(builder)
	builder.subs = hub

	// ---- Start WebSocket server (async) ----
	go func() {
		log.Info("preconf WS listening",
			"url", "ws://"+cfg.WSListen+"/ws",
			"healthz", "/healthz",
		)
		if err := http.ListenAndServe(cfg.WSListen, nil); err != nil {
			log.Crit("WS server failed", "err", err)
		}
	}()

	// ---- Start HTTP JSON-RPC (async) ----
	go ServeHTTPJSON(builder, cfg.HTTPListen)

	// ---- Setup graceful shutdown ----
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Info("Received shutdown signal", "signal", sig)
		cancel()
	}()

	// ---- Run main builder loop ----
	builder.Run(ctx)

	// ---- Wait until context is done ----
	<-ctx.Done()
	log.Info("Preconf-builder stopped cleanly")
}
func xNewBuilder(rpc *gethrpc.Client, addr common.Address, key *ecdsa.PrivateKey, cfg BuilderConfig) *Builder {
	return &Builder{rpc: rpc, addr: addr, key: key, cfg: cfg, receipts: make(map[common.Hash]*PreconfReceipt)}
}

func (b *Builder) xRun(ctx context.Context) {
    ticker := time.NewTicker(b.cfg.Cadence)
    defer ticker.Stop()

    log.Info("Builder running", "cadence", b.cfg.Cadence)

    for {
        select {
        case <-ticker.C:
            b.emitMiniBlock(ctx)
        case <-ctx.Done():
            log.Info("Builder stopped")
            return
        }
    }
}


func (b *Builder) xemitMiniBlock(ctx context.Context) {
	txs := b.pickPendingTXs(ctx, b.cfg.MaxTxPerSlice, b.cfg.GasSlice)
	if len(txs) == 0 {
		return // nothing to emit this cycle
	}

	b.mbCounter++
	mb := &MiniBlock{
		ID:          b.mbCounter,
		ParentBlock: b.currentHead(ctx),
		TimestampMs: time.Now().UnixMilli(),
		TxHashes:    txs,
		GasPlanned:  b.cfg.GasSlice,
		Signer:      b.addr,
	}

	mb.Signature = signMiniBlock(b.key, mb)

	// Record receipts
	for _, h := range txs {
		b.receipts[h] = &PreconfReceipt{
			TxHash:      h,
			MiniBlockID: mb.ID,
			Signer:      b.addr,
			Signature:   mb.Signature,
		}
	}

	b.mu.Lock()
	b.lastMini = mb
	b.mu.Unlock()

	if b.subs != nil {
		b.subs.Broadcast(mb)
	}

	log.Info("Emitted mini-block", "id", mb.ID, "txs", len(mb.TxHashes))
}

func NewBuilder(ipc *gethrpc.Client, addr common.Address, key *ecdsa.PrivateKey, cfg BuilderConfig) *Builder {
    return &Builder{
        cfg: cfg,
    }
}

func (b *Builder) Run(ctx context.Context) {
    ticker := time.NewTicker(b.cfg.Cadence)
    defer ticker.Stop()

    log.Info("Builder loop started", "cadence", b.cfg.Cadence)

    for {
        select {
        case <-ticker.C:
            b.emitMiniBlock(ctx)
        case <-ctx.Done():
            log.Info("Builder loop stopped")
            return
        }
    }
}

func (b *Builder) emitMiniBlock(ctx context.Context) {
    b.mbCounter++
    log.Info("Emitted mini-block",
        "id", b.mbCounter,
        "timestamp", time.Now().UnixMilli())
}

func (b *Builder) GetReceipt(tx common.Hash) *PreconfReceipt {
	return b.receipts[tx]
}

