package preconfclient

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sesafoundation/sesn/common"
)

type MiniBlock struct {
	ID          uint64          `json:"id"`
	ParentBlock common.Hash     `json:"parentBlock"`
	TimestampMs int64           `json:"timestampMs"`
	TxHashes    []common.Hash   `json:"txHashes"`
	GasPlanned  uint64          `json:"gasPlanned"`
	Signer      common.Address  `json:"signer"`
	Signature   []byte          `json:"signature"`
}

type Client struct {
	baseURL string        // e.g., http://127.0.0.1:8556
	cli     *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		cli: &http.Client{
			Timeout: 300 * time.Millisecond, // keep it snappy
		},
	}
}

func (c *Client) LatestMiniBlock() (*MiniBlock, error) {
	req, _ := http.NewRequest("GET", c.baseURL+"/latest", nil)
	resp, err := c.cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 204 {
		return nil, nil
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("preconf: %s", resp.Status)
	}
	var mb MiniBlock
	if err := json.NewDecoder(resp.Body).Decode(&mb); err != nil {
		return nil, err
	}
	return &mb, nil
}
