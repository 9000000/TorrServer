package torrstor

import (
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/anacrolix/torrent"

	"server/log"
	"server/settings"
	"server/torr/storage/state"
	"server/torr/utils"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
)

type Cache struct {
	storage.TorrentImpl
	storage *Storage

	capacity int64
	filled   int64
	hash     metainfo.Hash

	pieceLength int64
	pieceCount  int

	pieces   map[int]*Piece
	muPieces sync.RWMutex // BUG-2 fix: protects pieces map

	readers   map[*Reader]struct{}
	muReaders sync.Mutex

	isRemove bool
	isClosed bool
	muRemove sync.Mutex
	torrent  *torrent.Torrent

	// OPT-1 fix: debounce cleanPieces — at most once per interval
	lastCleanTime time.Time
	cleanInterval time.Duration
}

func NewCache(capacity int64, storage *Storage) *Cache {
	ret := &Cache{
		capacity:      capacity,
		filled:        0,
		pieces:        make(map[int]*Piece),
		storage:       storage,
		readers:       make(map[*Reader]struct{}),
		cleanInterval: 1 * time.Second, // OPT-1: debounce interval
	}

	return ret
}

func (c *Cache) Init(info *metainfo.Info, hash metainfo.Hash) {
	log.TLogln("Create cache for:", info.Name, hash.HexString())
	if c.capacity == 0 {
		c.capacity = info.PieceLength * 4
	}

	c.pieceLength = info.PieceLength
	c.pieceCount = info.NumPieces()
	c.hash = hash

	if settings.BTsets.UseDisk {
		name := filepath.Join(settings.BTsets.TorrentsSavePath, hash.HexString())
		err := os.MkdirAll(name, 0o777)
		if err != nil {
			log.TLogln("Error create dir:", err)
		}
	}

	c.muPieces.Lock()
	for i := 0; i < c.pieceCount; i++ {
		c.pieces[i] = NewPiece(i, c)
	}
	c.muPieces.Unlock()
}

func (c *Cache) SetTorrent(torr *torrent.Torrent) {
	c.torrent = torr
}

func (c *Cache) Piece(m metainfo.Piece) storage.PieceImpl {
	c.muPieces.RLock()
	defer c.muPieces.RUnlock()
	if c.pieces == nil {
		return &PieceFake{}
	}
	if val, ok := c.pieces[m.Index()]; ok {
		return val
	}
	return &PieceFake{}
}

func (c *Cache) Close() error {
	if c.torrent != nil {
		log.TLogln("Close cache for:", c.torrent.Name(), c.hash)
	} else {
		log.TLogln("Close cache for:", c.hash)
	}
	c.isClosed = true

	// MINOR-1 fix: lock storage.mu before accessing storage.caches
	c.storage.mu.Lock()
	delete(c.storage.caches, c.hash)
	c.storage.mu.Unlock()

	c.muPieces.Lock()
	if settings.BTsets.RemoveCacheOnDrop {
		name := filepath.Join(settings.BTsets.TorrentsSavePath, c.hash.HexString())
		if name != "" && name != "/" {
			for _, v := range c.pieces {
				if v.dPiece != nil {
					os.Remove(v.dPiece.name)
				}
			}
			os.Remove(name)
		}
	}
	c.pieces = nil
	c.muPieces.Unlock()

	c.muReaders.Lock()
	c.readers = nil
	c.muReaders.Unlock()

	// OPT-6 fix: only FreeOSMem (no forced GC) on close
	utils.FreeOSMem()
	return nil
}

func (c *Cache) removePiece(piece *Piece) {
	if !c.isClosed {
		piece.Release()
	}
}

func (c *Cache) AdjustRA(readahead int64) {
	if settings.BTsets.CacheSize == 0 {
		c.capacity = readahead * 3
	}
	if c.Readers() > 0 {
		c.muReaders.Lock()
		for r := range c.readers {
			r.SetReadahead(readahead)
		}
		c.muReaders.Unlock()
	}
}

