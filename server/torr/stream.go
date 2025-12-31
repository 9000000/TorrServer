package torr

import (
	// "context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/anacrolix/dms/dlna"
	"github.com/anacrolix/missinggo/v2/httptoo"
	"github.com/anacrolix/torrent"

	mt "server/mimetype"
	sets "server/settings"
	"server/torr/state"
)

// Add atomic counter for concurrent streams
var activeStreams int32

// AutoPreloadSize is the minimum bytes to preload before streaming (2MB default)
const AutoPreloadSize = 2 * 1024 * 1024

// AutoPreloadTimeout is the maximum time to wait for auto preload
const AutoPreloadTimeout = 15 * time.Second

// type contextResponseWriter struct {
// 	http.ResponseWriter
// 	ctx context.Context
// }

// func (w *contextResponseWriter) Write(p []byte) (n int, err error) {
// 	// Check context before each write
// 	select {
// 	case <-w.ctx.Done():
// 		return 0, w.ctx.Err()
// 	default:
// 		return w.ResponseWriter.Write(p)
// 	}
// }

func (t *Torrent) Stream(fileID int, req *http.Request, resp http.ResponseWriter) error {
	// Increment active streams counter
	streamID := atomic.AddInt32(&activeStreams, 1)
	defer atomic.AddInt32(&activeStreams, -1)
	// Stream disconnect timeout (same as torrent)
	streamTimeout := sets.BTsets.TorrentDisconnectTimeout

	if !t.GotInfo() {
		http.NotFound(resp, req)
		return errors.New("torrent doesn't have info yet")
	}
	// Get file information
	st := t.Status()
	var stFile *state.TorrentFileStat
	for _, fileStat := range st.FileStats {
		if fileStat.Id == fileID {
			stFile = fileStat
			break
		}
	}
	if stFile == nil {
		return fmt.Errorf("file with id %v not found", fileID)
	}
	// Find the actual torrent file
	files := t.Files()
	var file *torrent.File
	for _, tfile := range files {
		if tfile.Path() == stFile.Path {
			file = tfile
			break
		}
	}
	if file == nil {
		return fmt.Errorf("file with id %v not found", fileID)
	}
	// Check file size limit
	if int64(sets.MaxSize) > 0 && file.Length() > int64(sets.MaxSize) {
		err := fmt.Errorf("file size exceeded max allowed %d bytes", sets.MaxSize)
		log.Printf("File %s size (%d) exceeded max allowed %d bytes", file.DisplayPath(), file.Length(), sets.MaxSize)
		http.Error(resp, err.Error(), http.StatusForbidden)
		return err
	}

	// Auto preload before streaming to ensure data is available
	if err := t.autoPreload(file, req); err != nil {
		if sets.BTsets.EnableDebug {
			log.Printf("[Stream] Auto preload warning: %v", err)
		}
		// Continue anyway, don't fail the stream
	}

	// Create reader with context for timeout
	reader := t.NewReader(file)
	if reader == nil {
		return errors.New("cannot create torrent reader")
	}
	// Ensure reader is always closed
	defer t.CloseReader(reader)

	if sets.BTsets.ResponsiveMode {
		reader.SetResponsive()
	}
	// Log connection
	host, port, clerr := net.SplitHostPort(req.RemoteAddr)

	if sets.BTsets.EnableDebug {
		if clerr != nil {
			log.Printf("[Stream:%d] Connect client (Active streams: %d)", streamID, atomic.LoadInt32(&activeStreams))
		} else {
			log.Printf("[Stream:%d] Connect client %s:%s (Active streams: %d)",
				streamID, host, port, atomic.LoadInt32(&activeStreams))
		}
	}

	// Mark as viewed
	sets.SetViewed(&sets.Viewed{
		Hash:      t.Hash().HexString(),
		FileIndex: fileID,
	})

	// Set response headers
	resp.Header().Set("Connection", "close")
	// Add timeout header if configured
	if streamTimeout > 0 {
		resp.Header().Set("X-Stream-Timeout", fmt.Sprintf("%d", streamTimeout))
	}
	// Add ETag
	etag := hex.EncodeToString([]byte(fmt.Sprintf("%s/%s", t.Hash().HexString(), file.Path())))
	resp.Header().Set("ETag", httptoo.EncodeQuotedString(etag))
	// DLNA headers
	resp.Header().Set("transferMode.dlna.org", "Streaming")
	// add MimeType
	mime, err := mt.MimeTypeByPath(file.Path())
	if err == nil && mime.IsMedia() {
		resp.Header().Set("content-type", mime.String())
	}
	// DLNA Seek
	if req.Header.Get("getContentFeatures.dlna.org") != "" {
		resp.Header().Set("contentFeatures.dlna.org", dlna.ContentFeatures{
			SupportRange:    true,
			SupportTimeSeek: true,
		}.String())
	}
	// Add support for range requests
	if req.Header.Get("Range") != "" {
		resp.Header().Set("Accept-Ranges", "bytes")
	}
	// // Create a context with timeout if configured
	// ctx := req.Context()
	// if streamTimeout > 0 {
	// 	var cancel context.CancelFunc
	// 	ctx, cancel = context.WithTimeout(ctx, time.Duration(streamTimeout)*time.Second)
	// 	defer cancel()
	// }
	// // Update request with new context
	// req = req.WithContext(ctx)
	// // Handle client disconnections better
	// wrappedResp := &contextResponseWriter{
	// 	ResponseWriter: resp,
	// 	ctx:            ctx,
	// }
	// http.ServeContent(wrappedResp, req, file.Path(), time.Unix(t.Timestamp, 0), reader)

	http.ServeContent(resp, req, file.Path(), time.Unix(t.Timestamp, 0), reader)

	if sets.BTsets.EnableDebug {
		if clerr != nil {
			log.Printf("[Stream:%d] Disconnect client", streamID)
		} else {
			log.Printf("[Stream:%d] Disconnect client %s:%s", streamID, host, port)
		}
	}
	return nil
}

