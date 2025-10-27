package hotstuff

import (
	"sync"
	"time"

	"github.com/sesafoundation/sesn/common"
)

type key struct {
	Hash  common.Hash
	Round uint64
}

type VoteMsg struct {
	BlockHash  common.Hash
	Round      uint64
	VoterIndex uint16
	Sig        []byte
}

type CommitCert struct {
	Round  uint64
	AggSig []byte
	Bitmap []byte
	Count  int
}

// partial aggregate kept per (hash, round)
type partialAgg struct {
	Sigs   [][]byte
	Bitmap []byte // simple byte-array bitmap; compress later
	Count  int
}

type QCPool struct {
	mu   sync.Mutex
	vmap map[key]map[uint16][]byte // raw votes
	pmap map[key]*partialAgg       // rolling partial aggregate
}

func NewQCPool() *QCPool {
	return &QCPool{
		vmap: make(map[key]map[uint16][]byte),
		pmap: make(map[key]*partialAgg),
	}
}

func (q *QCPool) AddVote(v *VoteMsg) {
	q.mu.Lock(); defer q.mu.Unlock()
	k := key{v.BlockHash, v.Round}
	if _, ok := q.vmap[k]; !ok { q.vmap[k] = map[uint16][]byte{} }
	if _, dup := q.vmap[k][v.VoterIndex]; dup { return }
	q.vmap[k][v.VoterIndex] = v.Sig

	pa := q.pmap[k]
	if pa == nil {
		pa = &partialAgg{Sigs: make([][]byte, 0, 8), Bitmap: make([]byte, 1)}
		q.pmap[k] = pa
	}
	pa.Sigs = append(pa.Sigs, v.Sig)
	i := v.VoterIndex / 8; b := v.VoterIndex % 8
	if int(i) >= len(pa.Bitmap) { tmp := make([]byte, int(i)+1); copy(tmp, pa.Bitmap); pa.Bitmap = tmp }
	pa.Bitmap[i] |= (1 << b)
	pa.Count++
}

func (q *QCPool) Snapshot(k key) ([][]byte, []byte, int) {
	q.mu.Lock(); defer q.mu.Unlock()
	if pa := q.pmap[k]; pa != nil {
		return append([][]byte(nil), pa.Sigs...), append([]byte(nil), pa.Bitmap...), pa.Count
	}
	return nil, nil, 0
}

func (q *QCPool) WaitForQuorum(h common.Hash, round uint64, quorum int, max time.Duration, bls *BLS) (*CommitCert, bool) {
	k := key{h, round}
	dead := time.NewTimer(max); defer dead.Stop()
	tk := time.NewTicker(10 * time.Millisecond); defer tk.Stop()
	for {
		select {
		case <-dead.C:
			return nil, false
		case <-tk.C:
			sigs, bm, cnt := q.Snapshot(k)
			if cnt >= quorum {
				return &CommitCert{Round: round, AggSig: bls.Aggregate(sigs), Bitmap: bm, Count: cnt}, true
			}
		}
	}
}