func (c *Cache) GetState() *state.CacheState {
	cState := new(state.CacheState)

	piecesState := make(map[int]state.ItemState, 0)
	var fill int64 = 0

	// BUG-2 fix: lock pieces map during iteration
	c.muPieces.RLock()
	if c.pieces != nil && len(c.pieces) > 0 {
		for _, p := range c.pieces {
			pSize := p.GetSize() // BUG-1 fix: atomic read
			if pSize > 0 {
				fill += pSize
				piecesState[p.Id] = state.ItemState{
					Id:        p.Id,
					Size:      pSize,
					Length:    c.pieceLength,
					Completed: p.Complete,
					Priority:  int(c.torrent.PieceState(p.Id).Priority),
				}
			}
		}
	}
	c.muPieces.RUnlock()

	readersState := make([]*state.ReaderState, 0)

	if c.Readers() > 0 {
		c.muReaders.Lock()
		for r := range c.readers {
			rng := r.getPiecesRange()
			pc := r.getReaderPiece()
			readersState = append(readersState, &state.ReaderState{
				Start:  rng.Start,
				End:    rng.End,
				Reader: pc,
			})
		}
		c.muReaders.Unlock()
	}

	c.filled = fill
	cState.Capacity = c.capacity
	cState.PiecesLength = c.pieceLength
	cState.PiecesCount = c.pieceCount
	cState.Hash = c.hash.HexString()
	cState.Filled = fill
	cState.Pieces = piecesState
	cState.Readers = readersState
	return cState
}

func (c *Cache) cleanPieces() {
	if c.isRemove || c.isClosed {
		return
	}
	c.muRemove.Lock()
	if c.isRemove {
		c.muRemove.Unlock()
		return
	}
	// OPT-1 fix: debounce — skip if called too recently
	if time.Since(c.lastCleanTime) < c.cleanInterval {
		c.muRemove.Unlock()
		return
	}
	c.isRemove = true
	c.lastCleanTime = time.Now()
	defer func() {
		c.muRemove.Lock()
		c.isRemove = false
		c.muRemove.Unlock()
	}()
	c.muRemove.Unlock()

	remPieces := c.getRemPieces()
	if c.filled > c.capacity {
		rems := (c.filled-c.capacity)/c.pieceLength + 1
		for _, p := range remPieces {
			c.removePiece(p)
			rems--
			if rems <= 0 {
				// OPT-6 fix: use FreeOSMem (no forced full GC) during streaming
				utils.FreeOSMem()
				return
			}
		}
	}
}

func (c *Cache) getRemPieces() []*Piece {
	piecesRemove := make([]*Piece, 0)
	fill := int64(0)

	ranges := make([]Range, 0)
	c.muReaders.Lock()
	for r := range c.readers {
		r.checkReader()
		if r.isUse {
			ranges = append(ranges, r.getPiecesRange())
		}
	}
	c.muReaders.Unlock()
	ranges = mergeRange(ranges)

	// BUG-2 fix: lock pieces map during iteration
	c.muPieces.RLock()
	if c.pieces == nil {
		c.muPieces.RUnlock()
		return piecesRemove
	}
	for id, p := range c.pieces {
		pSize := p.GetSize() // BUG-1 fix: atomic read
		if pSize > 0 {
			fill += pSize
		}
		if len(ranges) > 0 {
			if !inRanges(ranges, id) {
				if pSize > 0 && !c.isIdInFileBE(ranges, id) {
					piecesRemove = append(piecesRemove, p)
				}
			}
		} else {
			// on preload clean
			if pSize > 0 && !c.isIdInFileBE(ranges, id) {
				piecesRemove = append(piecesRemove, p)
			}
		}
	}
	c.muPieces.RUnlock()

	c.clearPriority()
	c.setLoadPriority(ranges)

	sort.Slice(piecesRemove, func(i, j int) bool {
		return piecesRemove[i].GetAccessed() < piecesRemove[j].GetAccessed() // BUG-1 fix: atomic read
	})

	c.filled = fill
	return piecesRemove
}

