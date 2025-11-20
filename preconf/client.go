package preconf

import (
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	//"github.com/sesafoundation/sesn/common"
)

// PreconfClient holds the latest miniblock streamed by the builder
type PreconfClient struct {
	mu     sync.RWMutex
	latest *MiniBlock
}

// NewPreconfClient starts WS connection in the background
func NewPreconfClient(wsURL string) *PreconfClient {
	c := &PreconfClient{}
	go c.loop(wsURL)
	return c
}

// Latest returns most recent miniblock (thread-safe)
func (c *PreconfClient) Latest() *MiniBlock {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.latest
}

// ---- Loop that maintains the websocket feed ----
func (c *PreconfClient) loop(wsURL string) {
	for {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			log.Printf("Preconf WS dial failed (%s): %v — retrying in 1s", wsURL, err)
			time.Sleep(1 * time.Second)
			continue
		}
		log.Printf("Preconf WS connected: %s", wsURL)

		for {
			var mb MiniBlock
			if err := conn.ReadJSON(&mb); err != nil {
				log.Printf("Preconf WS read failed: %v — reconnecting", err)
				_ = conn.Close()
				break
			}

			// store miniblock atomically
			c.mu.Lock()
			c.latest = &mb
			c.mu.Unlock()
		}

		// reconnect after break
		time.Sleep(500 * time.Millisecond)
	}
}
