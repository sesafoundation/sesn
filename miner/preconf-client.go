package miner

import (
	"encoding/json"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sesafoundation/sesn/common"
)

type MiniBlock struct {
	ID        uint64         `json:"id"`
	TxHashes  []common.Hash  `json:"txHashes"`
	Timestamp int64          `json:"timestampMs"`
}

type PreconfClient struct {
	ws   *websocket.Conn
	last *MiniBlock         // most recent miniblock received
}

func NewPreconfClient(endpoint string) *PreconfClient {
	u, _ := url.Parse(endpoint)
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return nil
	}
	pc := &PreconfClient{ws: conn}
	go pc.listenLoop()
	return pc
}

func (pc *PreconfClient) listenLoop() {
	for {
		_, data, err := pc.ws.ReadMessage()
		if err != nil {
			return
		}
		var mb MiniBlock
		if json.Unmarshal(data, &mb) == nil {
			pc.last = &mb
		}
	}
}

// returns nil if no miniblock yet
func (pc *PreconfClient) Latest() *MiniBlock {
	return pc.last
}
