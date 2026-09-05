package filetransfer

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"routerprobe/internal/protocol"
	"routerprobe/internal/task"
)

func TestReceiverSuccessfulPublication(t *testing.T) {
	for _, overwrite := range []bool{false, true} {
		for _, data := range [][]byte{nil, {0, 1, 128, 255}} {
			t.Run(fmt.Sprintf("overwrite=%t/size=%d", overwrite, len(data)), func(t *testing.T) {
				target := filepath.Join(t.TempDir(), "target")
				if overwrite {
					if err := os.WriteFile(target, []byte("old"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				r, err := NewReceiver(target, overwrite)
				if err != nil {
					t.Fatal(err)
				}
				defer r.Close()
				if err = r.Write(data); err != nil {
					t.Fatal(err)
				}
				digest := sha256.Sum256(data)
				if err = r.Commit(int64(len(data)), fmt.Sprintf("%x", digest)); err != nil {
					t.Fatal(err)
				}
				r.Close()
				got, err := os.ReadFile(target)
				if err != nil || !bytes.Equal(got, data) || !r.Committed {
					t.Fatal("publication", got, err)
				}
				if _, err = os.Stat(r.temp); !os.IsNotExist(err) {
					t.Fatal("temporary file retained", err)
				}
			})
		}
	}
}

func TestEndedTransferReleasesMailbox(t *testing.T) {
	s := New(task.NewService())
	r := &record{events: make(chan event, 16), cancel: make(chan struct{}), t: Transport{Done: make(chan struct{})}}
	r.events <- event{ack: &task.Ack{Accepted: false, State: "rejected"}}
	for i := 0; i < 15; i++ {
		r.events <- event{frame: protocol.Frame{Payload: make([]byte, 64*1024)}}
	}
	var writers sync.WaitGroup
	for i := 0; i < 20; i++ {
		writers.Add(1)
		go func() { defer writers.Done(); _ = r.enqueue(event{}) }()
	}
	if err := s.run(r); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { writers.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("mailbox sender leaked")
	}
	r.eventMu.Lock()
	retained := r.events != nil
	r.eventMu.Unlock()
	if retained {
		t.Fatal("completed identity retains file payload mailbox")
	}
	if err := r.enqueue(event{}); err != nil {
		t.Fatal(err)
	}
}
