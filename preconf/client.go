package preconf

import (
    "sync"
    "time"
    "github.com/gorilla/websocket"
    "github.com/sesafoundation/sesn/common"
)

type PreconfClient struct {
    mu     sync.RWMutex
    latest *MiniBlock
}

func NewPreconfClient(url string) *PreconfClient {
    c := &PreconfClient{}
    go c.loop(url)
    return c
}

func (c *PreconfClient) Latest() *MiniBlock {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.latest
}
