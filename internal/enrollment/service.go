// Package enrollment owns durable admission and desired probe configuration.
// Live sessions and observations remain in Device Service.
package enrollment

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"routerprobe/internal/catalogupgrade"
	"routerprobe/internal/device"
	"routerprobe/internal/probetemplate"
	"routerprobe/internal/protocol"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var ErrNotManaged = errors.New("device is not managed")

type Configuration struct {
	Revision           uint64                 `json:"revision"`
	TemplateGeneration uint64                 `json:"template_generation"`
	Template           probetemplate.Template `json:"template"`
}
type InterfaceSampling struct {
	NetworkSeconds    *uint32 `json:"network_seconds,omitempty"`
	NetworkInterfaces *string `json:"network_interfaces,omitempty"`
}
type Profile struct {
	DeviceID          string                    `json:"device_id"`
	Version           uint64                    `json:"version"`
	Admission         string                    `json:"admission"`
	Name              string                    `json:"name"`
	ModelID           string                    `json:"model_id"`
	ModelName         string                    `json:"model_name"`
	Monitoring        *probetemplate.Monitoring `json:"monitoring,omitempty"`
	PropertyIntervals map[string]uint32         `json:"property_intervals,omitempty"`
	InterfaceSampling *InterfaceSampling        `json:"interface_sampling,omitempty"`
	BoundTemplate     *probetemplate.Template   `json:"bound_template,omitempty"`
	Configuration     Configuration             `json:"configuration"`
	Reported          device.Registration       `json:"reported"`
	FirstSeen         time.Time                 `json:"first_seen"`
	LastSeen          time.Time                 `json:"last_seen"`
}
type Model struct {
	ID         string   `json:"model_id"`
	Name       string   `json:"name"`
	Version    uint64   `json:"version"`
	Aliases    []string `json:"aliases"`
	TemplateID string   `json:"template_id"`
}
type catalog struct {
	Schema  int                `json:"schema_version"`
	Devices map[string]Profile `json:"devices"`
	Models  map[string]Model   `json:"models"`
}
type Service struct {
	mu              sync.RWMutex
	path            string
	lock            *os.File
	data            catalog
	revision        atomic.Uint64
	closed          bool
	startupWarnings []string
}

func clone[T any](v T) T { b, _ := json.Marshal(v); var n T; _ = json.Unmarshal(b, &n); return n }
func Open(path string) (*Service, error) {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return nil, e
	}
	lock, e := lockDirectory(path + ".lock")
	if e != nil {
		return nil, e
	}
	s := &Service{path: path, lock: lock, data: catalog{Schema: 1, Devices: map[string]Profile{}, Models: map[string]Model{}}}
	fail := func(e error) (*Service, error) {
		lock.Close()
		return nil, fmt.Errorf("device catalog %s: %w", path, e)
	}
	f, e := os.Open(path)
	if errors.Is(e, os.ErrNotExist) {
		st, err := lock.Stat()
		if err != nil {
			return fail(err)
		}
		if st.Size() != 0 {
			return fail(errors.New("device catalog missing"))
		}
		if e = s.save(s.data); e != nil {
			return fail(e)
		}
	} else {
		if e != nil {
			return fail(e)
		}
		b, err := io.ReadAll(io.LimitReader(f, 32*1024*1024+1))
		f.Close()
		if err != nil {
			return fail(err)
		}
		if len(b) > 32*1024*1024 || protocol.ValidStoredObject(b) != nil {
			return fail(probetemplate.ErrInvalid)
		}
		original := b
		b, upgraded := catalogupgrade.Devices(b)
		d := json.NewDecoder(bytes.NewReader(b))
		d.DisallowUnknownFields()
		if e := d.Decode(&s.data); e != nil {
			return fail(e)
		}
		if s.data.Schema != 1 || s.data.Devices == nil || s.data.Models == nil {
			return fail(probetemplate.ErrInvalid)
		}
		for id, p := range s.data.Devices {
			if id != p.DeviceID || p.Version == 0 || !admission(p.Admission) || !probetemplate.ValidText(id, 128) {
				return fail(probetemplate.ErrInvalid)
			}
		}
		if upgraded {
			backup, err := catalogupgrade.Backup(path, original)
			if err != nil {
				return fail(err)
			}
			if err = s.save(s.data); err != nil {
				return fail(err)
			}
			s.startupWarnings = append(s.startupWarnings, "retired device snapshot metadata removed; original catalog backup: "+backup)
		}
	}
	if _, e = lock.WriteAt([]byte("1"), 0); e != nil {
		return fail(e)
	}
	if e = lock.Sync(); e != nil {
		return fail(e)
	}
	return s, nil
}

