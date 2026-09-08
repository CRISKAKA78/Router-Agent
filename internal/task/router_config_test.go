package task

import (
	"bytes"
	"routerprobe/internal/routerconfig"
	"testing"
)

func TestRouterConfigImmutableSpec(t *testing.T) {
	s := NewService()
	key, value := "SN", "original"
	p := routerconfig.Params{Backend: "nvram", Operation: "set", Key: &key, Value: &value}
	spec, err := s.NewRouterConfig("device", p, 5)
	if err != nil {
		t.Fatal(err)
	}
	expected := append([]byte(nil), spec.Params...)
	value = "changed"
	spec.Params[0] = '!'
	snapshot, err := s.Snapshot(spec.ID)
	if err != nil || !bytes.Equal(snapshot.Spec.Params, expected) {
		t.Fatal("spec is mutable", snapshot, err)
	}
	snapshot.Spec.Params[0] = '!'
	again, _ := s.Snapshot(spec.ID)
	if !bytes.Equal(again.Spec.Params, expected) {
		t.Fatal("snapshot is mutable")
	}
	for _, timeout := range []uint32{0, 31} {
		if _, err = s.NewRouterConfig("device", p, timeout); err == nil {
			t.Fatal("invalid timeout", timeout)
		}
	}
	if _, err = s.NewRouterConfig("", p, 5); err == nil {
		t.Fatal("empty device accepted")
	}
}
