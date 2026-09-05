package filetransfer

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"routerprobe/internal/protocol"
	"routerprobe/internal/task"
	"sync"
	"time"
)

type UploadRequest struct {
	SourcePath, RemotePath, Mode string
	Overwrite                    bool
	Timeout                      time.Duration
}
type DownloadRequest struct {
	RemotePath, ResultName, TargetPath string
	Overwrite                          bool
	Timeout                            time.Duration
}
type Transport struct {
	SessionID string
	ChunkSize uint32
	Done      <-chan struct{}
	Send      func(uint8, uint16, interface{}) (uint64, error)
	SendChunk func([]byte) error
	Abort     func()
}
type Snapshot struct {
	TaskID, TransferID, LocalPath string
	Size                          int64
	SHA256                        string
	Committed                     bool
	Error                         string
}
type event struct {
	frame protocol.Frame
	ack   *task.Ack
}
type record struct {
	ending      bool   // guarded by Service.mu; terminal frame routed, worker may still be draining
	lastMessage uint64 // file worker only; used for direct ERROR correlation
	spec        task.Spec
	p           Params
	t           Transport
	source      *os.File
	target      string
	overwrite   bool
	events      chan event
	eventMu     sync.Mutex // protects detaching the receive mailbox when the worker exits
	cancel      chan struct{}
	once        sync.Once
	snapshot    Snapshot
}
type Service struct {
	closed    bool
	active    map[string]string // session -> active transfer; guarded by mu
	mu        sync.Mutex
	records   map[string]*record
	transfers map[string]*record
	tasks     *task.Service
	wg        sync.WaitGroup
}

func New(tasks *task.Service) *Service {
	return &Service{records: map[string]*record{}, transfers: map[string]*record{}, active: map[string]string{}, tasks: tasks}
}
func newUUID() (string, error) {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	b[6] = b[6]&15 | 64
	b[8] = b[8]&63 | 128
	h := hex.EncodeToString(b)
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:], nil
}
func (s *Service) Upload(ctx context.Context, device string, q UploadRequest, t Transport) (task.Spec, error) {
	f, n, h, e := OpenSource(ctx, q.SourcePath)
	if e != nil {
		return task.Spec{}, e
	}
	id, e := newUUID()
	if e != nil {
		f.Close()
		return task.Spec{}, e
	}
	p := Params{TransferID: id, RemotePath: q.RemotePath, Size: n, SHA256: h, Mode: q.Mode, Overwrite: q.Overwrite}
	if e = p.Validate("upload"); e != nil {
		f.Close()
		return task.Spec{}, e
	}
	if !Name(path.Base(q.RemotePath)) {
		f.Close()
		return task.Spec{}, errors.New("invalid remote file name")
	}
	spec, e := s.add(device, "upload", q.Timeout, p, t, f, q.SourcePath, q.Overwrite)
	if e != nil {
		f.Close()
	}
	return spec, e
}
func (s *Service) Download(ctx context.Context, device string, q DownloadRequest, t Transport) (task.Spec, error) {
	if e := ctx.Err(); e != nil {
		return task.Spec{}, e
	}
	id, e := newUUID()
	if e != nil {
		return task.Spec{}, e
	}
	p := Params{TransferID: id, RemotePath: q.RemotePath, ResultName: q.ResultName}
	if e = p.Validate("download"); e != nil {
		return task.Spec{}, e
	}
	return s.add(device, "download", q.Timeout, p, t, nil, q.TargetPath, q.Overwrite)
}
func (s *Service) add(device, kind string, timeout time.Duration, p Params, t Transport, f *os.File, local string, overwrite bool) (task.Spec, error) {
	b, _ := json.Marshal(p.Wire(kind))
	spec, e := s.tasks.NewFile(device, kind, timeout, b)
	if e != nil {
		return spec, e
	}
	r := &record{spec: spec, p: p, t: t, source: f, target: local, overwrite: overwrite, events: make(chan event, 16), cancel: make(chan struct{}), snapshot: Snapshot{TaskID: spec.ID, TransferID: p.TransferID, LocalPath: local}}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		s.tasks.Remove(spec.ID)
		return task.Spec{}, errors.New("file service closed")
	}
	s.records[spec.ID] = r
	s.transfers[p.TransferID] = r
	s.wg.Add(1)
	s.mu.Unlock()
	go func() {
		defer s.wg.Done()
		defer func() {
			if r.source != nil {
				r.source.Close()
			}
		}()
		if e := s.run(r); e != nil {
			s.mu.Lock()
			r.snapshot.Error = e.Error()
			s.mu.Unlock()
			select {
			case <-r.t.Done:
			default:
				failure := map[string]interface{}{"code": "TRANSFER_ERROR", "message": "file transfer failed"}
				flags := uint16(0)
				if r.lastMessage != 0 {
					failure["reply_to"] = r.lastMessage
					flags = protocol.FlagResponse
				}
				_, _ = r.t.Send(protocol.TypeError, flags, failure)
			}
			r.t.Abort()
		}
	}()
	return spec, nil
}
func (s *Service) Remove(id string) {
	s.mu.Lock()
	r := s.records[id]
	if r != nil {
		delete(s.records, id)
		delete(s.transfers, r.p.TransferID)
		r.once.Do(func() { close(r.cancel) })
	}
	s.mu.Unlock()
}
func (s *Service) Close() {
	s.mu.Lock()
	s.closed = true
	for _, r := range s.records {
		r.once.Do(func() { close(r.cancel) })
	}
	s.mu.Unlock()
	s.wg.Wait()
}
func (s *Service) Snapshot(id string) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.records[id]
	if r == nil {
		return Snapshot{}, task.ErrTaskNotFound
	}
	return r.snapshot, nil
}

