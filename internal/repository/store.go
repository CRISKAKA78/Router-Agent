package repository

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"routerprobe/internal/protocol"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

const DefaultDirectory = "./data/repository"

// Store owns one catalog transaction boundary for asset/version references.
// Readers receive copies. Its lock is never a Device/Gateway lock.
type Store struct {
	revision atomic.Uint64
	mu       sync.RWMutex
	root     string
	lock     *os.File
	data     catalog
	closed   bool
	publish  func(string, string) error
}

func Open(directory string) (*Store, error) {
	if directory == "" {
		directory = DefaultDirectory
	}
	root, e := filepath.Abs(directory)
	if e != nil {
		return nil, e
	}
	if e = os.MkdirAll(root, 0700); e != nil {
		return nil, e
	}
	f, e := lockDirectory(filepath.Join(root, "repository.lock"))
	if e != nil {
		return nil, fmt.Errorf("repository directory lock: %w", e)
	}
	s := &Store{root: root, lock: f, publish: replaceFile}
	ok := false
	defer func() {
		if !ok {
			f.Close()
		}
	}()
	for _, dir := range []string{"metadata", "staging", filepath.Join("blobs", "sha256")} {
		if e = os.MkdirAll(filepath.Join(root, dir), 0700); e != nil {
			return nil, e
		}
	}
	b, e := os.ReadFile(s.catalogPath())
	if os.IsNotExist(e) {
		lockInfo, err := f.Stat()
		if err != nil {
			return nil, err
		}
		if lockInfo.Size() != 0 {
			return nil, errors.New("initialized repository catalog missing")
		}
		// A missing catalog in an existing repository must not silently erase identity history.
		for _, dir := range []string{"metadata", "staging", filepath.Join("blobs", "sha256")} {
			entries, err := os.ReadDir(filepath.Join(root, dir))
			if err != nil {
				return nil, err
			}
			if len(entries) > 0 {
				return nil, errors.New("catalog missing in nonempty repository")
			}
		}
		s.data = catalog{SchemaVersion: 1, Assets: []Asset{}, Tools: []Tool{}, Versions: []Version{}}
		if e = s.commit(s.data); e != nil {
			return nil, e
		}
	} else {
		if e != nil {
			return nil, e
		}
		if !protocol.ValidUnicodeJSON(b) {
			return nil, errors.New("invalid catalog Unicode")
		}
		if e = uniqueJSON(b); e != nil {
			return nil, e
		}
		d := json.NewDecoder(bytes.NewReader(b))
		d.DisallowUnknownFields()
		if e = d.Decode(&s.data); e != nil {
			return nil, e
		}
		if e = d.Decode(new(interface{})); e != io.EOF {
			return nil, errors.New("trailing catalog data")
		}
		if e = validateCatalog(s.data); e != nil {
			return nil, e
		}
		for _, a := range s.data.Assets {
			st, err := os.Lstat(s.blobPath(a.SHA256))
			if err != nil {
				return nil, err
			}
			if !st.Mode().IsRegular() || st.Size() != a.Size {
				return nil, ErrIntegrity
			}
		}
	}
	// This retained initialization marker distinguishes a fresh directory from a
	// repository whose catalog was removed, even when it only contained Tools.
	if _, e = f.WriteAt([]byte("RMP Repository v1\n"), 0); e != nil {
		return nil, e
	}
	if e = f.Sync(); e != nil {
		return nil, e
	}
	ok = true
	return s, nil
}
func (s *Store) Directory() string   { return s.root }
func (s *Store) Files() *FileService { return &FileService{s} }
func (s *Store) Tools() *ToolService { return &ToolService{s} }
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return s.lock.Close()
}
func (s *Store) catalogPath() string      { return filepath.Join(s.root, "metadata", "catalog.json") }
func (s *Store) blobPath(h string) string { return filepath.Join(s.root, "blobs", "sha256", h[:2], h) }
func (s *Store) commit(c catalog) error {
	if s.closed {
		return ErrClosed
	}
	b, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Join(s.root, "metadata"), ".catalog-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, e = f.Write(b); e != nil {
		return e
	}
	if e = f.Sync(); e != nil {
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	if e = s.publish(f.Name(), s.catalogPath()); e != nil {
		return e
	}
	s.data = c
	s.revision.Add(1)
	return nil
}
func (s *Store) nextID() (string, error) {
	for {
		id, e := uuid()
		if e != nil {
			return "", e
		}
		used := false
		for _, a := range s.data.Assets {
			used = used || a.ID == id
		}
		for _, t := range s.data.Tools {
			used = used || t.ID == id
		}
		for _, v := range s.data.Versions {
			for _, a := range v.Artifacts {
				used = used || a.ID == id
			}
		}
		if !used {
			return id, nil
		}
	}
}
func validateCatalog(c catalog) error {
	if c.SchemaVersion != 1 || c.Assets == nil || c.Tools == nil || c.Versions == nil {
		return errors.New("invalid catalog schema")
	}
	ids := map[string]bool{}
	assets := map[string]Asset{}
	tools := map[string]Tool{}
	versions := map[string]bool{}
	addID := func(id string) error {
		if !validID(id) || ids[id] {
			return ErrConflict
		}
		ids[id] = true
		return nil
	}
	for _, a := range c.Assets {
		if e := addID(a.ID); e != nil {
			return e
		}
		h, e := hex.DecodeString(a.SHA256)
		if e != nil || len(h) != 32 || strings.ToLower(a.SHA256) != a.SHA256 || a.Size < 0 || !textOK(a.Name, 255, false) || a.CreatedAt.IsZero() {
			return errors.New("invalid asset metadata")
		}
		assets[a.ID] = a
	}
	for _, t := range c.Tools {
		if e := addID(t.ID); e != nil {
			return e
		}
		if !textOK(t.Name, 255, false) || !textOK(t.Description, 4096, true) || t.CreatedAt.IsZero() {
			return errors.New("invalid tool metadata")
		}
		tools[t.ID] = t
	}
	for _, v := range c.Versions {
		key := v.ToolID + "\x00" + v.Version
		if _, ok := tools[v.ToolID]; !ok {
			return errors.New("unknown version tool")
		}
		if !textOK(v.Version, 128, false) || versions[key] || v.CreatedAt.IsZero() || len(v.Artifacts) == 0 || len(v.Artifacts) > 128 {
			return errors.New("invalid version metadata")
		}
		versions[key] = true
		for _, a := range v.Artifacts {
			if e := addID(a.ID); e != nil {
				return e
			}
			asset, ok := assets[a.AssetID]
			if !ok || asset.Archived && !v.Archived {
				return errors.New("invalid artifact reference")
			}
			spec := ArtifactSpec{a.AssetID, a.Platform, a.Mode, a.Rules}
			n, e := normalizeSpec(spec)
			if e != nil || !reflect.DeepEqual(n, spec) {
				return errors.New("invalid artifact metadata")
			}
		}
	}
	return nil
}

