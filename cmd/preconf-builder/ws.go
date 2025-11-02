package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	//"github.com/sesafoundation/sesn/preconf-builder"
)

type WSHub struct {
	upgrader websocket.Upgrader
	//mu       sync.RWMutex
	conns    map[*websocket.Conn]struct{}
	builder  *Builder
}

func NewWSHub(b *Builder) *WSHub {
	h := &WSHub{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		conns:   make(map[*websocket.Conn]struct{}),
		builder: b,
	}

	// === HTTP routes ===
	http.HandleFunc("/latest", func(w http.ResponseWriter, r *http.Request) {
		b.mu.RLock()
		defer b.mu.RUnlock()

		mb := b.lastMini
		if mb == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(mb); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	http.HandleFunc("/ws", h.handleWS)
	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return h
}



func (h *WSHub) handleWS(w http.ResponseWriter, r *http.Request) {
	c, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil { log.Println("ws upgrade:", err); return }
	b.mu.Lock(); h.conns[c] = struct{}{}; b.mu.Unlock()

	// read loop (simple RPC over WS)
	go func() {
		defer func() {
			b.mu.Lock(); delete(h.conns, c); b.mu.Unlock()
			c.Close()
		}()
		for {
			_, msg, err := c.ReadMessage()
			if err != nil { return }
			// Simple requests:
			// {"method":"preconf_getReceipt","params":{"txHash":"0x.."}}
			var req struct {
				Method string          `json:"method"`
				Params json.RawMessage `json:"params"`
			}
			if err := json.Unmarshal(msg, &req); err != nil { continue }

			switch req.Method {
			case "preconf_getReceipt":
				var p struct{ TxHash string `json:"txHash"` }
				_ = json.Unmarshal(req.Params, &p)
				rcpt := h.builder.GetReceipt(hexToHash(p.TxHash))
				resp, _ := json.Marshal(rcpt)
				c.WriteMessage(websocket.TextMessage, resp)
			default:
				// ignore
			}
		}
	}()
}

func (h *WSHub) Broadcast(mb *MiniBlock) {
	b.mu.RLock(); defer b.mu.RUnlock()
	data, _ := json.Marshal(mb)
	for c := range h.conns {
		_ = c.WriteMessage(websocket.TextMessage, data)
	}
}

// small helper
func hexToHash(s string) (h [32]byte) {
	b := []byte{}
	if len(s) >= 2 && s[:2] == "0x" { s = s[2:] }
	if len(s)%2 == 1 { s = "0" + s }
	bz := make([]byte, len(s)/2)
	for i := 0; i < len(bz); i++ {
		var v byte
		for _, ch := range []byte{s[2*i], s[2*i+1]} {
			v <<= 4
			switch {
			case ch >= '0' && ch <= '9': v |= ch - '0'
			case ch >= 'a' && ch <= 'f': v |= ch - 'a' + 10
			case ch >= 'A' && ch <= 'F': v |= ch - 'A' + 10
			}
		}
		bz[i] = v
	}
	copy(h[:], bz)
	return
}
