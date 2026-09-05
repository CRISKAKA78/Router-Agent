package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	s, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func importTest(t *testing.T, s *Store, name, content string) Asset {
	t.Helper()
	a, e := s.Files().Import(context.Background(), name, strings.NewReader(content))
	if e != nil {
		t.Fatal(e)
	}
	return a
}
func artifactSpec(a Asset) ArtifactSpec {
	return ArtifactSpec{AssetID: a.ID, Platform: "linux", Mode: "0755", Rules: Rules{Arch: []string{"any"}, Libc: []string{"any"}}}
}
func toolTest(t *testing.T, s *Store) (Tool, Version, Asset) {
	t.Helper()
	a := importTest(t, s, "工具", "content")
	tool, e := s.Tools().Create("tool", "description")
	if e != nil {
		t.Fatal(e)
	}
	v, e := s.Tools().Publish(tool.ID, "v1", []ArtifactSpec{artifactSpec(a)})
	if e != nil {
		t.Fatal(e)
	}
	return tool, v, a
}

func TestRepositoryPersistenceIdentityAndArchive(t *testing.T) {
	s := openTest(t)
	tool, v, a := toolTest(t, s)
	b := importTest(t, s, "different name", "content")
	if a.ID == b.ID || a.SHA256 != b.SHA256 || !validID(tool.ID) || !validID(v.Artifacts[0].ID) {
		t.Fatal("identity/dedup")
	}
	blobs, e := filepath.Glob(filepath.Join(s.root, "blobs", "sha256", "*", "*"))
	if e != nil || len(blobs) != 1 {
		t.Fatal(blobs, e)
	}
	if e = s.Files().Archive(a.ID); !errors.Is(e, ErrReferenced) {
		t.Fatal(e)
	}
	same, e := s.Tools().Publish(tool.ID, "v1", []ArtifactSpec{artifactSpec(a)})
	if e != nil || !reflect.DeepEqual(same, v) {
		t.Fatal("idempotent version", e)
	}
	changed := artifactSpec(a)
	changed.Mode = "0600"
	if _, e = s.Tools().Publish(tool.ID, "v1", []ArtifactSpec{changed}); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	v2, e := s.Tools().Publish(tool.ID, "V1", []ArtifactSpec{artifactSpec(a)})
	if e != nil || v2.Artifacts[0].ID == v.Artifacts[0].ID {
		t.Fatal("global artifact identity", e)
	}
	if e = s.Tools().ArchiveVersion(tool.ID, "v1"); e != nil {
		t.Fatal(e)
	}
	if e = s.Files().Archive(a.ID); !errors.Is(e, ErrReferenced) {
		t.Fatal(e)
	}
	if e = s.Tools().Archive(tool.ID); e != nil {
		t.Fatal(e)
	}
	if e = s.Tools().WithArtifact(context.Background(), tool.ID, "V1", v2.Artifacts[0].ID, func(Artifact, Asset, string) error { t.Fatal("archived admitted"); return nil }); !errors.Is(e, ErrArchived) {
		t.Fatal(e)
	}
	if e = s.Tools().ArchiveVersion(tool.ID, "V1"); e != nil {
		t.Fatal(e)
	}
	if e = s.Files().Archive(a.ID); e != nil {
		t.Fatal(e)
	}
	if e = s.Files().WithAsset(context.Background(), a.ID, func(Asset, string) error { return nil }); !errors.Is(e, ErrArchived) {
		t.Fatal(e)
	}
	root := s.root
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	s2, e := Open(root)
	if e != nil {
		t.Fatal(e)
	}
	defer s2.Close()
	got, e := s2.Tools().Publish(tool.ID, "v1", []ArtifactSpec{artifactSpec(a)})
	if e != nil || got.Artifacts[0].ID != v.Artifacts[0].ID || !got.Archived {
		t.Fatal("archived identity not retained", e)
	}
	if _, e = s2.Tools().Publish(tool.ID, "new", []ArtifactSpec{artifactSpec(b)}); !errors.Is(e, ErrArchived) {
		t.Fatal(e)
	}
	list, e := s2.Files().List(false)
	if e != nil || len(list) != 1 || list[0].ID != b.ID {
		t.Fatal(list, e)
	}
	all, _ := s2.Files().List(true)
	if len(all) != 2 {
		t.Fatal(all)
	}
	if _, e = os.Stat(s2.blobPath(a.SHA256)); e != nil {
		t.Fatal("archive removed content", e)
	}
}

