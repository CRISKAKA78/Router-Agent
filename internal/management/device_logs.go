package management

import (
	"context"
	"io"
	"os"
	"routerprobe/internal/devicelog"
	"routerprobe/internal/repository"
	"strings"
)

func (s *Server) DeviceLogs() *devicelog.Service { return s.logs }
func (s *Server) PreviewLog(ctx context.Context, id string) (devicelog.Preview, error) {
	var result devicelog.Preview
	e := s.Files().ReadContent(ctx, id, func(asset repository.Asset, reader io.ReadSeeker) error {
		var err error
		result, err = devicelog.ReadPreview(ctx, asset.Name, reader)
		return err
	})
	return result, e
}

// ReadContent cannot reenter Repository: stage validated text, release its read
// lock, then import through the existing asset service with a fresh identity.
func (s *Server) DecodeLog(ctx context.Context, id string) (repository.Asset, error) {
	f, e := os.CreateTemp("", "router-log-text-*.txt")
	if e != nil {
		return repository.Asset{}, e
	}
	defer os.Remove(f.Name())
	defer f.Close()
	name := ""
	e = s.Files().ReadContent(ctx, id, func(a repository.Asset, r io.ReadSeeker) error {
		name = strings.TrimSuffix(a.Name, ".gz")
		return devicelog.CopyText(ctx, a.Name, r, f)
	})
	if e != nil {
		return repository.Asset{}, e
	}
	if _, e = f.Seek(0, io.SeekStart); e != nil {
		return repository.Asset{}, e
	}
	return s.Files().Import(ctx, name, f)
}