func (s *Service) ValidateResult(result task.Result) error {
	s.mu.Lock()
	r := s.records[result.TaskID]
	if r == nil {
		s.mu.Unlock()
		return nil
	}
	p := r.p
	kind := r.spec.Type
	snap := r.snapshot
	s.mu.Unlock()
	var details struct {
		TransferID string `json:"transfer_id"`
		Size       int64  `json:"size"`
		SHA256     string `json:"sha256"`
	}
	fields := []string{"transfer_id"}
	if result.Status == "success" {
		fields = append(fields, "size", "sha256")
	}
	if err := Decode(result.Details, &details, fields...); err != nil {
		return err
	}
	if details.TransferID != p.TransferID || result.Stdout != "" || result.Truncated {
		return errors.New("invalid file result")
	}
	if result.Status != "success" {
		if result.ExitCode != -1 {
			return errors.New("invalid failed file exit_code")
		}
		return nil
	}
	if result.ExitCode != 0 {
		return errors.New("invalid successful file exit_code")
	}
	if kind == "upload" {
		if details.Size != p.Size || details.SHA256 != p.SHA256 {
			return errors.New("upload result metadata mismatch")
		}
	} else if !snap.Committed || details.Size != snap.Size || details.SHA256 != snap.SHA256 {
		return errors.New("download result without matching local commit")
	}
	return nil
}
func (s *Service) OnAck(sessionID string, a task.Ack) error {
	s.mu.Lock()
	r := s.records[a.TaskID]
	s.mu.Unlock()
	if r == nil || r.t.SessionID != sessionID {
		return nil
	}
	return r.enqueue(event{ack: &a})
}
func (r *record) enqueue(e event) error {
	r.eventMu.Lock()
	events := r.events
	r.eventMu.Unlock()
	if events == nil {
		return nil
	}
	select {
	case <-r.t.Done:
		return nil
	case <-r.cancel:
		return nil
	case events <- e:
		return nil
	}
}
func (s *Service) Route(sessionID string, f protocol.Frame) error {
	var id string
	if f.Header.Type == protocol.TypeFileChunk {
		if len(f.Payload) < 28 {
			return errors.New("short FILE_CHUNK")
		}
		h := hex.EncodeToString(f.Payload[:16])
		id = h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
	} else {
		var v struct {
			TransferID string `json:"transfer_id"`
		}
		if e := Decode(f.Payload, &v, "transfer_id"); e != nil {
			return e
		}
		id = v.TransferID
	}
	flags := uint16(0)
	if f.Header.Type == protocol.TypeFileChunk {
		flags = protocol.FlagBinary
	} else if f.Header.Type == protocol.TypeFileAck {
		flags = protocol.FlagResponse
	}
	if f.Header.Flags != flags {
		return errors.New("invalid FILE flags")
	}
	s.mu.Lock()
	r := s.transfers[id]
	if r != nil && r.t.SessionID == sessionID {
		if f.Header.Type == protocol.TypeFileEnd {
			r.ending = true
		}
		if f.Header.Type == protocol.TypeFileAck {
			if a, e := ParseAck(f.Payload); e == nil && (a.Status == "done" || a.Status == "failed") {
				r.ending = true
			}
		}
	}
	s.mu.Unlock()
	if r == nil || r.t.SessionID != sessionID {
		return errors.New("unknown transfer/session")
	}
	select {
	case <-r.cancel:
		return errors.New("FILE for ended transfer")
	default:
	}
	return r.enqueue(event{frame: f})
}
func (r *record) next() (event, error) {
	select {
	case <-r.t.Done:
		return event{}, io.EOF
	case <-r.cancel:
		return event{}, io.EOF
	case e := <-r.events:
		return e, nil
	}
}
func (r *record) fileFrame() (protocol.Frame, error) {
	for {
		e, err := r.next()
		if err != nil {
			return protocol.Frame{}, err
		}
		if e.ack != nil {
			continue
		}
		r.lastMessage = e.frame.Header.MessageID
		return e.frame, nil
	}
}
func (r *record) ack(reply uint64, phase string, n int64) (bool, error) {
	f, e := r.fileFrame()
	if e != nil {
		return false, e
	}
	if f.Header.Type != protocol.TypeFileAck {
		return false, errors.New("expected FILE_ACK")
	}
	a, e := ParseAck(f.Payload)
	if e != nil {
		return false, e
	}
	if a.ReplyTo != reply || a.TransferID != r.p.TransferID || (a.Status != phase && a.Status != "failed") || (a.Status == "done" && a.Received != n) {
		return false, errors.New("FILE_ACK correlation mismatch")
	}
	return a.Status == phase, nil
}
func (s *Service) run(r *record) error {
	// ACKs for retries never start a second transfer. A closed record remains in
	// the identity table; later ACKs are discarded by cancel, FILE frames rejected.
	defer func() {
		r.once.Do(func() { close(r.cancel) })
		// Retain identity/snapshot, not queued file bytes. Concurrent enqueuers
		// may hold the old mailbox briefly but are released by cancel.
		r.eventMu.Lock()
		r.events = nil
		r.eventMu.Unlock()
	}()
	defer func() {
		s.mu.Lock()
		if s.active[r.t.SessionID] == r.p.TransferID {
			delete(s.active, r.t.SessionID)
		}
		s.mu.Unlock()
	}()
	for {
		e, err := r.next()
		if err != nil {
			return nil
		}
		if e.ack == nil {
			return errors.New("FILE before TASK_ACK")
		}
		if !e.ack.Accepted {
			return nil
		}
		if e.ack.State == "success" || e.ack.State == "failed" || e.ack.State == "timeout" {
			return nil
		}
		break
	}
	if r.spec.Type == "upload" {
		return s.send(r)
	}
	return s.receive(r)
}
func (s *Service) claim(r *record) error {
	for {
		s.mu.Lock()
		id := s.active[r.t.SessionID]
		if id == "" || id == r.p.TransferID {
			s.active[r.t.SessionID] = r.p.TransferID
			s.mu.Unlock()
			return nil
		}
		previous := s.transfers[id]
		if previous == nil || !previous.ending {
			s.mu.Unlock()
			return errors.New("parallel file stream")
		}
		done := previous.cancel
		s.mu.Unlock()
		// The reader can route the next BEGIN before the prior worker processes
		// its already received terminal frame. Preserve that wire causal order.
		select {
		case <-done:
		case <-r.t.Done:
			return io.EOF
		case <-r.cancel:
			return io.EOF
		}
	}
}
func (s *Service) send(r *record) error {
	b := Begin{TransferID: r.p.TransferID, TaskID: r.spec.ID, Direction: "server_to_device", Name: path.Base(r.p.RemotePath), RemotePath: r.p.RemotePath, Size: r.p.Size, SHA256: r.p.SHA256, ChunkSize: r.t.ChunkSize, Mode: &r.p.Mode, Overwrite: &r.p.Overwrite}
	id, e := r.t.Send(protocol.TypeFileBegin, 0, b)
	if e != nil {
		return e
	}
	ok, e := r.ack(id, "ready", 0)
	if e != nil || !ok {
		return e
	}
	if e = s.claim(r); e != nil {
		return e
	}
	r.lastMessage = 0
	buf := make([]byte, b.ChunkSize)
	h := sha256.New()
	var off int64
	for {
		select {
		case <-r.t.Done:
			return io.EOF
		default:
		}
		n, err := r.source.Read(buf)
		if n > 0 {
			if int64(n) > b.Size-off {
				return errors.New("source size changed")
			}
			h.Write(buf[:n])
			chunk, _ := Chunk(b.TransferID, uint64(off), buf[:n])
			if e = r.t.SendChunk(chunk); e != nil {
				return e
			}
			off += int64(n)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	if off != b.Size || hex.EncodeToString(h.Sum(nil)) != b.SHA256 {
		return errors.New("source size/SHA-256 changed")
	}
	id, e = r.t.Send(protocol.TypeFileEnd, 0, End{b.TransferID, b.Size, b.SHA256})
	if e != nil {
		return e
	}
	_, e = r.ack(id, "done", b.Size)
	return e
}
func (s *Service) receive(r *record) error {
	f, e := r.fileFrame()
	if e != nil {
		return nil
	}
	if f.Header.Type != protocol.TypeFileBegin {
		return errors.New("expected FILE_BEGIN")
	}
	b, e := ParseBegin(f.Payload, r.t.ChunkSize)
	if e != nil {
		return e
	}
	if b.TaskID != r.spec.ID || b.TransferID != r.p.TransferID || b.Direction != "device_to_server" || b.RemotePath != r.p.RemotePath || b.Name != r.p.ResultName {
		return errors.New("FILE_BEGIN task mismatch")
	}
	if e = s.claim(r); e != nil {
		return e
	}
	dst, e := NewReceiver(r.target, r.overwrite)
	if e != nil {
		_, err := r.t.Send(protocol.TypeFileAck, protocol.FlagResponse, NewAck(f.Header.MessageID, b.TransferID, "failed", 0))
		return err
	}
	defer dst.Close()
	if _, e = r.t.Send(protocol.TypeFileAck, protocol.FlagResponse, NewAck(f.Header.MessageID, b.TransferID, "ready", 0)); e != nil {
		return e
	}
	for {
		f, e = r.fileFrame()
		if e != nil {
			return nil
		}
		switch f.Header.Type {
		case protocol.TypeFileChunk:
			data, err := ParseChunk(f.Payload, b.TransferID, dst.Received, b.Size, b.ChunkSize)
			if err != nil {
				return err
			}
			if err = dst.Write(data); err != nil {
				return err
			}
		case protocol.TypeFileEnd:
			var end End
			if e = Decode(f.Payload, &end, "transfer_id", "size", "sha256"); e != nil {
				return e
			}
			status := "done"
			if end.TransferID != b.TransferID || end.Size != b.Size || end.SHA256 != b.SHA256 {
				status = "failed"
			} else if e = dst.Commit(b.Size, b.SHA256); e != nil {
				status = "failed"
			}
			s.mu.Lock()
			r.snapshot.Committed = dst.Committed
			if dst.Committed {
				r.snapshot.Size = b.Size
				r.snapshot.SHA256 = b.SHA256
			}
			if status == "failed" {
				r.snapshot.Error = "size/SHA-256 or publication failed"
			}
			s.mu.Unlock()
			_, e = r.t.Send(protocol.TypeFileAck, protocol.FlagResponse, NewAck(f.Header.MessageID, b.TransferID, status, dst.Received))
			return e
		default:
			return fmt.Errorf("unexpected file type %x", f.Header.Type)
		}
	}
}
