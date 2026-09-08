// Package probetemplate owns server-managed startup collection templates.
package probetemplate

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"routerprobe/internal/protocol"
	"routerprobe/internal/routerconfig"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"
)

var (
	ErrInvalid  = errors.New("invalid probe template")
	ErrNotFound = errors.New("probe template not found")
	ErrConflict = errors.New("probe template conflict")
	ErrClosed   = errors.New("probe template service closed")
	ErrCapacity = errors.New("probe template capacity exhausted")
	keyPattern  = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
)

type Property struct {
	Name    string `json:"name"`
	Command string `json:"command,omitempty"`
	Source  string `json:"source,omitempty"`
	Key     string `json:"key,omitempty"`
	Timeout uint32 `json:"timeout_seconds"`
}

// Keep command and configuration key mutually exclusive even when a caller
// explicitly sends an empty, otherwise omittable field.
func (p *Property) UnmarshalJSON(b []byte) error {
	type plain Property
	var value plain
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&value); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil {
		return err
	}
	_, command := fields["command"]
	_, key := fields["key"]
	if ((value.Source == "" || value.Source == "command") && key) ||
		((value.Source == "nvram" || value.Source == "uci") && command) {
		return ErrInvalid
	}
	*p = Property(value)
	return nil
}

type Template struct {
	ID         string              `json:"template_id"`
	Name       string              `json:"name"`
	Version    uint64              `json:"version"`
	Properties map[string]Property `json:"properties"`
	Deleted    bool                `json:"deleted,omitempty"`
}
type Input struct {
	Name       string              `json:"name"`
	Properties map[string]Property `json:"properties"`
}
type catalog struct {
	Schema    int        `json:"schema_version"`
	Templates []Template `json:"templates"`
}
type Service struct {
	mu     sync.RWMutex
	path   string
	lock   *os.File
	items  map[string]Template
	closed bool
}

