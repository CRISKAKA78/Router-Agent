package gateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"routerprobe/internal/devicelog"
	"routerprobe/internal/protocol"
	"routerprobe/internal/routerconfig"
	"slices"
	"sync"
	"time"
)

type logReply struct {
	Event     string          `json:"event"`
	RequestID string          `json:"request_id"`
	Data      json.RawMessage `json:"data"`
	Error     string          `json:"error"`
}
type logRequests struct {
	sync.Mutex
	pending map[string]chan logReply
	liveAt  time.Time
}

func (s *Server) logSession(id, expected string) (*session, error) {
	if e := s.requireManaged(id); e != nil {
		return nil, e
	}
	s.mu.Lock()
	active := s.sessions[id]
	s.mu.Unlock()
	if active == nil {
		return nil, ErrOffline
	}
	if expected == "" || active.sessionID != expected {
		return nil, ErrSessionChanged
	}
	if !slices.Contains(active.capabilities, devicelog.Capability) {
		return nil, routerconfig.ErrUnsupported
	}
	return active, nil
}
func (s *Server) CreateDeviceLog(ctx context.Context, id, expected string, p devicelog.Params) (string, error) {
	if p.Validate() != nil {
		return "", devicelog.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return "", e
	}
	active, e := s.logSession(id, expected)
	if e != nil {
		return "", e
	}
	spec, e := s.tasks.NewDeviceLog(id, p)
	if e != nil {
		return "", e
	}
	message, e := s.dispatchChecked(active, spec, true)
	if e != nil && message == 0 {
		s.tasks.Remove(spec.ID)
		return "", e
	}
	return spec.ID, e
}
func (s *Server) QueryDeviceLog(ctx context.Context, id, expected string, q devicelog.Query) (json.RawMessage, error) {
	if e := q.Validate(); e != nil {
		return nil, e
	}
	active, e := s.logSession(id, expected)
	if e != nil {
		return nil, e
	}
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	var nonce [16]byte
	if _, e = rand.Read(nonce[:]); e != nil {
		return nil, e
	}
	q.Event = "device_log_query"
	q.RequestID = hex.EncodeToString(nonce[:])
	ch := make(chan logReply, 1)
	active.logs.Lock()
	if len(active.logs.pending) >= 2 || (q.Operation == "live" && time.Now().Before(active.logs.liveAt)) {
		active.logs.Unlock()
		return nil, devicelog.ErrBusy
	}
	if q.Operation == "live" {
		active.logs.liveAt = time.Now().Add(500 * time.Millisecond)
	}
	if active.logs.pending == nil {
		active.logs.pending = map[string]chan logReply{}
	}
	active.logs.pending[q.RequestID] = ch
	active.logs.Unlock()
	defer func() { active.logs.Lock(); delete(active.logs.pending, q.RequestID); active.logs.Unlock() }()
	_, e = active.transport.sendJSON(protocol.TypeEvent, 0, q, func(uint64) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.sessions[id] != active {
			return ErrSessionChanged
		}
		return nil
	})
	if e != nil {
		return nil, e
	}
	select {
	case r := <-ch:
		if r.Error != "" {
			return nil, fmt.Errorf("%w: %s", devicelog.ErrRead, r.Error)
		}
		return r.Data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-active.lifetime:
		return nil, ErrSessionChanged
	}
}
func acceptLogReply(active *session, raw []byte) error {
	var r logReply
	if len(raw) > 60000 || protocol.ValidObject(raw) != nil || json.Unmarshal(raw, &r) != nil || r.Event != "device_log_reply" || len(r.RequestID) != 32 || len(r.Error) > 256 || !slices.Contains(active.capabilities, devicelog.Capability) {
		return errors.New("invalid device log reply")
	}
	active.logs.Lock()
	ch := active.logs.pending[r.RequestID]
	active.logs.Unlock()
	if ch != nil {
		select {
		case ch <- r:
		default:
		}
	}
	return nil
}