// GetActiveStreams returns number of currently active streams
func GetActiveStreams() int32 {
	return atomic.LoadInt32(&activeStreams)
}

// autoPreload ensures minimum data is available before streaming
// This prevents the "first request fails" issue by preloading initial pieces
func (t *Torrent) autoPreload(file *torrent.File, req *http.Request) error {
	if t.Stat == state.TorrentClosed {
		return errors.New("torrent closed")
	}

	// Determine start position from Range header
	startPos := int64(0)
	if rangeHeader := req.Header.Get("Range"); rangeHeader != "" {
		var start int64
		if _, err := fmt.Sscanf(rangeHeader, "bytes=%d-", &start); err == nil {
			startPos = start
		}
	}

	// Calculate preload size - use smaller of AutoPreloadSize or remaining file
	preloadSize := int64(AutoPreloadSize)
	remaining := file.Length() - startPos
	if preloadSize > remaining {
		preloadSize = remaining
	}
	if preloadSize <= 0 {
		return nil
	}

	// Check if we already have enough data cached
	if t.cache != nil {
		cached := t.cache.GetState().Filled
		if cached >= preloadSize {
			if sets.BTsets.EnableDebug {
				log.Printf("[AutoPreload] Skipped - already cached %d bytes", cached)
			}
			return nil
		}
	}

	if sets.BTsets.EnableDebug {
		log.Printf("[AutoPreload] Starting preload of %d bytes from position %d", preloadSize, startPos)
	}

	// Create a temporary reader for preloading
	reader := file.NewReader()
	if reader == nil {
		return errors.New("cannot create reader for preload")
	}
	defer reader.Close()

	reader.SetResponsive()
	reader.SetReadahead(preloadSize)

	// Seek to start position if needed
	if startPos > 0 {
		if _, err := reader.Seek(startPos, io.SeekStart); err != nil {
			return fmt.Errorf("seek error: %w", err)
		}
	}

	// Read with timeout
	done := make(chan error, 1)
	go func() {
		buf := make([]byte, 32*1024) // 32KB buffer
		totalRead := int64(0)
		for totalRead < preloadSize {
			n, err := reader.Read(buf)
			if err != nil {
				if err == io.EOF {
					done <- nil
					return
				}
				done <- err
				return
			}
			totalRead += int64(n)

			// Check if torrent was closed
			if t.Stat == state.TorrentClosed {
				done <- errors.New("torrent closed during preload")
				return
			}
		}
		done <- nil
	}()

	// Wait for preload with timeout
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("preload read error: %w", err)
		}
		if sets.BTsets.EnableDebug {
			log.Printf("[AutoPreload] Completed successfully")
		}
		return nil
	case <-time.After(AutoPreloadTimeout):
		if sets.BTsets.EnableDebug {
			log.Printf("[AutoPreload] Timeout after %v, continuing anyway", AutoPreloadTimeout)
		}
		return nil // Don't fail on timeout, just continue
	case <-req.Context().Done():
		return req.Context().Err()
	}
}