func TestRepositoryAtomicCatalogFailureAndCorruptContent(t *testing.T) {
	s := openTest(t)
	_, _, a := toolTest(t, s)
	before, e := os.ReadFile(s.catalogPath())
	if e != nil {
		t.Fatal(e)
	}
	failure := errors.New("publish failed")
	s.publish = func(string, string) error { return failure }
	if _, e = s.Tools().Create("new", "description"); !errors.Is(e, failure) {
		t.Fatal(e)
	}
	if e = s.Files().Archive(a.ID); !errors.Is(e, ErrReferenced) {
		t.Fatal(e)
	}
	if _, e = s.Files().Import(context.Background(), "orphan", strings.NewReader("new bytes")); !errors.Is(e, failure) {
		t.Fatal(e)
	}
	after, _ := os.ReadFile(s.catalogPath())
	if !bytes.Equal(before, after) {
		t.Fatal("failed write changed catalog")
	}
	tools, _ := s.Tools().List(true)
	assets, _ := s.Files().List(true)
	if len(tools) != 1 || len(assets) != 1 {
		t.Fatal("failed write published memory")
	}
	leftovers, e := s.Leftovers()
	if e != nil || len(leftovers) != 1 {
		t.Fatal(leftovers, e)
	}
	s.publish = replaceFile
	if e = os.WriteFile(s.blobPath(a.SHA256), []byte("CONTENT"), 0600); e != nil {
		t.Fatal(e)
	}
	if e = s.Files().WithAsset(context.Background(), a.ID, func(Asset, string) error { t.Fatal("corrupt blob admitted"); return nil }); !errors.Is(e, ErrIntegrity) {
		t.Fatal(e)
	}
	if _, e = s.Files().Import(context.Background(), "duplicate", strings.NewReader("content")); !errors.Is(e, ErrIntegrity) {
		t.Fatal(e)
	}
	assets, _ = s.Files().List(true)
	if len(assets) != 1 {
		t.Fatal("corrupt dedup created asset")
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func TestRepositoryImportFailuresAndEmptyFile(t *testing.T) {
	s := openTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, test := range []struct {
		name   string
		ctx    context.Context
		reader io.Reader
	}{{"cancel", ctx, strings.NewReader("abc")}, {"read", context.Background(), failingReader{}}} {
		if _, e := s.Files().Import(test.ctx, test.name, test.reader); e == nil {
			t.Fatal(test.name)
		}
	}
	if _, e := s.Files().ImportVerified(context.Background(), "wrong", strings.NewReader("x"), 1, strings.Repeat("0", 64)); !errors.Is(e, ErrIntegrity) {
		t.Fatal(e)
	}
	a := importTest(t, s, "../../display-name-only", "")
	if a.Size != 0 || a.ID == "" {
		t.Fatal(a)
	}
	if e := s.Files().WithAsset(context.Background(), a.ID, func(a Asset, p string) error {
		if !strings.HasPrefix(p, s.root) {
			t.Fatal(p)
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	entries, e := os.ReadDir(filepath.Join(s.root, "staging"))
	if e != nil || len(entries) != 0 {
		t.Fatal(entries, e)
	}
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Files().List(true); !errors.Is(e, ErrClosed) {
		t.Fatal(e)
	}
}

func TestRepositoryCatalogRejectsInvalidState(t *testing.T) {
	for _, which := range []string{"schema", "duplicate-key", "trailing", "reference", "artifact-global", "duplicate-tool", "unknown-field", "missing-blob", "missing-catalog", "null-rules"} {
		t.Run(which, func(t *testing.T) {
			s := openTest(t)
			tool, v, a := toolTest(t, s)
			root := s.root
			c := cloneCatalog(s.data)
			path := s.catalogPath()
			s.Close()
			switch which {
			case "schema":
				c.SchemaVersion = 2
			case "reference":
				c.Versions[0].Artifacts[0].AssetID = tool.ID
			case "artifact-global":
				second := cloneVersion(v)
				second.Version = "v2"
				c.Versions = append(c.Versions, second)
			case "duplicate-tool":
				c.Tools = append(c.Tools, c.Tools[0])
			case "null-rules":
				c.Versions[0].Artifacts[0].Rules.Arch = nil
			case "missing-blob":
				os.Remove(s.blobPath(a.SHA256))
			case "missing-catalog":
				os.Remove(path)
			}
			b, _ := json.Marshal(c)
			switch which {
			case "duplicate-key":
				b = append([]byte(`{"schema_version":1,`), b[1:]...)
			case "trailing":
				b = append(b, []byte(` {}`)...)
			case "unknown-field":
				b = append([]byte(`{"surprise":true,`), b[1:]...)
			}
			if which != "missing-catalog" {
				if e := os.WriteFile(path, b, 0600); e != nil {
					t.Fatal(e)
				}
			}
			r, e := Open(root)
			if e == nil {
				r.Close()
				t.Fatal("corrupt catalog accepted")
			}
		})
	}
}

func TestRepositoryCopiesConcurrentIDsAndNormalization(t *testing.T) {
	s := openTest(t)
	a := importTest(t, s, "asset", "x")
	tool, e := s.Tools().Create("tool", "")
	if e != nil {
		t.Fatal(e)
	}
	spec := artifactSpec(a)
	spec.Rules.Arch = []string{"arm64", "aarch64"}
	spec.Rules.Libc = []string{"uClibc"}
	v, e := s.Tools().Publish(tool.ID, "stable", []ArtifactSpec{spec})
	if e != nil {
		t.Fatal(e)
	}
	spec.Rules.Arch[0] = "corrupt"
	v.Artifacts[0].Rules.Arch[0] = "corrupt"
	v, e = s.Tools().Version(tool.ID, "stable")
	if e != nil || v.Artifacts[0].Rules.Arch[0] != "aarch64" || v.Artifacts[0].Rules.Libc[0] != "uclibc" {
		t.Fatal(v, e)
	}
	var wg sync.WaitGroup
	ids := make(chan string, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a, e := s.Files().Import(context.Background(), "same", strings.NewReader("x"))
			if e != nil {
				t.Error(e)
				return
			}
			ids <- a.ID
			s.Files().List(true)
			s.Tools().Version(tool.ID, "stable")
		}()
	}
	wg.Wait()
	close(ids)
	seen := map[string]bool{}
	for id := range ids {
		if seen[id] {
			t.Fatal("reused ID")
		}
		seen[id] = true
	}
	if len(seen) != 12 {
		t.Fatal(seen)
	}
	s.Close()
	r, e := Open(s.root)
	if e != nil {
		t.Fatal(e)
	}
	r.Close()
}

func TestRepositoryDirectoryLock(t *testing.T) {
	s := openTest(t)
	if other, e := Open(s.root); e == nil {
		other.Close()
		t.Fatal("second owner admitted")
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestRepositoryLockChild$")
	cmd.Env = append(os.Environ(), "RMP_REPOSITORY_LOCK_CHILD="+s.root)
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("cross-process lock: %v %s", e, b)
	}
	if e := s.Close(); e != nil {
		t.Fatal(e)
	}
	r, e := Open(s.root)
	if e != nil {
		t.Fatal("lock not released", e)
	}
	r.Close()
}
func TestRepositoryLockChild(t *testing.T) {
	path := os.Getenv("RMP_REPOSITORY_LOCK_CHILD")
	if path == "" {
		return
	}
	if s, e := Open(path); e == nil {
		s.Close()
		t.Fatal("child acquired held lock")
	}
}

func TestRepositoryArchiveWaitsForAdmittedUse(t *testing.T) {
	s := openTest(t)
	a := importTest(t, s, "asset", "bytes")
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- s.Files().WithAsset(context.Background(), a.ID, func(Asset, string) error { close(entered); <-release; return nil })
	}()
	<-entered
	archived := make(chan error, 1)
	go func() { archived <- s.Files().Archive(a.ID) }()
	select {
	case e := <-archived:
		t.Fatal("archive raced admitted use", e)
	default:
	}
	close(release)
	if e := <-done; e != nil {
		t.Fatal(e)
	}
	if e := <-archived; e != nil {
		t.Fatal(e)
	}
}

func TestRepositoryMovedDirectoryAndMissingCatalog(t *testing.T) {
	parent := t.TempDir()
	original := filepath.Join(parent, "original")
	moved := filepath.Join(parent, "moved")
	s, e := Open(original)
	if e != nil {
		t.Fatal(e)
	}
	tool, v, a := toolTest(t, s)
	s.Close()
	if e = os.Rename(original, moved); e != nil {
		t.Fatal(e)
	}
	s, e = Open(moved)
	if e != nil {
		t.Fatal(e)
	}
	got, e := s.Tools().Version(tool.ID, v.Version)
	if e != nil || !reflect.DeepEqual(got, v) {
		t.Fatal(got, e)
	}
	if e = s.Files().WithAsset(context.Background(), a.ID, func(_ Asset, p string) error {
		if !strings.HasPrefix(p, moved) {
			t.Fatal(p)
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	s.Close()
	// Even a catalog with only tools has a persistent initialization marker.
	root := t.TempDir()
	s, e = Open(root)
	if e != nil {
		t.Fatal(e)
	}
	s.Tools().Create("only tool", "")
	s.Close()
	os.Remove(s.catalogPath())
	if s, e = Open(root); e == nil {
		s.Close()
		t.Fatal("missing tools-only catalog silently reset")
	}
}

func TestRepositoryInputValidationAndQueryIsolation(t *testing.T) {
	s := openTest(t)
	tool, v, a := toolTest(t, s)
	for _, change := range []func(*ArtifactSpec){func(a *ArtifactSpec) { a.Platform = "windows" }, func(a *ArtifactSpec) { a.Mode = "755" }, func(a *ArtifactSpec) { a.Mode = "0899" }, func(a *ArtifactSpec) { a.AssetID = "not-id" }, func(a *ArtifactSpec) { a.Rules.Libc = nil }, func(a *ArtifactSpec) { a.Rules.Arch = []string{"any", "arm"} }} {
		spec := artifactSpec(a)
		change(&spec)
		if _, e := s.Tools().Publish(tool.ID, "invalid", []ArtifactSpec{spec}); e == nil {
			t.Fatal("invalid artifact accepted")
		}
	}
	if _, e := s.Tools().Create("", ""); e == nil {
		t.Fatal("empty tool name")
	}
	if _, e := s.Tools().Publish(tool.ID, "", []ArtifactSpec{artifactSpec(a)}); e == nil {
		t.Fatal("empty version")
	}
	if _, e := s.Tools().Version("missing", "1"); !errors.Is(e, ErrNotFound) {
		t.Fatal(e)
	}
	if _, e := s.Tools().Versions("missing", true); !errors.Is(e, ErrNotFound) {
		t.Fatal(e)
	}
	if e := s.Files().Archive("missing"); !errors.Is(e, ErrNotFound) {
		t.Fatal(e)
	}
	all, e := s.Tools().Versions(tool.ID, true)
	if e != nil {
		t.Fatal(e)
	}
	all[0].Artifacts[0].Rules.Libc[0] = "mutated"
	actual, e := s.Tools().Version(tool.ID, v.Version)
	if e != nil || !reflect.DeepEqual(actual, v) {
		t.Fatal("nested snapshot alias", e)
	}
	other, e := s.Tools().Create("second", "")
	if e != nil {
		t.Fatal(e)
	}
	second, e := s.Tools().Publish(other.ID, "v1", []ArtifactSpec{artifactSpec(a)})
	if e != nil || second.Artifacts[0].ID == v.Artifacts[0].ID {
		t.Fatal("cross-tool artifact collision", e)
	}
}