func (c *Cache) setLoadPriority(ranges []Range) {
	c.muReaders.Lock()
	for r := range c.readers {
		if !r.isUse {
			continue
		}
		if c.isIdInFileBE(ranges, r.getReaderPiece()) {
			continue
		}
		readerPos := r.getReaderPiece()
		readerRAHPos := r.getReaderRAHPiece()
		end := r.getPiecesRange().End
		readerCount := len(c.readers)
		if readerCount == 0 {
			readerCount = 1
		}
		count := settings.BTsets.ConnectionsLimit / readerCount // max concurrent loading blocks
		limit := 0

		for i := readerPos; i < end && limit < count; i++ {
			c.muPieces.RLock()
			p, ok := c.pieces[i]
			isComplete := false
			if ok {
				isComplete = p.Complete
			}
			c.muPieces.RUnlock()

			if !ok {
				continue
			}
			if !isComplete {
				if i == readerPos {
					c.torrent.Piece(i).SetPriority(torrent.PiecePriorityNow)
				} else if i == readerPos+1 {
					c.torrent.Piece(i).SetPriority(torrent.PiecePriorityNext)
				} else if i > readerPos && i <= readerRAHPos {
					c.torrent.Piece(i).SetPriority(torrent.PiecePriorityReadahead)
				} else if i > readerRAHPos && i <= readerRAHPos+5 && c.torrent.PieceState(i).Priority != torrent.PiecePriorityHigh {
					c.torrent.Piece(i).SetPriority(torrent.PiecePriorityHigh)
				} else if i > readerRAHPos+5 && c.torrent.PieceState(i).Priority != torrent.PiecePriorityNormal {
					c.torrent.Piece(i).SetPriority(torrent.PiecePriorityNormal)
				}
				limit++
			}
		}
	}
	c.muReaders.Unlock()
}

func (c *Cache) isIdInFileBE(ranges []Range, id int) bool {
	// keep 8/16 MB
	FileRangeNotDelete := int64(c.pieceLength)
	if FileRangeNotDelete < 8<<20 {
		FileRangeNotDelete = 8 << 20
	}

	for _, rng := range ranges {
		if rng.File == nil {
			continue
		}
		ss := int(rng.File.Offset() / c.pieceLength)
		se := int((rng.File.Offset() + FileRangeNotDelete) / c.pieceLength)

		es := int((rng.File.Offset() + rng.File.Length() - FileRangeNotDelete) / c.pieceLength)
		ee := int((rng.File.Offset() + rng.File.Length()) / c.pieceLength)

		if id >= ss && id < se || id > es && id <= ee {
			return true
		}
	}
	return false
}

//////////////////
// Reader section
////////

func (c *Cache) NewReader(file *torrent.File) *Reader {
	return newReader(file, c)
}

func (c *Cache) GetUseReaders() int {
	if c == nil {
		return 0
	}
	c.muReaders.Lock()
	defer c.muReaders.Unlock()
	readers := 0
	for reader := range c.readers {
		if reader.isUse {
			readers++
		}
	}
	return readers
}

func (c *Cache) Readers() int {
	if c == nil {
		return 0
	}
	c.muReaders.Lock()
	defer c.muReaders.Unlock()
	if c.readers == nil {
		return 0
	}
	return len(c.readers)
}

func (c *Cache) CloseReader(r *Reader) {
	r.cache.muReaders.Lock()
	r.Close()
	delete(r.cache.readers, r)
	r.cache.muReaders.Unlock()
	go c.clearPriority()
}

func (c *Cache) clearPriority() {
	if c.torrent == nil {
		return
	}
	// OPT-4 fix: removed time.Sleep(time.Second) — the debounce logic
	// in cleanPieces already prevents excessive calls, and sleeping here
	// just delays priority cleanup without providing real benefit.
	ranges := make([]Range, 0)
	c.muReaders.Lock()
	for r := range c.readers {
		r.checkReader()
		if r.isUse {
			ranges = append(ranges, r.getPiecesRange())
		}
	}
	c.muReaders.Unlock()
	ranges = mergeRange(ranges)

	c.muPieces.RLock()
	if c.pieces == nil {
		c.muPieces.RUnlock()
		return
	}
	var keys []int
	for id := range c.pieces {
		keys = append(keys, id)
	}
	c.muPieces.RUnlock()

	for _, id := range keys {
		if len(ranges) > 0 {
			if !inRanges(ranges, id) {
				if c.torrent.PieceState(id).Priority != torrent.PiecePriorityNone {
					c.torrent.Piece(id).SetPriority(torrent.PiecePriorityNone)
				}
			}
		} else {
			if c.torrent.PieceState(id).Priority != torrent.PiecePriorityNone {
				c.torrent.Piece(id).SetPriority(torrent.PiecePriorityNone)
			}
		}
	}
}

func (c *Cache) GetCapacity() int64 {
	if c == nil {
		return 0
	}
	return c.capacity
}
