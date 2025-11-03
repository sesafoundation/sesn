package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/sesafoundation/sesn/log"
)

type Builder struct {
    mbCounter uint64
    cfg struct {
        Cadence time.Duration
    }
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