// Reject duplicate object members as well as malformed JSON, so corrupt catalogs
// cannot reinterpret an earlier identity or reference with a later duplicate key.
func uniqueJSON(b []byte) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var value func() error
	value = func() error {
		t, e := d.Token()
		if e != nil {
			return e
		}
		delim, ok := t.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				k, e := d.Token()
				if e != nil {
					return e
				}
				key, ok := k.(string)
				if !ok || seen[key] {
					return errors.New("duplicate catalog key")
				}
				seen[key] = true
				if e = value(); e != nil {
					return e
				}
			}
		case '[':
			for d.More() {
				if e = value(); e != nil {
					return e
				}
			}
		default:
			return errors.New("invalid JSON delimiter")
		}
		_, e = d.Token()
		return e
	}
	if e := value(); e != nil {
		return e
	}
	if _, e := d.Token(); e != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}
func copyContent(ctx context.Context, w io.Writer, r io.Reader) (int64, string, error) {
	h := sha256.New()
	buf := make([]byte, 64*1024)
	var total int64
	for {
		if e := ctx.Err(); e != nil {
			return 0, "", e
		}
		n, e := r.Read(buf)
		if n > 0 {
			written, err := w.Write(buf[:n])
			if err != nil {
				return 0, "", err
			}
			if written != n {
				return 0, "", io.ErrShortWrite
			}
			h.Write(buf[:n])
			total += int64(n)
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			return 0, "", e
		}
	}
	return total, hex.EncodeToString(h.Sum(nil)), nil
}
func verifyContent(ctx context.Context, path string, n int64, h string) error {
	st, e := os.Lstat(path)
	if e != nil {
		return e
	}
	if !st.Mode().IsRegular() {
		return ErrIntegrity
	}
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	actual, digest, e := copyContent(ctx, io.Discard, f)
	if e != nil {
		return e
	}
	if actual != n || digest != h {
		return ErrIntegrity
	}
	return nil
}

// Leftovers reports files without deleting crash leftovers or unreferenced blobs.
func (s *Store) Leftovers() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, ErrClosed
	}
	referenced := map[string]bool{}
	for _, a := range s.data.Assets {
		referenced[s.blobPath(a.SHA256)] = true
	}
	result := []string{}
	for _, sub := range []string{"staging", "metadata", "blobs"} {
		e := filepath.WalkDir(filepath.Join(s.root, sub), func(p string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if !d.IsDir() && p != s.catalogPath() && !referenced[p] {
				result = append(result, p)
			}
			if d.IsDir() && sub == "staging" && p != filepath.Join(s.root, sub) {
				result = append(result, p)
			}
			return nil
		})
		if e != nil {
			return nil, e
		}
	}
	sort.Strings(result)
	return result, nil
}

// NewDownloadTarget reserves a private directory. Only its owner may remove it,
// and only after the existing transfer has released the target.
func (s *Store) NewDownloadTarget() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return "", ErrClosed
	}
	dir, e := os.MkdirTemp(filepath.Join(s.root, "staging"), "download-*")
	if e != nil {
		return "", e
	}
	return filepath.Join(dir, "payload"), nil
}
func (s *Store) ReleaseDownloadTarget(path string) error {
	if filepath.Base(path) != "payload" || filepath.Dir(filepath.Dir(path)) != filepath.Join(s.root, "staging") || !strings.HasPrefix(filepath.Base(filepath.Dir(path)), "download-") {
		return errors.New("invalid staging target")
	}
	if e := os.Remove(path); e != nil && !os.IsNotExist(e) {
		return e
	}
	return os.Remove(filepath.Dir(path))
}
