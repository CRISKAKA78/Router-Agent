package management

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"routerprobe/internal/device"
	"routerprobe/internal/filetransfer"
	"routerprobe/internal/gateway"
	"routerprobe/internal/repository"
	"routerprobe/internal/task"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeTasks struct {
	uploads      int
	q            filetransfer.UploadRequest
	downloadPath string
	id           string
	err          error
	file         filetransfer.Snapshot
	snapshot     task.Snapshot
	beforeUpload func()
}

func (f *fakeTasks) CreateUpload(ctx context.Context, id string, q filetransfer.UploadRequest) (string, error) {
	f.uploads++
	f.q = q
	if f.beforeUpload != nil {
		f.beforeUpload()
	}
	return f.id, f.err
}
func (f *fakeTasks) CreateDownload(ctx context.Context, id string, q filetransfer.DownloadRequest) (string, error) {
	f.downloadPath = q.TargetPath
	f.file.LocalPath = q.TargetPath
	if f.id != "" {
		if e := os.WriteFile(q.TargetPath, []byte("download bytes"), 0600); e != nil {
			return "", e
		}
	}
	return f.id, f.err
}
func (f *fakeTasks) FileSnapshot(string) (filetransfer.Snapshot, error) { return f.file, nil }
func (f *fakeTasks) TaskSnapshot(string) (task.Snapshot, error)         { return f.snapshot, nil }
func (f *fakeTasks) WaitTaskResult(context.Context, string) (task.Result, error) {
	return *f.snapshot.Result, nil
}
func (f *fakeTasks) ResendTask(context.Context, string) error { return nil }
func setup(t *testing.T) (*Service, *device.Service, *fakeTasks, repository.Tool, repository.Version) {
	t.Helper()
	r, e := repository.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { r.Close() })
	d, _ := device.New(0)
	d.Publish(device.Registration{DeviceID: "device", Arch: "aarch64", Capabilities: []string{"file", "exec"}}, "session", time.Now())
	f := &fakeTasks{id: "task", file: filetransfer.Snapshot{TaskID: "task", TransferID: "transfer"}}
	s := NewService(r, d, f)
	a, e := s.Files().Import(context.Background(), "tool", strings.NewReader("bytes"))
	if e != nil {
		t.Fatal(e)
	}
	tool, e := s.Tools().Create("tool", "")
	if e != nil {
		t.Fatal(e)
	}
	v, e := s.Tools().Publish(tool.ID, "1", []repository.ArtifactSpec{{AssetID: a.ID, Platform: "linux", Mode: "0755", Rules: repository.Rules{Arch: []string{"arm64"}, Libc: []string{"any"}}}})
	if e != nil {
		t.Fatal(e)
	}
	return s, d, f, tool, v
}
func request(tool repository.Tool) DeployRequest {
	return DeployRequest{DeviceID: "device", ToolID: tool.ID, Version: "1", RemotePath: "/tmp/tool", Timeout: time.Second}
}
func TestDeployBindsAssetSessionAndUncertainIdentity(t *testing.T) {
	s, _, f, tool, v := setup(t)
	q := request(tool)
	f.err = gateway.ErrDispatchUncertain
	op, e := s.Deploy(context.Background(), q)
	if !errors.Is(e, gateway.ErrDispatchUncertain) || op.TaskID != "task" || op.TransferID != "transfer" || op.ArtifactID != v.Artifacts[0].ID {
		t.Fatal(op, e)
	}
	if f.uploads != 1 || f.q.ExpectedSessionID != "session" || f.q.Expected == nil || f.q.Expected.Size != 5 || f.q.Mode != "0755" {
		t.Fatal(f.q)
	}
	saved, e := s.Operation(op.TaskID)
	if e != nil || saved != op {
		t.Fatal(saved, e)
	}
	s.ResendTask(context.Background(), op.TaskID)
	if f.uploads != 1 {
		t.Fatal("resend opened another file task")
	}
	f.id = ""
	f.err = errors.New("before send")
	if op, e = s.Deploy(context.Background(), q); e == nil || op.TaskID != "" {
		t.Fatal(op, e)
	}
}
func TestCompatibilitySelectionAndAdmission(t *testing.T) {
	s, d, f, tool, v := setup(t)
	q := request(tool)
	a := v.Artifacts[0]
	spec := repository.ArtifactSpec{AssetID: a.AssetID, Platform: a.Platform, Mode: a.Mode, Rules: a.Rules}
	constrained := spec
	constrained.Rules.Libc = []string{"uclibc"}
	s.Tools().Publish(tool.ID, "unknown", []repository.ArtifactSpec{constrained})
	q.Version = "unknown"
	candidates, e := s.Compatibility("device", tool.ID, "unknown")
	if e != nil || candidates[0].Match.Status != repository.Unknown {
		t.Fatal(candidates, e)
	}
	if _, e = s.Deploy(context.Background(), q); !errors.Is(e, ErrIncompatible) || f.uploads != 0 {
		t.Fatal(e)
	}
	two, e := s.Tools().Publish(tool.ID, "two", []repository.ArtifactSpec{spec, spec})
	if e != nil {
		t.Fatal(e)
	}
	q.Version = "two"
	if _, e = s.Deploy(context.Background(), q); !errors.Is(e, ErrAmbiguous) {
		t.Fatal(e)
	}
	q.ArtifactID = two.Artifacts[1].ID
	if _, e = s.Deploy(context.Background(), q); e != nil {
		t.Fatal(e)
	}
	q.ArtifactID = v.Artifacts[0].ID
	if _, e = s.Deploy(context.Background(), q); !errors.Is(e, ErrIncompatible) {
		t.Fatal("artifact from another version", e)
	}
	d.End("device", "session", device.Disconnected, time.Now())
	q = request(tool)
	if _, e = s.Deploy(context.Background(), q); !errors.Is(e, ErrOffline) {
		t.Fatal(e)
	}
	if _, e = s.Compatibility("device", tool.ID, "1"); e != nil {
		t.Fatal("offline compatibility query", e)
	}
	d.Publish(device.Registration{DeviceID: "device", Arch: "aarch64"}, "new", time.Now())
	if _, e = s.Deploy(context.Background(), q); !errors.Is(e, ErrIncompatible) {
		t.Fatal(e)
	}
	d.Publish(device.Registration{DeviceID: "device", Arch: "aarch64", Capabilities: []string{"file"}}, "third", time.Now())
	s.Tools().Archive(tool.ID)
	if _, e = s.Deploy(context.Background(), q); !errors.Is(e, repository.ErrArchived) {
		t.Fatal(e)
	}
}
func configureDownload(f *fakeTasks) {
	h := sha256.Sum256([]byte("download bytes"))
	f.file.Committed = true
	f.file.Released = true
	f.file.Size = 14
	f.file.SHA256 = hex.EncodeToString(h[:])
	f.snapshot = task.Snapshot{State: task.StateFailed, Result: &task.Result{TaskID: "task", Status: "failed"}}
}
func TestDownloadImportCommitDistinctFromResultAndIdempotent(t *testing.T) {
	s, _, f, _, _ := setup(t)
	configureDownload(f)
	f.err = gateway.ErrDispatchUncertain
	op, e := s.Download(context.Background(), DownloadRequest{DeviceID: "device", RemotePath: "/tmp/file", Name: "result", Timeout: time.Second})
	if !errors.Is(e, gateway.ErrDispatchUncertain) || op.TaskID == "" {
		t.Fatal(op, e)
	}
	var wg sync.WaitGroup
	ids := make(chan string, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := s.CompleteDownload(context.Background(), op.TaskID)
			if e != nil {
				t.Error(e)
				return
			}
			if r.Task.State != task.StateFailed || !r.File.Committed {
				t.Error("commit rewritten as task success")
			}
			ids <- r.Asset.ID
		}()
	}
	wg.Wait()
	close(ids)
	first := ""
	for id := range ids {
		if first == "" {
			first = id
		}
		if first != id {
			t.Fatal("duplicate import identity")
		}
	}
	if first == "" {
		t.Fatal("no imports")
	}
	if _, e = os.Stat(f.downloadPath); !os.IsNotExist(e) {
		t.Fatal("staging not removed", e)
	}
	s.Files().Archive(first)
	again, e := s.CompleteDownload(context.Background(), op.TaskID)
	if e != nil || again.Asset.ID != first || !again.Asset.Archived {
		t.Fatal(again, e)
	}
	if e = s.CleanupDownload(op.TaskID); e != nil {
		t.Fatal(e)
	}
}
func TestDownloadImportRejectsIncompleteAndMutation(t *testing.T) {
	s, _, f, _, _ := setup(t)
	configureDownload(f)
	op, e := s.Download(context.Background(), DownloadRequest{DeviceID: "device", RemotePath: "/tmp/file", Name: "result", Timeout: time.Second})
	if e != nil {
		t.Fatal(e)
	}
	f.file.Released = false
	if _, e = s.CompleteDownload(context.Background(), op.TaskID); !errors.Is(e, ErrNotCommitted) {
		t.Fatal(e)
	}
	if e = s.CleanupDownload(op.TaskID); !errors.Is(e, ErrNotCommitted) {
		t.Fatal(e)
	}
	f.file.Released = true
	if e = s.CleanupDownload(op.TaskID); !errors.Is(e, ErrNotCommitted) {
		t.Fatal("unimported commit discarded", e)
	}
	os.WriteFile(f.downloadPath, []byte("mutated bytes!"), 0600)
	if _, e = s.CompleteDownload(context.Background(), op.TaskID); !errors.Is(e, repository.ErrIntegrity) {
		t.Fatal(e)
	}
	f.file.Committed = false
	if _, e = s.CompleteDownload(context.Background(), op.TaskID); !errors.Is(e, ErrNotCommitted) {
		t.Fatal(e)
	}
	if e = s.CleanupDownload(op.TaskID); e != nil {
		t.Fatal(e)
	}
	f.id = ""
	f.err = errors.New("dispatch failed")
	if _, e = s.Download(context.Background(), DownloadRequest{DeviceID: "device"}); e == nil {
		t.Fatal("failed dispatch")
	}
	if _, e = os.Stat(filepath.Dir(f.downloadPath)); !os.IsNotExist(e) {
		t.Fatal("failed dispatch staging leaked", e)
	}
}
func TestManagementServerRepositoryLifecycle(t *testing.T) {
	dir := t.TempDir()
	s, e := New(Config{RepositoryDirectory: dir})
	if e != nil {
		t.Fatal(e)
	}
	a, e := s.Files().Import(context.Background(), "persist", strings.NewReader("data"))
	if e != nil {
		t.Fatal(e)
	}
	s.Close()
	s, e = New(Config{RepositoryDirectory: dir})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	b, e := s.Files().Get(a.ID)
	if e != nil || b != a {
		t.Fatal(b, e)
	}
	if len(s.Devices().List()) != 0 {
		t.Fatal("device restored")
	}
}
