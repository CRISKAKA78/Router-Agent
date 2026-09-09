package gateway

import (
	"encoding/json"
	"testing"
)

func TestRetiredRegistrationCollectionRejected(t *testing.T) {
	for _, key := range []string{"template", "attributes", "collection_errors", "report_intervals"} {
		var base map[string]any
		json.Unmarshal([]byte(validRegister("current")), &base)
		base[key] = map[string]any{}
		payload, _ := json.Marshal(base)
		if _, err := parseRegister(payload); err == nil {
			t.Fatal("retired field accepted", key)
		}
	}
}
