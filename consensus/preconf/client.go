package preconf

import (
    "encoding/json"
    "io"
    "net/http"
    "time"

    "github.com/sesafoundation/sesn/common"
)

// MiniBlock (same shape as builder output)
type MiniBlock struct {
    ID          uint64         `json:"id"`
    ParentBlock common.Hash    `json:"parentBlock"`
    TimestampMs int64          `json:"timestampMs"`
    TxHashes    []common.Hash  `json:"txHashes"`
    GasPlanned  uint64         `json:"gasPlanned"`
    Signer      common.Address `json:"signer"`
    Signature   []byte         `json:"signature"`
}

type Client struct {
    url string
    http *http.Client
}

func New(url string) *Client {
    return &Client{
        url:  url,
        http: &http.Client{Timeout: 200 * time.Millisecond},
    }
}

// LatestMiniBlock fetches /latest from builder
func (c *Client) LatestMiniBlock() (*MiniBlock, error) {
    resp, err := c.http.Get(c.url + "/latest")
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    if resp.StatusCode != 200 {
        io.Copy(io.Discard, resp.Body)
        return nil, nil
    }
    var mb MiniBlock
    if err := json.NewDecoder(resp.Body).Decode(&mb); err != nil {
        return nil, err
    }
    return &mb, nil
}
