package torrstor

import (
	"io"
	"sync"
)

type MemPiece struct {
	piece *Piece

	buffer []byte
	mu     sync.RWMutex
}

func NewMemPiece(p *Piece) *MemPiece {
	return &MemPiece{piece: p}
}

func (p *MemPiece) WriteAt(b []byte, off int64) (n int, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.buffer == nil {
		go p.piece.cache.cleanPieces()
		p.buffer = make([]byte, p.piece.cache.pieceLength, p.piece.cache.pieceLength)
	}
	n = copy(p.buffer[off:], b[:])
	// Use max(current, off+n) semantics to correctly handle re-written chunks
	newSize := off + int64(n)
	if newSize > p.piece.cache.pieceLength {
		newSize = p.piece.cache.pieceLength
	}
	p.piece.SetSize(newSize)
	p.piece.Touch()
	return
}

func (p *MemPiece) ReadAt(b []byte, off int64) (n int, err error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	size := len(b)
	if size+int(off) > len(p.buffer) {
		size = len(p.buffer) - int(off)
		if size < 0 {
			size = 0
		}
	}
	if len(p.buffer) < int(off) || len(p.buffer) < int(off)+size {
		return 0, io.EOF
	}
	n = copy(b, p.buffer[int(off) : int(off)+size][:])
	p.piece.Touch()
	if int64(len(b))+off >= p.piece.GetSize() {
		go p.piece.cache.cleanPieces()
	}
	if n == 0 {
		return 0, io.EOF
	}
	return n, nil
}

func (p *MemPiece) Release() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.buffer != nil {
		p.buffer = nil
	}
	p.piece.ResetSize()
	p.piece.Complete = false
}