func ValidText(s string, max int) bool {
	return s != "" && len(s) <= max && utf8.ValidString(s) && !strings.ContainsRune(s, 0)
}
func ValidKey(key string) bool {
	if !keyPattern.MatchString(key) {
		return false
	}
	switch key {
	case "device_id", "arch", "boot_id", "probe_version", "capabilities", "template", "attributes", "collection_errors":
		return false
	}
	return true
}
func StandardLimit(key string) int {
	switch key {
	case "serial", "model", "firmware", "kernel":
		return 128
	case "hostname":
		return 255
	case "libc":
		return 64
	}
	return 0
}
func Validate(v Input) error {
	if !ValidText(v.Name, 128) || strings.TrimSpace(v.Name) != v.Name || len(v.Properties) < 1 || len(v.Properties) > 38 {
		return ErrInvalid
	}
	custom := 0
	for k, p := range v.Properties {
		if !ValidKey(k) || !ValidText(p.Name, 128) || p.Timeout < 1 || p.Timeout > 30 {
			return ErrInvalid
		}
		switch p.Source {
		case "", "command":
			if p.Key != "" || !ValidText(p.Command, 4096) || strings.TrimSpace(p.Command) == "" {
				return ErrInvalid
			}
		case "nvram", "uci":
			if p.Command != "" || !routerconfig.ValidKey(p.Source, p.Key) {
				return ErrInvalid
			}
		default:
			return ErrInvalid
		}
		if StandardLimit(k) == 0 {
			custom++
		}
	}
	b, _ := json.Marshal(v)
	if custom > 32 || len(b) > 48*1024 {
		return ErrInvalid
	}
	return nil
}
func normalize(v Input) Input {
	v.Properties = clone(Template{Properties: v.Properties}).Properties
	for k, p := range v.Properties {
		if p.Timeout == 0 {
			p.Timeout = 5
			v.Properties[k] = p
		}
	}
	return v
}
func clone(v Template) Template {
	m := make(map[string]Property, len(v.Properties))
	for k, p := range v.Properties {
		m[k] = p
	}
	v.Properties = m
	return v
}
func Open(path string) (*Service, error) {
	path, e := filepath.Abs(path)
	if e != nil {
		return nil, e
	}
	if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return nil, e
	}
	lock, e := lockDirectory(path + ".lock")
	if e != nil {
		return nil, e
	}
	s := &Service{path: path, lock: lock, items: map[string]Template{}}
	fail := func(e error) (*Service, error) { lock.Close(); return nil, e }
	f, e := os.Open(path)
	if errors.Is(e, os.ErrNotExist) {
		info, statErr := lock.Stat()
		if statErr != nil {
			return fail(statErr)
		}
		if info.Size() != 0 {
			return fail(errors.New("probe template catalog missing"))
		}
		if e = s.save(s.items); e != nil {
			return fail(e)
		}
	} else {
		if e != nil {
			return fail(e)
		}
		b, readErr := io.ReadAll(io.LimitReader(f, 8*1024*1024+1))
		f.Close()
		if readErr != nil {
			return fail(readErr)
		}
		if len(b) > 8*1024*1024 || protocol.ValidObject(b) != nil {
			return fail(ErrInvalid)
		}
		var c catalog
		d := json.NewDecoder(bytes.NewReader(b))
		d.DisallowUnknownFields()
		if d.Decode(&c) != nil || c.Schema != 1 || len(c.Templates) > 1000 {
			return fail(ErrInvalid)
		}
		names := map[string]bool{}
		for _, v := range c.Templates {
			if !ValidText(v.ID, 128) || v.Version < 1 || Validate(Input{v.Name, v.Properties}) != nil {
				return fail(ErrInvalid)
			}
			if _, ok := s.items[v.ID]; ok || (!v.Deleted && names[v.Name]) {
				return fail(ErrInvalid)
			}
			if !v.Deleted {
				names[v.Name] = true
			}
			s.items[v.ID] = v
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
func (s *Service) save(items map[string]Template) error {
	c := catalog{Schema: 1, Templates: []Template{}}
	for _, v := range items {
		c.Templates = append(c.Templates, v)
	}
	sort.Slice(c.Templates, func(i, j int) bool { return c.Templates[i].ID < c.Templates[j].ID })
	b, e := json.Marshal(c)
	if e != nil {
		return e
	}
	if len(b) > 8*1024*1024 {
		return ErrCapacity
	}
	f, e := os.CreateTemp(filepath.Dir(s.path), ".probe-templates-*")
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
func (s *Service) List() ([]Template, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, ErrClosed
	}
	out := []Template{}
	for _, v := range s.items {
		if !v.Deleted {
			out = append(out, clone(v))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
func (s *Service) Resolve(id, name string) (Template, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return Template{}, ErrClosed
	}
	if (id == "") == (name == "") {
		return Template{}, ErrInvalid
	}
	for _, v := range s.items {
		if !v.Deleted && ((id != "" && v.ID == id) || (name != "" && v.Name == name)) {
			return clone(v), nil
		}
	}
	return Template{}, ErrNotFound
}
func (s *Service) Put(id string, expected uint64, in Input) (Template, error) {
	in = normalize(in)
	if e := Validate(in); e != nil {
		return Template{}, e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Template{}, ErrClosed
	}
	v := Template{ID: id, Name: in.Name, Version: 1, Properties: in.Properties}
	if id == "" {
		if len(s.items) >= 1000 {
			return Template{}, ErrCapacity
		}
		var b [16]byte
		for {
			if _, e := rand.Read(b[:]); e != nil {
				return Template{}, e
			}
			v.ID = hex.EncodeToString(b[:])
			if _, ok := s.items[v.ID]; !ok {
				break
			}
		}
	} else {
		old, ok := s.items[id]
		if !ok || old.Deleted {
			return Template{}, ErrNotFound
		}
		if expected == 0 || old.Version != expected || expected == ^uint64(0) {
			return Template{}, ErrConflict
		}
		v.Version = expected + 1
	}
	for _, old := range s.items {
		if !old.Deleted && old.ID != v.ID && old.Name == in.Name {
			return Template{}, ErrConflict
		}
	}
	next := make(map[string]Template, len(s.items)+1)
	for k, t := range s.items {
		next[k] = t
	}
	next[v.ID] = v
	if e := s.save(next); e != nil {
		return Template{}, e
	}
	s.items = next
	return clone(v), nil
}
func (s *Service) Delete(id string, expected uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	v, ok := s.items[id]
	if !ok || v.Deleted {
		return ErrNotFound
	}
	if expected == 0 || v.Version != expected {
		return ErrConflict
	}
	next := make(map[string]Template, len(s.items))
	for k, t := range s.items {
		next[k] = t
	}
	v.Deleted = true
	next[id] = v
	if e := s.save(next); e != nil {
		return e
	}
	s.items = next
	return nil
}
func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return s.lock.Close()
}
