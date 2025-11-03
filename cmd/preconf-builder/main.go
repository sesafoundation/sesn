package main


import (
	"context"
	"crypto/ecdsa"
	//"log"
	//"flag"
	//"net/http"
	"os"
	"os/signal"
	//"path/filepath"
	"time"
	"sync"
	//"strings"
	"syscall"

	gethrpc "github.com/sesafoundation/sesn/rpc"
	"github.com/sesafoundation/sesn/log"
	"github.com/sesafoundation/sesn/common"
	//uo"github.com/naoina/toml"
)
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
    if b.cfg.Cadence == 0 {
        b.cfg.Cadence = 100 * time.Millisecond
    }
    log.Info("✅ Builder started", "cadence", b.cfg.Cadence)

    ticker := time.NewTicker(b.cfg.Cadence)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            b.mbCounter++
            log.Info("🧱 Emitted mini-block", "id", b.mbCounter)
        case <-ctx.Done():
            log.Info("🛑 Builder stopped")
            return
        }
    }
}

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
    go func() {
        sig := <-sigCh
        log.Info("Received shutdown signal", "signal", sig)
        cancel()
    }()

    builder := &Builder{}
    builder.Run(ctx)
}

func (b *Builder) GetReceipt(tx common.Hash) *PreconfReceipt {
	return b.receipts[tx]
}
