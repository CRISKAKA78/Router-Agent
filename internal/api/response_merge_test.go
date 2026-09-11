package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

// Smart-neighbor validation and device-log errors share the response envelope.
func TestMergedErrorEnvelope(t *testing.T) {
	for _, tc := range []struct{ name, message, field, details, wantMessage string }{
		{"default", "", "", "", "invalid request"},
		{"field", "", "neighbor_probe.domains[0].interface", "interface required", "invalid request"},
		{"message", "archive unavailable", "", "", "archive unavailable"},
		{"combined", "invalid interface", "neighbor_probe.domains[0].interface", "interface required", "invalid interface"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			write(w, response{status: 400, code: "invalid_request", message: tc.message, field: tc.field, details: tc.details})
			var body struct {
				Error map[string]string `json:"error"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if w.Code != 400 || body.Error["code"] != "invalid_request" || body.Error["message"] != tc.wantMessage || body.Error["field"] != tc.field || body.Error["details"] != tc.details {
				t.Fatalf("unexpected error envelope: %s", w.Body.String())
			}
			if _, ok := body.Error["field"]; ok != (tc.field != "") {
				t.Fatal("field presence changed")
			}
		})
	}
}
