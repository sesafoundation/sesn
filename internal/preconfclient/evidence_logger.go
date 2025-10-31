package preconfclient

import (
	"encoding/json"
	"os"
	"sync"

	"github.com/ethereum/go-ethereum/common"
)

type Evidence struct {
	Proposer common.Address `json:"proposer"`
	Victim   common.Address `json:"victim"`
	TxHash   common.Hash    `json:"txHash"`
	Reason   string         `json:"reason"` // "missing" | "constraint" | "gasoverflow" | "misordered"
	PayWei   string         `json:"payWei"` // string for big ints; fill a constant for now
}

type EvidenceLogger struct {
	filename string
	mu       sync.Mutex
}

func NewEvidenceLogger(filename string) *EvidenceLogger {
	return &EvidenceLogger{filename: filename}
}

func (l *EvidenceLogger) Append(ev Evidence) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	var arr []Evidence
	if bz, err := os.ReadFile(l.filename); err == nil && len(bz) > 0 {
		_ = json.Unmarshal(bz, &arr) // best-effort
	}
	arr = append(arr, ev)
	bz, _ := json.MarshalIndent(arr, "", "  ")
	return os.WriteFile(l.filename, bz, 0o644)
}
