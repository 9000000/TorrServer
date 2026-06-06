package torrstor

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"server/log"
	"server/settings"
)

type DiskPiece struct {
	piece *Piece

	name string

	mu sync.RWMutex
}

func NewDiskPiece(p *Piece) *DiskPiece {
	name := filepath.Join(settings.BTsets.TorrentsSavePath, p.cache.hash.HexString(), strconv.Itoa(p.Id))
	ff, err := os.Stat(name)
	if err == nil {
		p.SetSize(ff.Size())
		p.Complete = ff.Size() == p.cache.pieceLength
		p.accessed = ff.ModTime().Unix()
	}
	return &DiskPiece{piece: p, name: name}
}

func (p *DiskPiece) WriteAt(b []byte, off int64) (n int, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	ff, err := os.OpenFile(p.name, os.O_RDWR|os.O_CREATE, 0o666)
	if err != nil {
		log.TLogln("Error open file:", err)
		return 0, err
	}
	defer ff.Close()
	n, err = ff.WriteAt(b, off)

	// Use max(current, off+n) semantics to correctly handle re-written chunks
	newSize := off + int64(n)
	if newSize > p.piece.cache.pieceLength {
		newSize = p.piece.cache.pieceLength
	}
	p.piece.SetSize(newSize)
	p.piece.Touch()
	return
}

func (p *DiskPiece) ReadAt(b []byte, off int64) (n int, err error) {
	// BUG-5 fix: Use RLock for read-only operations to allow concurrent reads
	p.mu.RLock()
	defer p.mu.RUnlock()

	ff, err := os.OpenFile(p.name, os.O_RDONLY, 0o666)
	if os.IsNotExist(err) {
		return 0, io.EOF
	}
	if err != nil {
		log.TLogln("Error open file:", err)
		return 0, err
	}
	defer ff.Close()

	n, err = ff.ReadAt(b, off)

	p.piece.Touch()
	if int64(len(b))+off >= p.piece.GetSize() {
		go p.piece.cache.cleanPieces()
	}
	// BUG-4 fix: Return actual I/O errors instead of swallowing them.
	// Only suppress io.EOF from short reads at end of piece, which is normal.
	if err == io.EOF && n > 0 {
		return n, nil
	}
	return n, err
}

func (p *DiskPiece) Release() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.piece.ResetSize()
	p.piece.Complete = false

	os.Remove(p.name)
}

