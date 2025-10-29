package hotstuff

import (
	"encoding/binary"
	"fmt"
	"path/filepath"

	"github.com/dgraph-io/badger/v3"
	"github.com/sesafoundation/sesn/common"
	"github.com/sesafoundation/sesn/consensus/sonium/finality/hotstuff/wire"
	"google.golang.org/protobuf/proto"
)

type BadgerStore struct {
	db *badger.DB
}

func NewBadgerStore(path string) *BadgerStore {
	opts := badger.DefaultOptions(filepath.Clean(path))
	opts.Logger = nil // silence
	db, err := badger.Open(opts)
	if err != nil {
		panic(fmt.Sprintf("open badger: %v", err))
	}
	return &BadgerStore{db: db}
}

func (s *BadgerStore) Close() error { return s.db.Close() }

// ---------------- finalized height ----------------

func (s *BadgerStore) SetFinalizedHeight(h uint64) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte("fh:"), u64(h))
	})
}

func (s *BadgerStore) GetFinalizedHeight() (uint64, error) {
	var out uint64
	err := s.db.View(func(txn *badger.Txn) error {
		itm, err := txn.Get([]byte("fh:"))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				out = 0
				return nil
			}
			return err
		}
		return itm.Value(func(v []byte) error {
			out = bu64(v)
			return nil
		})
	})
	return out, err
}

// ---------------- quorum certs ----------------

func qcKey(h uint64) []byte {
	return append([]byte("qc:"), u64(h)...)
}

func (s *BadgerStore) PutQC(qc *QuorumCert) error {
	// Convert your QuorumCert (engine type) to wire.QuorumCert
	wqc := &wire.QuorumCert{
		ChainId: qc.ChainID,
		Height:  qc.Height,
		View:    qc.View,
		BlockId: qc.BlockID[:],
		AggSig:  qc.AggSig,
		Bitmap:  qc.Bitmap,
		Quorum:  uint32(qc.Quorum),
	}
	b, err := proto.Marshal(wqc)
	if err != nil {
		return err
	}
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(qcKey(qc.Height), b)
	})
}

func (s *BadgerStore) GetQC(height uint64) (*QuorumCert, error) {
	var out *QuorumCert
	err := s.db.View(func(txn *badger.Txn) error {
		itm, err := txn.Get(qcKey(height))
		if err != nil {
			return err
		}
		return itm.Value(func(v []byte) error {
			w := new(wire.QuorumCert)
			if err := proto.Unmarshal(v, w); err != nil {
				return err
			}
			if len(w.BlockId) != 32 {
				return fmt.Errorf("bad qc block_id")
			}
			var bid common.Hash
			copy(bid[:], w.BlockId)
			out = &QuorumCert{
				ChainID: w.ChainId,
				Height:  w.Height,
				View:    w.View,
				BlockID: bid,
				AggSig:  w.AggSig,
				Bitmap:  w.Bitmap,
				Quorum:  int(w.Quorum),
			}
			return nil
		})
	})
	return out, err
}

// ---------------- votes ----------------

func voteKey(h, v uint64, addr common.Address) []byte {
	k := append([]byte("vote:"), u64(h)...)
	k = append(k, ':')
	k = append(k, u64(v)...)
	k = append(k, ':')
	k = append(k, addr.Bytes()...)
	return k
}

func (s *BadgerStore) PutVote(height, view uint64, from common.Address, sig []byte) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(voteKey(height, view, from), sig)
	})
}

func (s *BadgerStore) HasVote(height, view uint64, from common.Address) bool {
	err := s.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get(voteKey(height, view, from))
		return err
	})
	return err == nil
}

// ---------------- helpers ----------------

func u64(v uint64) []byte {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], v)
	return b[:]
}

func bu64(b []byte) uint64 {
	if len(b) < 8 {
		return 0
	}
	return binary.BigEndian.Uint64(b[:8])
}
