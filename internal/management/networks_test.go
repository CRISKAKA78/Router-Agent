package management

import (
	"context"
	"errors"
	"routerprobe/internal/filetransfer"
	"routerprobe/internal/overlay"
	"testing"
	"time"
)

func TestNetworkPackageUploadRelease(t *testing.T) {
	// A real successful server-to-device upload has no local download commit.
	calls := 0
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := waitNetworkPackageRelease(ctx, func() (filetransfer.Snapshot, error) {
		calls++
		return filetransfer.Snapshot{Released: calls >= 2, Committed: false}, nil
	})
	if err != nil || calls != 2 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestNetworkPackageReleaseUncertain(t *testing.T) {
	for _, tc := range []struct {
		name string
		snap filetransfer.Snapshot
		err  error
	}{
		{"transfer failure", filetransfer.Snapshot{Released: true, Error: "failed"}, nil},
		{"snapshot unavailable", filetransfer.Snapshot{}, errors.New("missing")},
		{"not released", filetransfer.Snapshot{}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			err := waitNetworkPackageRelease(ctx, func() (filetransfer.Snapshot, error) { return tc.snap, tc.err })
			if !errors.Is(err, overlay.ErrUncertain) {
				t.Fatal(err)
			}
		})
	}
}
