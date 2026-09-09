package probetemplate

import (
	"encoding/json"
	"testing"
)

func TestStorageVisibilityRoundTrip(t *testing.T) {
	var input Input
	if err := json.Unmarshal([]byte(`{"name":"Storage","properties":{},"presentation":{"storage_visible":false}}`), &input); err != nil {
		t.Fatal(err)
	}
	path := t.TempDir() + "/catalog.json"
	service, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := service.Put("", 0, input)
	if err != nil {
		t.Fatal(err)
	}
	copy := CopyPresentation(saved.Presentation)
	if copy.StorageVisible == nil || *copy.StorageVisible {
		t.Fatal("visibility lost")
	}
	*copy.StorageVisible = true
	if *saved.Presentation.StorageVisible {
		t.Fatal("copy aliased source")
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	loaded, err := reopened.Resolve(saved.ID, "")
	if err != nil || loaded.Presentation.StorageVisible == nil || *loaded.Presentation.StorageVisible {
		t.Fatal("persisted visibility lost", err)
	}
}
