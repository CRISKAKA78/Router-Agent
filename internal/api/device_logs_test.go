package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"routerprobe/internal/repository"
	"testing"
)

func TestDeviceLogAPIValidationAndCapability(t *testing.T) {
	_, _, base, control := fixture(t, Config{})
	_, session := register(t, control, "old-log-probe")
	root := "/api/v1/devices/old-log-probe"
	request(t, base, "GET", root+"/logs/live?session_id="+session, "", "", 422)
	request(t, base, "GET", root+"/logs/live", "", "", 400)
	request(t, base, "GET", root+"/logs/live?session_id="+session+"&offset=-1", "", "", 400)
	request(t, base, "POST", root+"/log-tasks", "unsupported", fmt.Sprintf(`{"session_id":%q,"params":{"action":"enable_live"}}`, session), 422)
	for i, b := range []string{`{"params":{"action":"enable_live"}}`, `{"session_id":"x","params":{"action":"enable_live","command":"reboot"}}`, `{"session_id":"x","params":{"action":"history_settings","enabled":"1","interval":"0","persist":"1"}}`} {
		request(t, base, "POST", root+"/log-tasks", fmt.Sprint(i), b, 400)
	}
}
func TestLogAssetPreviewAndTextIdempotency(t *testing.T) {
	_, app, base, _ := fixture(t, Config{})
	var gz bytes.Buffer
	for _, s := range []string{"first\n", "second\n"} {
		w := gzip.NewWriter(&gz)
		w.Write([]byte(s))
		w.Close()
	}
	asset, e := app.Files().Import(context.Background(), "FF_BKDATA_2025-09-25.txt.gz", bytes.NewReader(gz.Bytes()))
	if e != nil {
		t.Fatal(e)
	}
	path := "/api/v1/log-assets/" + asset.ID
	v := data(request(t, base, "GET", path+"/preview", "", "", 200))
	if v["text"] != "first\nsecond\n" || v["parser_status"] != "awaiting_vendor_samples" {
		t.Fatal(v)
	}
	decoded := data(request(t, base, "POST", path+"/text", "decode", "{}", 201))
	repeat := data(request(t, base, "POST", path+"/text", "decode", "{}", 201))
	if decoded["asset_id"] != repeat["asset_id"] || decoded["asset_id"] == nil {
		t.Fatal(decoded, repeat)
	}
	e = app.Files().ReadContent(context.Background(), decoded["asset_id"].(string), func(a repository.Asset, r io.ReadSeeker) error {
		b, e := io.ReadAll(r)
		if string(b) != "first\nsecond\n" {
			t.Fatal(string(b))
		}
		return e
	})
	if e != nil {
		t.Fatal(e)
	}
}
