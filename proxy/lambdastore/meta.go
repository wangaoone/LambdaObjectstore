package lambdastore

import (
	"sync/atomic"

	protocol "github.com/mason-leap-lab/infinicache/common/types"
)

// type ChuckMeta {
// 	Key string
// 	Size uint64
// 	Hit bool
// 	Accessed time.Time
//
// 	prev *ChunkMeta
// 	next *ChunkMeta
// }

// FULL = (Updates - SnapshotUpdates + SnapshotSize) / Bandwidth + (Term - SnapShotTerm + 1) * RTT
// INCREMENTAL = (Updates - LastUpdates) / Bandwidth + (Seq - LastSeq) * RTT
// FULL < INCREMENTAL
type Meta struct {
	// Sequence of the last confirmed log. Logs store by sequence.
	Term     uint64

	// Total transmission size for restoring all confirmed logs.
	Updates  uint64

	// Rank for lambda to decide if a fast recovery is required.
	DiffRank float64

	// Hash of the last confirmed log.
	Hash     string

	// Sequence of snapshot.
	SnapshotTerm    uint64

	// Total transmission size for restoring all confirmed logs from start to SnapShotSeq.
	SnapshotUpdates uint64

	// Total size of snapshot for transmission.
	SnapshotSize    uint64

	// Flag shows that if meta is out of sync with the corresponding lambda.
	Stale           bool

	// Capacity of the instance.
	Capacity        uint64

	size            uint64             // Size of the instance.
	// chunks map[string]*ChuckMeta
	// head ChuckMeta
	// anchor *ChuckMeta
}

func (m *Meta) Size() uint64 {
	return atomic.LoadUint64(&m.size)
}

func (m *Meta) IncreaseSize(inc int64) uint64 {
	return atomic.AddUint64(&m.size, uint64(inc))
}

func (m *Meta) DecreaseSize(dec int64) uint64 {
	return atomic.AddUint64(&m.size, ^uint64(dec-1))
}

func (m *Meta) ToProtocolMeta(id uint64) *protocol.Meta {
	return &protocol.Meta {
		Id:              id,
		Term:            m.Term,
		Updates:         m.Updates,
		DiffRank:        m.DiffRank,
		Hash:            m.Hash,
		SnapshotTerm:    m.SnapshotTerm,
		SnapshotUpdates: m.SnapshotUpdates,
		SnapshotSize:    m.SnapshotSize,
	}
}

func (m *Meta) FromProtocolMeta(meta *protocol.Meta) bool {
	if meta.Term < m.Term {
		return false
	}

	m.Term = meta.Term
	m.Updates = meta.Updates
	m.DiffRank = meta.DiffRank
	m.Hash = meta.Hash
	m.SnapshotTerm = meta.SnapshotTerm
	m.SnapshotUpdates = meta.SnapshotUpdates
	m.SnapshotSize = meta.SnapshotSize
	return true
}
