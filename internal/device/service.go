// Package device owns the in-memory device inventory and session lifecycle.
// It has no transport, task, file, or persistence dependencies.
package device

import (
	"errors"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const DefaultHistoryLimit = 64

var ErrNotFound = errors.New("device not found")

// Registration is the latest validated REGISTER declaration. Empty optional
// strings mean unknown. Capabilities are declarations, not authorization.
type Registration struct {
	DeviceID, Serial, Model, Firmware, ProbeVersion string
	Hostname, Arch, Kernel, Libc, BootID            string
	Capabilities                                    []string
	Template                                        *TemplateReference
	Attributes                                      map[string]Attribute
	CollectionErrors                                map[string]CollectionError
}
type TemplateReference struct {
	ID      string `json:"template_id"`
	Name    string `json:"name"`
	Version uint64 `json:"version"`
}
type Attribute struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type CollectionError struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type Status string

const (
	Online  Status = "online"
	Offline Status = "offline"
)

type EndReason string

const (
	Disconnected        EndReason = "disconnected"
	Replaced            EndReason = "replaced"
	HeartbeatTimeout    EndReason = "heartbeat_timeout"
	WriteError          EndReason = "write_error"
	ProtocolError       EndReason = "protocol_error"
	ServerClosed        EndReason = "server_closed"
	RequestedDisconnect EndReason = "requested_disconnect"
)

type Session struct {
	ID                             string
	Registration                   Registration
	StartedAt, LastSeenAt, EndedAt time.Time
	EndReason                      EndReason // empty while current
}

// Snapshot contains no history array; Sessions provides the retained history.
// LatestSession is current when online, or the most recently ended session.
// Zero LastOfflineAt means no online -> offline transition has been observed.
type Snapshot struct {
	Registration                                         Registration
	Status                                               Status
	FirstSeenAt, LastSeenAt, LastOnlineAt, LastOfflineAt time.Time
	CurrentSession                                       *Session
	LatestSession                                        Session
	TotalSessions, EvictedSessions                       uint64
}

type SessionHistory struct {
	Current                        *Session
	Ended                          []Session // oldest to newest, ordered by end transition
	Limit                          int
	TotalSessions, EvictedSessions uint64
}

// Query is the internal read surface for Application/Adapter consumers.
// Each call is an atomic snapshot; separate calls may observe later transitions.
type Query interface {
	List() []Snapshot // sorted by device_id
	Get(deviceID string) (Snapshot, error)
	Sessions(deviceID string) (SessionHistory, error)
}

type record struct {
	current                *Session
	history                []Session
	firstSeen, lastOffline time.Time
	total, evicted         uint64
}

type Service struct {
	revision     atomic.Uint64
	mu           sync.RWMutex
	devices      map[string]*record
	historyLimit int
}

// New uses the default limit for zero and rejects negative limits. Inventory
// entries remain until the owning Server process ends; only history is evicted.
func New(historyLimit int) (*Service, error) {
	if historyLimit < 0 {
		return nil, errors.New("device history limit must not be negative")
	}
	if historyLimit == 0 {
		historyLimit = DefaultHistoryLimit
	}
	return &Service{devices: make(map[string]*record), historyLimit: historyLimit}, nil
}

// Publish is called in Gateway publication order, after a complete REGISTER_ACK.
// The caller provides a fresh session ID and validated registration. A duplicate
// current ID is ignored, so it cannot manufacture a replacement or rewrite info.
func (s *Service) Publish(info Registration, sessionID string, at time.Time) {
	defer s.revision.Add(1)
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.devices[info.DeviceID]
	if r == nil {
		r = &record{firstSeen: at}
		s.devices[info.DeviceID] = r
	}
	if r.current != nil {
		if r.current.ID == sessionID {
			return
		}
		s.finish(r, Replaced, at)
	}
	r.current = &Session{ID: sessionID, Registration: cloneRegistration(info), StartedAt: at, LastSeenAt: at}
	r.total++
}

// Seen accepts activity only from the current session. Delayed callbacks for
// replaced/ended sessions cannot alter either current state or retained history.
func (s *Service) Seen(deviceID, sessionID string, at time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.devices[deviceID]
	if r == nil || r.current == nil || r.current.ID != sessionID {
		return false
	}
	if at.After(r.current.LastSeenAt) {
		r.current.LastSeenAt = at
	}
	return true
}

// End is idempotent and only transitions the current device online -> offline.
// Session replacement uses Publish and deliberately does not update lastOffline.
func (s *Service) End(deviceID, sessionID string, reason EndReason, at time.Time) bool {
	defer s.revision.Add(1)
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.devices[deviceID]
	if r == nil || r.current == nil || r.current.ID != sessionID {
		return false
	}
	s.finish(r, reason, at)
	r.lastOffline = at
	return true
}

func (s *Service) finish(r *record, reason EndReason, at time.Time) {
	ended := *r.current
	ended.EndedAt, ended.EndReason = at, reason
	if len(r.history) == s.historyLimit {
		copy(r.history, r.history[1:])
		r.history[len(r.history)-1] = ended
		r.evicted++
	} else {
		r.history = append(r.history, ended)
	}
	r.current = nil
}

func (s *Service) Get(deviceID string) (Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r := s.devices[deviceID]
	if r == nil {
		return Snapshot{}, ErrNotFound
	}
	return snapshot(r), nil
}

func (s *Service) List() []Snapshot {
	s.mu.RLock()
	result := make([]Snapshot, 0, len(s.devices))
	for _, r := range s.devices {
		result = append(result, snapshot(r))
	}
	s.mu.RUnlock()
	sort.Slice(result, func(i, j int) bool { return result[i].Registration.DeviceID < result[j].Registration.DeviceID })
	return result
}

func (s *Service) Sessions(deviceID string) (SessionHistory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r := s.devices[deviceID]
	if r == nil {
		return SessionHistory{}, ErrNotFound
	}
	h := SessionHistory{Limit: s.historyLimit, TotalSessions: r.total, EvictedSessions: r.evicted, Ended: make([]Session, len(r.history))}
	if r.current != nil {
		v := cloneSession(*r.current)
		h.Current = &v
	}
	for i, v := range r.history {
		h.Ended[i] = cloneSession(v)
	}
	return h, nil
}

func snapshot(r *record) Snapshot {
	v := Snapshot{Status: Offline, FirstSeenAt: r.firstSeen, LastOfflineAt: r.lastOffline, TotalSessions: r.total, EvictedSessions: r.evicted}
	if r.current != nil {
		v.Status = Online
		current := cloneSession(*r.current)
		v.CurrentSession = &current
		v.LatestSession = cloneSession(*r.current)
	} else {
		v.LatestSession = cloneSession(r.history[len(r.history)-1])
	}
	v.Registration = cloneRegistration(v.LatestSession.Registration)
	v.LastSeenAt = v.LatestSession.LastSeenAt
	v.LastOnlineAt = v.LatestSession.StartedAt
	return v
}

func cloneRegistration(v Registration) Registration {
	v.Capabilities = append([]string{}, v.Capabilities...)
	if v.Template != nil {
		t := *v.Template
		v.Template = &t
	}
	attrs := map[string]Attribute{}
	if v.Attributes == nil {
		attrs = nil
	}
	for k, a := range v.Attributes {
		attrs[k] = a
	}
	v.Attributes = attrs
	errs := map[string]CollectionError{}
	if v.CollectionErrors == nil {
		errs = nil
	}
	for k, e := range v.CollectionErrors {
		errs[k] = e
	}
	v.CollectionErrors = errs
	return v
}

func cloneSession(v Session) Session {
	v.Registration = cloneRegistration(v.Registration)
	return v
}

func (s *Service) Revision() uint64 { return s.revision.Load() }
