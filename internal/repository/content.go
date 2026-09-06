package repository

import (
	"context"
	"io"
	"os"
)

// Revision tracks committed catalog changes, including imports and archives.
func (f *FileService) Revision() uint64 { return f.s.revision.Load() }

// ReadContent keeps local paths inside the Repository service. The callback must
// not reenter Repository, as with WithAsset. Bytes are streamed with fixed buffers.
func (f *FileService) ReadContent(ctx context.Context, id string, use func(Asset, io.ReadSeeker) error) error {
	return f.WithAsset(ctx, id, func(a Asset, path string) error {
		r, err := os.Open(path)
		if err != nil {
			return err
		}
		defer r.Close()
		return use(a, r)
	})
}
