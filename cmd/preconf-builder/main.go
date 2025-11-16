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
    // ---- Force logging to console ----
    log.Info("IPC Path check", "IPCPath", os.Getenv("BUILDER_IPC"))
    log.Root().SetHandler(log.StreamHandler(os.Stdout, log.TerminalFormat(true)))
    log.Info(">>> preconf-builder starting")

    // ---- TEMP DEBUG ----
    log.Info(">>> reached top of main()")

    // ---- Minimal config for testing ----
    cfg := BuilderConfig{
        Cadence:    100 * time.Millisecond,
       // IPCPath:    os.Getenv("BUILDER_IPC"),
	  	IPCPath  : "/node1/setd.ipc",
        WSListen:   ":8556",
        HTTPListen: ":8557",
    }

    // ---- Connect to IPC (but do NOT exit if fail) ----
    ipc, err := gethrpc.Dial(cfg.IPCPath)
    if err != nil {
        log.Info("IPC Path check", "IPCPath", os.Getenv("BUILDER_IPC"))
        log.Warn("IPC attach failed, continuing anyway", "err", err)
        ipc = nil
    }

    var propKey *ecdsa.PrivateKey = loadProposerKey()
    propAddr := deriveAddress(propKey)

    // ---- Create builder ----
    builder := NewBuilder(ipc, propAddr, propKey, cfg)

    log.Info(">>> Builder created OK")

    // ---- Start WS + HTTP (background) ----
    go func() {
        log.Info("WS server starting", "addr", cfg.WSListen)
        http.ListenAndServe(cfg.WSListen, nil)
    }()
    go ServeHTTPJSON(builder, cfg.HTTPListen)

    // ---- Handle SIGINT ----
    ctx, cancel := context.WithCancel(context.Background())
    go func() {
        sig := make(chan os.Signal, 1)
        signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
        <-sig
        log.Info(">>> Received shutdown signal")
        cancel()
    }()

    log.Info(">>> About to call builder.Run()")

    builder.Run(ctx) // 🔥 blocks here forever

    log.Info(">>> builder.Run() returned — should only happen on shutdown")
    log.Info(">>> preconf-builder exiting cleanly")
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

func loadProposerKey() *ecdsa.PrivateKey {
    hexKey := os.Getenv("BUILDER_KEY")
    if hexKey == "" {
        log.Crit("BUILDER_KEY not set — export the private key hex without 0x")
    }
    key, err := crypto.HexToECDSA(hexKey)
    if err != nil {
        log.Crit("Invalid BUILDER_KEY", "err", err)
    }
    return key
}