func (s *Service) StartupWarnings() []string { return append([]string(nil), s.startupWarnings...) }

func (s *Service) save(c catalog) error {
	b, e := json.Marshal(c)
	if e != nil {
		return e
	}
	if len(b) > 32*1024*1024 {
		return probetemplate.ErrCapacity
	}
	f, e := os.CreateTemp(filepath.Dir(s.path), ".devices-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(b); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return replaceFile(f.Name(), s.path)
}
func (s *Service) commit(c catalog) error {
	if s.closed {
		return probetemplate.ErrClosed
	}
	if e := s.save(c); e != nil {
		return e
	}
	s.data = c
	s.revision.Add(1)
	return nil
}
func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return s.lock.Close()
}
func (s *Service) Revision() uint64 { return s.revision.Load() }
func (s *Service) Discover(r device.Registration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := clone(s.data)
	p, ok := c.Devices[r.DeviceID]
	now := time.Now().UTC()
	if !ok {
		p = Profile{DeviceID: r.DeviceID, Version: 1, Admission: "pending", FirstSeen: now}
		p.Name = r.Hostname
		if p.Name == "" {
			p.Name = r.DeviceID
		}
	}
	p.Reported = clone(r)
	p.LastSeen = now
	c.Devices[r.DeviceID] = p
	return s.commit(c)
}
func (s *Service) Get(id string) (Profile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.data.Devices[id]
	if !ok {
		return Profile{}, device.ErrNotFound
	}
	return clone(p), nil
}
func (s *Service) List() []Profile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Profile{}
	for _, p := range s.data.Devices {
		out = append(out, clone(p))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DeviceID < out[j].DeviceID })
	return out
}
func (s *Service) RequireManaged(id string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.data.Devices[id].Admission != "managed" {
		return ErrNotManaged
	}
	return nil
}
func admission(v string) bool { return v == "pending" || v == "managed" || v == "ignored" }
func (s *Service) Put(p Profile, expected uint64) (Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.data.Devices[p.DeviceID]
	if !ok {
		return Profile{}, device.ErrNotFound
	}
	if old.Version != expected || expected == ^uint64(0) {
		return Profile{}, probetemplate.ErrConflict
	}
	if old.Admission == "managed" && p.Admission != "managed" {
		return Profile{}, probetemplate.ErrConflict
	}
	if !admission(p.Admission) || !probetemplate.ValidText(p.Name, 128) {
		return Profile{}, probetemplate.ErrInvalid
	}
	p.Version = expected + 1
	p.Reported = old.Reported
	p.FirstSeen = old.FirstSeen
	p.LastSeen = old.LastSeen
	c := clone(s.data)
	c.Devices[p.DeviceID] = clone(p)
	if e := s.commit(c); e != nil {
		return Profile{}, e
	}
	return clone(p), nil
}
func (s *Service) Models() []Model {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Model{}
	for _, m := range s.data.Models {
		out = append(out, clone(m))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
func norm(v string) string { return strings.ToLower(strings.TrimSpace(v)) }
func (s *Service) PutModel(m Model) (Model, error) {
	if !probetemplate.ValidText(m.ID, 128) || !probetemplate.ValidText(m.Name, 128) || len(m.Aliases) > 32 {
		return Model{}, probetemplate.ErrInvalid
	}
	names := map[string]bool{norm(m.Name): true}
	for _, a := range m.Aliases {
		if !probetemplate.ValidText(a, 128) || names[norm(a)] {
			return Model{}, probetemplate.ErrInvalid
		}
		names[norm(a)] = true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.data.Models[m.ID]
	if old.Version != m.Version || m.Version == ^uint64(0) {
		return Model{}, probetemplate.ErrConflict
	}
	for id, other := range s.data.Models {
		if id == m.ID {
			continue
		}
		for _, v := range append([]string{other.Name}, other.Aliases...) {
			if names[norm(v)] {
				return Model{}, probetemplate.ErrConflict
			}
		}
	}
	if m.Aliases == nil {
		m.Aliases = []string{}
	}
	m.Version++
	c := clone(s.data)
	c.Models[m.ID] = clone(m)
	if e := s.commit(c); e != nil {
		return Model{}, e
	}
	return clone(m), nil
}
func (s *Service) Match(name string) *Model {
	for _, m := range s.Models() {
		for _, v := range append([]string{m.Name}, m.Aliases...) {
			if norm(name) != "" && norm(name) == norm(v) {
				return &m
			}
		}
	}
	return nil
}
func (s *Service) Referenced(templateID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.data.Devices {
		if p.BoundTemplate != nil && p.BoundTemplate.ID == templateID {
			return true
		}
	}
	for _, m := range s.data.Models {
		if m.TemplateID == templateID {
			return true
		}
	}
	return false
}
