package main

import (
	"context"
	"crypto/ecdsa"
	"log"
	"net/http"
	"time"
	"sync"

	"github.com/sesafoundation/sesn/common"
	gethrpc "github.com/sesafoundation/sesn/rpc"
)

func main() {
	cfg := BuilderConfig{
    Cadence:       100 * time.Millisecond, 
    MaxTxPerSlice: 1500,
    GasSlice:      3_000_000,
    IPCPath:       "/path/to/geth.ipc",
    WSListen:      ":8556",
    HTTPListen:    ":8557",
    NetworkID:     2250,
}

	ipc, err := gethrpc.Dial(cfg.IPCPath)
	if err != nil { log.Fatalf("attach IPC: %v", err) }
	defer ipc.Close()

	// TODO: replace with proper keystore/HSM loader
	var propKey *ecdsa.PrivateKey = loadProposerKey()
	propAddr := deriveAddress(propKey)

	builder := NewBuilder(ipc, propAddr, propKey, cfg)
	hub := NewWSHub(builder)
	builder.subs = hub

	// Health + WS
	go func() {
		log.Printf("preconf WS at ws://%s/ws  (healthz at /healthz)", cfg.WSListen)
		log.Fatal(http.ListenAndServe(cfg.WSListen, nil))
	}()

	// Minimal HTTP JSON-RPC for preconf_getReceipt
	ServeHTTPJSON(builder, cfg.HTTPListen)

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
