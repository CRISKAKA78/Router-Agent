package repository

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type FileService struct{ s *Store }

// Import creates a new business identity even when the bytes already exist.
func (f *FileService) Import(ctx context.Context, name string, source io.Reader) (Asset, error) {
	return f.importContent(ctx, name, source, nil)
}

// ImportVerified binds a completed download to its previously committed metadata.
func (f *FileService) ImportVerified(ctx context.Context, name string, source io.Reader, size int64, digest string) (Asset, error) {
	return f.importContent(ctx, name, source, &Asset{Size: size, SHA256: digest})
}
func (f *FileService) importContent(ctx context.Context, name string, source io.Reader, expected *Asset) (Asset, error) {
	if !textOK(name, 255, false) || source == nil {
		return Asset{}, errors.New("invalid asset input")
	}
	s := f.s
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Asset{}, ErrClosed
	}
	temp, e := os.CreateTemp(filepath.Join(s.root, "staging"), "import-*")
	if e != nil {
		return Asset{}, e
	}
	defer os.Remove(temp.Name())
	defer temp.Close()
	n, h, e := copyContent(ctx, temp, source)
	if e != nil {
		return Asset{}, e
	}
	if expected != nil && (n != expected.Size || h != expected.SHA256) {
		return Asset{}, ErrIntegrity
	}
	if e = temp.Sync(); e != nil {
		return Asset{}, e
	}
	if e = temp.Close(); e != nil {
		return Asset{}, e
	}
	target := s.blobPath(h)
	if e = os.MkdirAll(filepath.Dir(target), 0700); e != nil {
		return Asset{}, e
	}
	if e = os.Link(temp.Name(), target); e != nil {
		if !os.IsExist(e) {
			return Asset{}, e
		}
		if e = verifyContent(ctx, target, n, h); e != nil {
			return Asset{}, e
		}
	}
	if e = ctx.Err(); e != nil {
		return Asset{}, e
	}
	id, e := s.nextID()
	if e != nil {
		return Asset{}, e
	}
	a := Asset{ID: id, Name: name, Size: n, SHA256: h, CreatedAt: time.Now().UTC()}
	c := cloneCatalog(s.data)
	c.Assets = append(c.Assets, a)
	if e = s.commit(c); e != nil {
		return Asset{}, e
	}
	return a, nil
}
func (f *FileService) Get(id string) (Asset, error) {
	s := f.s
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return Asset{}, ErrClosed
	}
	return s.asset(id)
}
func (s *Store) asset(id string) (Asset, error) {
	for _, a := range s.data.Assets {
		if a.ID == id {
			return a, nil
		}
	}
	return Asset{}, ErrNotFound
}
func (f *FileService) List(includeArchived bool) ([]Asset, error) {
	s := f.s
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, ErrClosed
	}
	out := []Asset{}
	for _, a := range s.data.Assets {
		if includeArchived || !a.Archived {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
func (f *FileService) Archive(id string) error {
	s := f.s
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	a, e := s.asset(id)
	if e != nil {
		return e
	}
	if a.Archived {
		return nil
	}
	for _, v := range s.data.Versions {
		if !v.Archived {
			for _, a := range v.Artifacts {
				if a.AssetID == id {
					return ErrReferenced
				}
			}
		}
	}
	c := cloneCatalog(s.data)
	for i := range c.Assets {
		if c.Assets[i].ID == id {
			c.Assets[i].Archived = true
		}
	}
	return s.commit(c)
}

// WithAsset admits a use while preventing archive/close until preparation and
// dispatch return. Callback must not call Repository methods (non-reentrant).
// The path is internal; the transfer must also check the expected digest when opening.
func (f *FileService) WithAsset(ctx context.Context, id string, use func(Asset, string) error) error {
	s := f.s
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return ErrClosed
	}
	a, e := s.asset(id)
	if e != nil {
		return e
	}
	if a.Archived {
		return ErrArchived
	}
	if e = verifyContent(ctx, s.blobPath(a.SHA256), a.Size, a.SHA256); e != nil {
		return e
	}
	return use(a, s.blobPath(a.SHA256))
}
