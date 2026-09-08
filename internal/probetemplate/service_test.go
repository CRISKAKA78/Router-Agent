package probetemplate

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestPersistenceConflictsAndDeletion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "templates.json")
	s, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	if other, e := Open(path); e == nil {
		other.Close()
		t.Fatal("second writer admitted")
	}
	input := Input{Name: "路由器", Properties: map[string]Property{"serial": {Name: "序列号", Command: "nvram get SN"}}}
	v, e := s.Put("", 0, input)
	if e != nil || v.Version != 1 || v.Properties["serial"].Timeout != 5 {
		t.Fatal(v, e)
	}
	input.Properties["serial"] = Property{}
	v.Properties["serial"] = Property{}
	got, e := s.Resolve("", "路由器")
	if e != nil || got.Properties["serial"].Command != "nvram get SN" {
		t.Fatal(got, e)
	}
	if _, e = s.Put("", 0, Input{got.Name, got.Properties}); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	success := make(chan bool, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := s.Put(got.ID, 1, Input{got.Name, got.Properties})
			success <- e == nil
		}()
	}
	wg.Wait()
	if (<-success) == (<-success) {
		t.Fatal("expected one version winner")
	}
	s.Close()
	s, e = Open(path)
	if e != nil {
		t.Fatal(e)
	}
	got, e = s.Resolve(got.ID, "")
	if e != nil || got.Version != 2 {
		t.Fatal(got, e)
	}
	if e = s.Delete(got.ID, 1); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	if e = s.Delete(got.ID, 2); e != nil {
		t.Fatal(e)
	}
	s.Close()
	s, e = Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if _, e = s.Resolve(got.ID, ""); !errors.Is(e, ErrNotFound) {
		t.Fatal(e)
	}
	replacement, e := s.Put("", 0, Input{got.Name, got.Properties})
	if e != nil || replacement.ID == got.ID {
		t.Fatal(replacement, e)
	}
}
func TestRejectCorruptAndMissingCatalog(t *testing.T) {
	for _, body := range []string{`{"schema_version":1,"schema_version":1,"templates":[]}`, `{"schema_version":2,"templates":[]}`, `{"schema_version":1,"templates":null}`} {
		path := filepath.Join(t.TempDir(), "templates.json")
		os.WriteFile(path, []byte(body), 0600)
		if s, e := Open(path); e == nil {
			s.Close()
			t.Fatal("accepted", body)
		}
	}
	path := filepath.Join(t.TempDir(), "templates.json")
	s, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	s.Close()
	os.Remove(path)
	if s, e = Open(path); e == nil {
		s.Close()
		t.Fatal("missing established catalog silently reset")
	}
}
func TestValidation(t *testing.T) {
	for _, key := range []string{"device_id", "arch", "boot_id", "probe_version", "capabilities", "template", "attributes", "collection_errors", "Bad", "a-b", ""} {
		if Validate(Input{"t", map[string]Property{key: {Name: "x", Command: "true", Timeout: 5}}}) == nil {
			t.Fatal(key)
		}
	}
	s, e := Open(filepath.Join(t.TempDir(), "t.json"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	for _, p := range []Property{{Name: "x", Command: "true", Timeout: 31}, {Name: "x", Command: " "}, {Name: "x", Command: "a\x00b"}} {
		if _, e = s.Put("", 0, Input{"t", map[string]Property{"custom": p}}); !errors.Is(e, ErrInvalid) {
			t.Fatal(p, e)
		}
	}
	// Persistence failure must not publish an in-memory template.
	s.path = filepath.Join(t.TempDir(), "missing", "catalog.json")
	if _, e = s.Put("", 0, Input{"t", map[string]Property{"custom": {Name: "x", Command: "true"}}}); e == nil {
		t.Fatal("save unexpectedly succeeded")
	}
	if list, _ := s.List(); len(list) != 0 {
		t.Fatal(list)
	}
}
