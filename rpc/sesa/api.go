package sesa

import (
	"context"
	fin "github.com/ethereum/go-ethereum/internal/finality"
	"github.com/ethereum/go-ethereum/core/types"
)

type API struct { /* accessor to chain */ }

type FinalityStatus struct {
	Final       bool   `json:"final"`
	Round       uint64 `json:"round"`
	Participants int   `json:"participants"`
	Quorum      int    `json:"quorum"`
}

func (api *API) GetFinalityStatus(ctx context.Context, hash common.Hash) (*FinalityStatus, error) {
	h := api.chain.GetHeaderByHash(hash)
	if h == nil { return &FinalityStatus{Final:false}, nil }
	p, ok := fin.Extract(h.Extra)
	if !ok { return &FinalityStatus{Final:false}, nil }
	// derive participants = popcount(bitmap)
	return &FinalityStatus{Final:true, Round:p.Round, Participants: popcount(p.Bitmap), Quorum: /* compute */}, nil
}
