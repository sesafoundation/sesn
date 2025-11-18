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
    "flag"  
    "github.com/naoina/toml"
    //"github.com/ethereum/go-ethereum/crypto"

    "github.com/sesafoundation/sesn/log"
    gethrpc "github.com/sesafoundation/sesn/rpc"
	"github.com/sesafoundation/sesn/common"

)

func main() {
    configPath := flag.String("config", "", "TOML config file (optional)")
    flag.Parse()

    log.Root().SetHandler(log.StreamHandler(os.Stdout, log.TerminalFormat(true)))
    log.Info("🚀 preconf-builder starting")

    // ---- raw TOML struct ----
    var raw RawConfig

    if *configPath != "" {
        // Load config from file
        data, err := os.ReadFile(*configPath)
        if err != nil { log.Crit("Failed to read config", "err", err) }
        if err := toml.Unmarshal(data, &raw); err != nil {
            log.Crit("Invalid TOML config", "err", err)
        }
        log.Info("Config loaded", "file", *configPath)
    } else {
        // Fallback: embedded default config
        if err := loadDefaultConfig(&raw); err != nil {
            log.Crit("Failed to load embedded default config", "err", err)
        }
        log.Info("Loaded default embedded config")
    }

    // ---- Convert raw -> runtime cfg ----
    var cfg BuilderConfig
    d, err := time.ParseDuration(raw.Builder.Cadence)
    if err != nil { log.Crit("Invalid cadence", "err", err) }

    cfg.Cadence       = d
    cfg.IPCPath       = raw.Builder.IPCPath
    cfg.WSListen      = raw.Builder.WSListen
    cfg.HTTPListen    = raw.Builder.HTTPListen
    cfg.NetworkID     = raw.Builder.NetworkID
    cfg.GasSlice      = raw.Builder.GasSlice
    cfg.MaxTxPerSlice = raw.Builder.MaxTxPerSlice
    cfg.KeystorePath  = raw.Builder.KeystorePath
    cfg.KeyPassword   = raw.Builder.KeyPassword

    log.Info("Config summary",
        "IPC", cfg.IPCPath,
        "Cadence", cfg.Cadence,
        "WS", cfg.WSListen,
        "HTTP", cfg.HTTPListen,
    )

    // ---- attach IPC ----
    ipc, err := gethrpc.Dial(cfg.IPCPath)
    if err != nil { log.Crit("IPC attach failed", "err", err) }
    defer ipc.Close()

    // ---- load proposer ----
    propKey := loadProposerKey(cfg.KeystorePath, cfg.KeyPassword)
    propAddr := deriveAddress(propKey)
    log.Info("Proposer loaded", "address", propAddr.Hex())

    // ---- build + networking ----
    builder := NewBuilder(ipc, propAddr, propKey, cfg)
    hub := NewWSHub(builder)
    builder.subs = hub

    go func() {
        log.Info("📡 WS", "url", "ws://"+cfg.WSListen+"/ws")
        http.ListenAndServe(cfg.WSListen, nil)
    }()
    go ServeHTTPJSON(builder, cfg.HTTPListen)

    // ---- graceful shutdown ----
    ctx, cancel := context.WithCancel(context.Background())
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
    go func() { <-sigCh; cancel() }()

    log.Info("⚙ running", "cadence", cfg.Cadence)
    builder.Run(ctx)
    log.Info("🛑 stopped")
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



