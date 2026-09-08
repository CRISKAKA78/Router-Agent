package gateway

import (
	"encoding/json"
	"testing"
)

func TestRegistrationCollectionValidation(t *testing.T) {
	base := map[string]any{"device_id": "id", "probe_version": "v", "arch": "arm", "boot_id": "b", "capabilities": []string{}, "template": map[string]any{"template_id": "t", "name": "template", "version": 1}, "attributes": map[string]any{"signal": map[string]any{"name": "信号", "value": "90"}}, "collection_errors": map[string]any{"model": map[string]any{"name": "型号", "reason": "empty"}}}
	b, _ := json.Marshal(base)
	v, e := parseRegister(b)
	if e != nil || v.Attributes["signal"].Value != "90" {
		t.Fatal(v, e)
	}
	base["model"] = "also success"
	b, _ = json.Marshal(base)
	if _, e = parseRegister(b); e == nil {
		t.Fatal("failure/success conflict")
	}
	delete(base, "model")
	base["attributes"] = map[string]any{"arch": map[string]any{"name": "x", "value": "arm"}}
	b, _ = json.Marshal(base)
	if _, e = parseRegister(b); e == nil {
		t.Fatal("protected field")
	}
	delete(base, "template")
	b, _ = json.Marshal(base)
	if _, e = parseRegister(b); e == nil {
		t.Fatal("attributes without template")
	}
}
