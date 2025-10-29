package hotstuff

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/dgraph-io/badger/v3"
	"github.com/sesafoundation/sesn/common"
)

// ─────────────────────────────────────────────
// Store interface — abstract persistence layer
// ─────────────────────────────────────────────
type Store interface {
	PutQC(*QuorumCert) error
	GetQC(height uint64) (*QuorumCert, error)
	PutVote(height, view uint64, from common.Address, sig []byte) error
	HasVote(height, view uint64, from common.Address) bool
	SetFinalizedHeight(h uint64) error
	GetFinalizedHeight() (uint64, error)
	Close() error
}

// ─────────────────────────────────────────────
// BadgerStore — disk-backed implementation
// ─────────────────────────────────────────────
type BadgerStore struct {
	db *badger.DB
}

// ─────────────────────────────────────────────
// NewBadgerStore creates / opens a Badger DB
// ─────────────────────────────────────────────
func NewBadgerStore(path string) (*BadgerStore, error) {
	opts := badger.DefaultOptions(filepath.Clean(path))
	opts.Logger = nil // silence default logs
	opts.SyncWrites = true
	opts.DetectConflicts = false
	opts.CompactL0OnClose = true

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("open badger store: %w", err)
	}
	return &BadgerStore{db: db}, nil
}

// ─────────────────────────────────────────────
// Helper: generate keys
// ─────────────────────────────────────────────
func keyQC(height uint64) []byte       { return []byte(fmt.Sprintf("qc:%020d", height)) }
func keyVote(height, view uint64, from common.Address) []byte {
	return []byte(fmt.Sprintf("vote:%020d:%020d:%s", height, view, from.Hex()))
}
var keyFinalized = []byte("finalizedHeight")

// ─────────────────────────────────────────────
// PutQC stores a quorum certificate
// ─────────────────────────────────────────────
func (b *BadgerStore) PutQC(qc *QuorumCert) error {
	data, err := json.Marshal(qc)
	if err != nil {
		return fmt.Errorf("marshal qc: %w", err)
	}
	return b.db.Update(func(txn *badger.Txn) error {
		return txn.Set(keyQC(qc.Height), data)
	})
}

// GetQC retrieves the quorum certificate for given height.
func (b *BadgerStore) GetQC(height uint64) (*QuorumCert, error) {
	var qc QuorumCert
	err := b.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(keyQC(height))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &qc)
		})
	})
	if err == badger.ErrKeyNotFound {
		return nil, nil
	}
	return &qc, err
}

// ─────────────────────────────────────────────
// Vote storage — prevents duplicates
// ─────────────────────────────────────────────
func (b *BadgerStore) PutVote(height, view uint64, from common.Address, sig []byte) error {
	k := keyVote(height, view, from)
	return b.db.Update(func(txn *badger.Txn) error {
		_, err := txn.Get(k)
		if err == nil {
			return nil // already exists
		}
		if err != badger.ErrKeyNotFound {
			return err
		}
		return txn.Set(k, sig)
	})
}

func (b *BadgerStore) HasVote(height, view uint64, from common.Address) bool {
	k := keyVote(height, view, from)
	err := b.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get(k)
		return err
	})
	return err == nil
}

// ─────────────────────────────────────────────
// Finalized height
// ─────────────────────────────────────────────
func (b *BadgerStore) SetFinalizedHeight(h uint64) error {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, h)
	return b.db.Update(func(txn *badger.Txn) error {
		return txn.Set(keyFinalized, buf)
	})
}

func (b *BadgerStore) GetFinalizedHeight() (uint64, error) {
	var h uint64
	err := b.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(keyFinalized)
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			h = binary.BigEndian.Uint64(val)
			return nil
		})
	})
	if err == badger.ErrKeyNotFound {
		return 0, nil
	}
	return h, err
}

// ─────────────────────────────────────────────
// Close store
// ─────────────────────────────────────────────
func (b *BadgerStore) Close() error {
	if b.db == nil {
		return nil
	}
	defer time.Sleep(100 * time.Millisecond)
	return b.db.Close()
}
