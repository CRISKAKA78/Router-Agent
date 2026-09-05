package filetransfer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"hash"
	"io"
	"os"
	"path/filepath"
)

func OpenSource(ctx context.Context, path string) (*os.File, int64, string, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, 0, "", e
	}
	ok := false
	defer func() {
		if !ok {
			f.Close()
		}
	}()
	st, e := f.Stat()
	if e != nil {
		return nil, 0, "", e
	}
	if !st.Mode().IsRegular() {
		return nil, 0, "", errors.New("source must be a regular file")
	}
	h := sha256.New()
	buf := make([]byte, 64*1024)
	var size int64
	for {
		if e = ctx.Err(); e != nil {
			return nil, 0, "", e
		}
		n, err := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
			size += int64(n)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, "", err
		}
	}
	if _, e = f.Seek(0, io.SeekStart); e != nil {
		return nil, 0, "", e
	}
	ok = true
	return f, size, hex.EncodeToString(h.Sum(nil)), nil
}

type Receiver struct {
	f            *os.File
	temp, target string
	overwrite    bool
	h            hash.Hash
	Received     int64
	Committed    bool
}

func NewReceiver(target string, overwrite bool) (*Receiver, error) {
	if !filepath.IsAbs(target) {
		return nil, errors.New("local target must be absolute")
	}
	if !overwrite {
		if _, e := os.Lstat(target); e == nil {
			return nil, os.ErrExist
		} else if !os.IsNotExist(e) {
			return nil, e
		}
	}
	f, e := os.CreateTemp(filepath.Dir(target), ".rmp-transfer-*")
	if e != nil {
		return nil, e
	}
	return &Receiver{f: f, temp: f.Name(), target: target, overwrite: overwrite, h: sha256.New()}, nil
}
func (r *Receiver) Write(b []byte) error {
	n, e := r.f.Write(b)
	if n > 0 {
		r.h.Write(b[:n])
		r.Received += int64(n)
	}
	if e == nil && n != len(b) {
		e = io.ErrShortWrite
	}
	return e
}
func (r *Receiver) Commit(size int64, digest string) error {
	if r.Received != size || hex.EncodeToString(r.h.Sum(nil)) != digest {
		return errors.New("size/SHA-256 mismatch")
	}
	if e := r.f.Close(); e != nil {
		return e
	}
	r.f = nil
	var e error
	if r.overwrite {
		e = os.Rename(r.temp, r.target)
	} else {
		e = os.Link(r.temp, r.target)
	}
	if e != nil {
		return e
	}
	r.Committed = true
	// A failed temp unlink cannot undo the already committed complete file.
	_ = os.Remove(r.temp)
	return nil
}
func (r *Receiver) Close() {
	if r == nil {
		return
	}
	if r.f != nil {
		r.f.Close()
		r.f = nil
	}
	_ = os.Remove(r.temp)
}
