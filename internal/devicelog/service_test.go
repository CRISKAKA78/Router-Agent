package devicelog

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func compressed(t *testing.T, text string) []byte {
	t.Helper()
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	w.Write([]byte(text))
	if e := w.Close(); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func TestParams(t *testing.T) {
	for _, p := range []Params{{"action": "enable_live"}, {"action": "snapshot", "path": "/jffs/FF_BKDATA_2025-09-25.txt.gz"}, {"action": "history_settings", "enabled": "1", "interval": "300", "persist": "1"}, {"action": "history_settings", "enabled": "0", "interval": "65535", "persist": "0"}} {
		if p.Validate() != nil {
			t.Fatal(p)
		}
	}
	for _, p := range []Params{nil, {"action": "enable_live", "command": "reboot"}, {"action": "snapshot", "path": "/etc/passwd"}, {"action": "snapshot", "path": "relative/FF_BKDATA_2025-09-25.txt.gz"}, {"action": "history_settings", "enabled": "1", "interval": "0", "persist": "1"}, {"action": "history_settings", "enabled": "1", "interval": "65536", "persist": "1"}, {"action": "history_settings", "enabled": "1", "interval": "300"}} {
		if p.Validate() == nil {
			t.Fatal(p)
		}
	}
}
func TestQueryAndDecode(t *testing.T) {
	for _, q := range []Query{{Operation: "live"}, {Operation: "status"}, {Operation: "history", Directory: "/jffs"}} {
		if q.Validate() != nil {
			t.Fatal(q)
		}
	}
	for _, q := range []Query{{Operation: "reboot"}, {Operation: "live", Directory: "/tmp"}, {Operation: "history", Directory: "relative"}, {Operation: "live", Offset: 1 << 60}} {
		if q.Validate() == nil {
			t.Fatal(q)
		}
	}
	v, e := Decode("live", []byte(`{"state":"ok","generation":"1:2","start":0,"offset":3,"gap":false,"data_hex":"e4b8ad"}`))
	if e != nil || string(v.(Live).Data) != "中" {
		t.Fatal(v, e)
	}
	for _, raw := range []string{`{"state":"ok","offset":1,"data_hex":"xx"}`, `{"state":"ok","offset":2,"data_hex":"01"}`, `{"state":"unknown"}`, `{"state":"ok","offset":0,"data_hex":"` + strings.Repeat("00", 8193) + `"}`} {
		if _, e := Decode("live", []byte(raw)); e == nil {
			t.Fatal("invalid batch accepted")
		}
	}
	if _, e := Decode("history", []byte(`{"files":[{"name":"../bad","path":"/jffs/../bad"}],"directories":[]}`)); e == nil {
		t.Fatal("unsafe file accepted")
	}
}
func TestGzipMembersIntegrityAndPreview(t *testing.T) {
	raw := append(compressed(t, "first\n"), compressed(t, "第二段\n")...)
	v, e := ReadPreview(context.Background(), "log.txt.gz", bytes.NewReader(raw))
	if e != nil || v.Text != "first\n第二段\n" || v.ParserStatus != "awaiting_vendor_samples" || v.Truncated {
		t.Fatal(v, e)
	}
	var out bytes.Buffer
	if e = CopyText(context.Background(), "log.txt.gz", bytes.NewReader(raw), &out); e != nil || out.String() != v.Text {
		t.Fatal(out.String(), e)
	}
	broken := append([]byte(nil), raw...)
	broken[len(broken)-8] ^= 1
	for _, bad := range [][]byte{raw[:len(raw)-2], broken, []byte("not gzip")} {
		if _, e = ReadPreview(context.Background(), "log.gz", bytes.NewReader(bad)); e == nil {
			t.Fatal("invalid gzip accepted")
		}
	}
	v, e = ReadPreview(context.Background(), "log.txt", strings.NewReader(strings.Repeat("x", 600000)))
	if e != nil || !v.Truncated || len(v.Text) != 512<<10 || v.Bytes != 600000 {
		t.Fatal(v.Bytes, e)
	}
	c, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = ReadPreview(c, "x.txt", strings.NewReader("x")); e == nil {
		t.Fatal("cancel ignored")
	}
	_, _ = json.Marshal(v)
}
