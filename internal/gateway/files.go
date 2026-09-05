package gateway

import (
	"context"
	"errors"
	"routerprobe/internal/filetransfer"
	"routerprobe/internal/protocol"
	"routerprobe/internal/task"
	"sync"
	"time"
)

// Priority is chosen before acquiring the stream lock. Pending controls always
// win over the next chunk; an already started frame remains indivisible.
type priorityGate struct {
	mu       sync.Mutex
	cond     *sync.Cond
	busy     bool
	controls int
}

func (g *priorityGate) lock(low bool) {
	g.mu.Lock()
	if g.cond == nil {
		g.cond = sync.NewCond(&g.mu)
	}
	if !low {
		g.controls++
	}
	for g.busy || low && g.controls > 0 {
		g.cond.Wait()
	}
	if !low {
		g.controls--
	}
	g.busy = true
	g.mu.Unlock()
}
func (g *priorityGate) unlock() { g.mu.Lock(); g.busy = false; g.cond.Broadcast(); g.mu.Unlock() }
func (w *connectionWriter) sendChunk(b []byte) error {
	if len(b) < 29 || len(b) > 28+filetransfer.MaxChunk {
		return errors.New("invalid chunk length")
	}
	w.priority.lock(true)
	defer w.priority.unlock()
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.failed != nil {
		return w.failed
	}
	id := w.nextOutgoingID
	if id == 0 || id == ^uint64(0) {
		return w.failLocked(errors.New("message_id exhausted"))
	}
	if e := w.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); e != nil {
		return w.failLocked(e)
	}
	if e := protocol.WriteFrame(w.conn, protocol.Frame{Header: protocol.Header{Version: protocol.Version1, Type: protocol.TypeFileChunk, Flags: protocol.FlagBinary, MessageID: id}, Payload: b}); e != nil {
		return w.failLocked(e)
	}
	w.nextOutgoingID++
	return nil
}
func (s *Server) fileTransport(a *session) filetransfer.Transport {
	return filetransfer.Transport{SessionID: a.sessionID, ChunkSize: s.config.FileChunkSize, Done: a.done, Abort: func() { a.transport.conn.Close() }, SendChunk: a.transport.sendChunk, Send: func(typ uint8, flags uint16, v interface{}) (uint64, error) {
		id, err := a.transport.sendJSON(typ, flags, v, nil)
		var transfer string
		switch m := v.(type) {
		case filetransfer.Begin:
			transfer = m.TransferID
		case filetransfer.End:
			transfer = m.TransferID
		case filetransfer.Ack:
			transfer = m.TransferID
		}
		if err == nil {
			s.config.Logger.Printf("sent=FILE type=%02x message_id=%d session_id=%s transfer_id=%s", typ, id, a.sessionID, transfer)
		}
		return id, err
	}}
}

var ErrSessionChanged = errors.New("device session changed")

func (s *Server) CreateUpload(ctx context.Context, device string, q filetransfer.UploadRequest) (string, error) {
	return s.createFile(ctx, device, q.ExpectedSessionID, func(a *session) (task.Spec, error) { return s.files.Upload(ctx, device, q, s.fileTransport(a)) })
}
func (s *Server) CreateDownload(ctx context.Context, device string, q filetransfer.DownloadRequest) (string, error) {
	return s.createFile(ctx, device, q.ExpectedSessionID, func(a *session) (task.Spec, error) { return s.files.Download(ctx, device, q, s.fileTransport(a)) })
}
func (s *Server) createFile(ctx context.Context, device, expectedSession string, prepare func(*session) (task.Spec, error)) (string, error) {
	if e := ctx.Err(); e != nil {
		return "", e
	}
	s.mu.Lock()
	a := s.sessions[device]
	s.mu.Unlock()
	if a == nil {
		return "", errors.New("device offline")
	}
	if expectedSession != "" && a.sessionID != expectedSession {
		return "", ErrSessionChanged
	}
	spec, e := prepare(a)
	if e != nil {
		return "", e
	}
	if e = ctx.Err(); e != nil {
		s.files.Remove(spec.ID)
		s.tasks.Remove(spec.ID)
		return "", e
	}
	id, e := s.dispatchChecked(a, spec, expectedSession != "")
	if e != nil && id == 0 {
		s.files.Remove(spec.ID)
		s.tasks.Remove(spec.ID)
		return "", e
	}
	return spec.ID, e
}
func (s *Server) FileSnapshot(id string) (filetransfer.Snapshot, error) { return s.files.Snapshot(id) }
