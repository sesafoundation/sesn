package main

import (
    "context"
    "crypto/ecdsa"
    //"encoding/json"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"
	"sync"
    //"github.com/ethereum/go-ethereum/crypto"

    "github.com/sesafoundation/sesn/log"
    gethrpc "github.com/sesafoundation/sesn/rpc"
	"github.com/sesafoundation/sesn/common"

)

func main() {
    // ---- CLI ----
    configPath := flag.String("config", "", "TOML config file (preconf-builder.toml)")
    flag.Parse()

    // ---- Console logging ----
    log.Root().SetHandler(log.StreamHandler(os.Stdout, log.TerminalFormat(true)))
    log.Info("🚀 preconf-builder starting")

    // ---- Load config (file or fatal) ----
    if *configPath == "" {
        log.Crit("Missing --config flag. Usage: ./preconf-builder --config preconf-builder.toml")
    }

    // parse file into raw (string-based) config
    var raw struct {
        Builder struct {
            IPCPath       string
            Cadence       string
            MaxTxPerSlice int
            GasSlice      uint64
            WSListen      string
            HTTPListen    string
            NetworkID     uint64
            KeystorePath  string
            KeyPassword   string
        }
    }

    data, err := os.ReadFile(*configPath)
    if err != nil {
        log.Crit("Failed to read config file", "err", err)
    }
    if err := toml.Unmarshal(data, &raw); err != nil {
        log.Crit("Invalid TOML config", "err", err)
    }
    log.Info("Config loaded", "file", *configPath)

    // ---- Convert TOML into runtime config ----
    var cfg BuilderConfig
    cfg.IPCPath       = raw.Builder.IPCPath
    cfg.WSListen      = raw.Builder.WSListen
    cfg.HTTPListen    = raw.Builder.HTTPListen
    cfg.NetworkID     = raw.Builder.NetworkID
    cfg.GasSlice      = raw.Builder.GasSlice
    cfg.MaxTxPerSlice = raw.Builder.MaxTxPerSlice
    cfg.KeystorePath  = raw.Builder.KeystorePath
    cfg.KeyPassword   = raw.Builder.KeyPassword

    d, err := time.ParseDuration(raw.Builder.Cadence)
    if err != nil {
        log.Crit("Invalid Cadence format (ex: 100ms)", "err", err)
    }
    cfg.Cadence = d

    // ---- Connect to IPC ----
    log.Info("Attaching IPC", "path", cfg.IPCPath)
    ipc, err := gethrpc.Dial(cfg.IPCPath)
    if err != nil {
        log.Crit("IPC attach failed", "err", err)
    }

    // ---- Load proposer key ----
    propKey := loadProposerKey(cfg.KeystorePath, cfg.KeyPassword)
    propAddr := deriveAddress(propKey)
    log.Info("Proposer key OK", "address", propAddr.Hex())

    // ---- Create builder ----
    builder := NewBuilder(ipc, propAddr, propKey, cfg)
    hub := NewWSHub(builder)
    builder.subs = hub

    // ---- Start WS server ----
    go func() {
        log.Info("📡 WS", "url", "ws://"+cfg.WSListen+"/ws", "health", "/healthz")
        if err := http.ListenAndServe(cfg.WSListen, nil); err != nil {
            log.Crit("WS error", "err", err)
        }
    }()

    // ---- Start HTTP JSON (preconf_getReceipt) ----
    go ServeHTTPJSON(builder, cfg.HTTPListen)

    // ---- Shutdown handler ----
    ctx, cancel := context.WithCancel(context.Background())
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
    go func() {
        <-sigCh
        log.Info("👋 Shutdown signal received")
        cancel()
    }()

    // ---- Main loop (blocks forever) ----
    log.Info("⚙ Running builder loop", "cadence", cfg.Cadence)
    builder.Run(ctx)

    log.Info("🛑 Builder stopped cleanly")
}

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




func (b *Builder) Run(ctx context.Context) {
    cadence := b.cfg.Cadence
    if cadence == 0 {
        cadence = 100 * time.Millisecond
    }

    log.Info(">>> Run() started", "cadence", cadence)

    ticker := time.NewTicker(cadence)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            b.emitMiniBlock(ctx)
        case <-ctx.Done():
            log.Info(">>> Run() stopping — ctx.Done()")
            return
        }
    }
}

func (b *Builder) emitMiniBlock(ctx context.Context) {
    b.mbCounter++
    log.Info("🧱 Emitting mini-block", "id", b.mbCounter, "time", time.Now().Format(time.RFC3339Nano))
}

func NewBuilder(ipc *gethrpc.Client, addr common.Address, key *ecdsa.PrivateKey, cfg BuilderConfig) *Builder {
log.Info("newbuilder")
    return &Builder{
		    cfg: cfg,
    }
}

func (b *Builder) GetReceipt(tx common.Hash) *PreconfReceipt {
	return b.receipts[tx]
}



