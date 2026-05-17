package torrstor

import (
	"sync/atomic"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/storage"
	"server/settings"
)

type Piece struct {
	storage.PieceImpl `json:"-"`

	Id int `json:"-"`
	// size is accessed atomically to prevent data races between
	// concurrent readers (GetState, getRemPieces) and writers (WriteAt).
	size int64

	Complete bool `json:"complete"`
	// accessed is accessed atomically — written by ReadAt/WriteAt,
	// read by getRemPieces sort comparator.
	accessed int64

	mPiece *MemPiece  `json:"-"`
	dPiece *DiskPiece `json:"-"`

	cache *Cache `json:"-"`
}

// GetSize returns the current piece size atomically.
func (p *Piece) GetSize() int64 {
	return atomic.LoadInt64(&p.size)
}

// SetSize sets piece size using max(current, newSize) semantics.
// This correctly handles re-written chunks without double-counting.
func (p *Piece) SetSize(newSize int64) {
	for {
		old := atomic.LoadInt64(&p.size)
		if newSize <= old {
			return
		}
		if atomic.CompareAndSwapInt64(&p.size, old, newSize) {
			return
		}
	}
}

// ResetSize sets piece size to zero (used during Release).
func (p *Piece) ResetSize() {
	atomic.StoreInt64(&p.size, 0)
}

// GetAccessed returns the last access timestamp atomically.
func (p *Piece) GetAccessed() int64 {
	return atomic.LoadInt64(&p.accessed)
}

// Touch updates the last access timestamp to now.
func (p *Piece) Touch() {
	atomic.StoreInt64(&p.accessed, time.Now().Unix())
}

func NewPiece(id int, cache *Cache) *Piece {
	p := &Piece{
		Id:    id,
		cache: cache,
	}

	if !settings.BTsets.UseDisk {
		p.mPiece = NewMemPiece(p)
	} else {
		p.dPiece = NewDiskPiece(p)
	}
	return p
}

func (p *Piece) WriteAt(b []byte, off int64) (n int, err error) {
	if !settings.BTsets.UseDisk {
		return p.mPiece.WriteAt(b, off)
	} else {
		return p.dPiece.WriteAt(b, off)
	}
}

func (p *Piece) ReadAt(b []byte, off int64) (n int, err error) {
	if !settings.BTsets.UseDisk {
		return p.mPiece.ReadAt(b, off)
	} else {
		return p.dPiece.ReadAt(b, off)
	}
}

func (p *Piece) MarkComplete() error {
	p.Complete = true
	return nil
}

func (p *Piece) MarkNotComplete() error {
	p.Complete = false
	return nil
}

func (p *Piece) Completion() storage.Completion {
	return storage.Completion{
		Complete: p.Complete,
		Ok:       true,
	}
}

func (p *Piece) Release() {
	if !settings.BTsets.UseDisk {
		p.mPiece.Release()
	} else {
		p.dPiece.Release()
	}
	if !p.cache.isClosed {
		p.cache.torrent.Piece(p.Id).SetPriority(torrent.PiecePriorityNone)
		p.cache.torrent.Piece(p.Id).UpdateCompletion()
	}
}
