package filetransfer

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testID = "00112233-4455-4677-8899-aabbccddeeff"

func TestWireContract(t *testing.T) {
	b, e := Chunk(testID, 0, []byte{0, 255, 128})
	if e != nil {
		t.Fatal(e)
	}
	if hex.EncodeToString(b[:16]) != "00112233445546778899aabbccddeeff" {
		t.Fatal("mixed-endian UUID")
	}
	if _, e = ParseChunk(b, testID, 0, 3, 3); e != nil {
		t.Fatal(e)
	}
	for _, bad := range []struct {
		off, size int64
		limit     uint32
	}{{1, 3, 3}, {0, 2, 3}, {0, 3, 2}} {
		if _, e = ParseChunk(b, testID, bad.off, bad.size, bad.limit); e == nil {
			t.Fatal("invalid chunk accepted")
		}
	}
	for _, id := range []string{strings.ToUpper(testID), "00112233445546778899aabbccddeeff", "00112233_4455-4677-8899-aabbccddeeff"} {
		if _, e = UUID(id); e == nil {
			t.Fatal("noncanonical UUID")
		}
	}
	for _, status := range []string{"ready", "done", "failed"} {
		raw, _ := json.Marshal(NewAck(7, testID, status, 0))
		if _, e = ParseAck(raw); e != nil {
			t.Fatal(e)
		}
		var m map[string]interface{}
		json.Unmarshal(raw, &m)
		if status == "ready" {
			if _, ok := m["sha256_ok"]; ok {
				t.Fatal("ready includes checksum")
			}
			for _, v := range []interface{}{true, false, nil} {
				m["sha256_ok"] = v
				raw, _ = json.Marshal(m)
				if _, e = ParseAck(raw); e == nil {
					t.Fatal("ready checksum accepted")
				}
			}
		} else {
			delete(m, "sha256_ok")
			raw, _ = json.Marshal(m)
			if _, e = ParseAck(raw); e == nil {
				t.Fatal("missing checksum accepted")
			}
			m["sha256_ok"] = status != "done"
			raw, _ = json.Marshal(m)
			if _, e = ParseAck(raw); e == nil {
				t.Fatal("incorrect checksum accepted")
			}
		}
	}
}

func TestFileJSONRequiredAndUnicode(t *testing.T) {
	for _, raw := range []string{`{"transfer_id":"x","transfer_id":"y"}`, `{"transfer_id":"\ud800"}`, `{"transfer_id":"\udc00"}`, `{"transfer_id":null}`, `[]`} {
		var v struct {
			TransferID string `json:"transfer_id"`
		}
		if Decode([]byte(raw), &v, "transfer_id") == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	var v struct {
		TransferID string `json:"transfer_id"`
	}
	if e := Decode([]byte(`{"transfer_id":"\ud83d\ude00","extension":1}`), &v, "transfer_id"); e != nil || v.TransferID != "😀" {
		t.Fatal("valid surrogate pair", e)
	}
}
func TestReceiverAtomicPublication(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	old := []byte("original")
	os.WriteFile(target, old, 0600)
	r, e := NewReceiver(target, true)
	if e != nil {
		t.Fatal(e)
	}
	r.Write([]byte("wrong"))
	if e = r.Commit(5, strings.Repeat("0", 64)); e == nil {
		t.Fatal("bad hash committed")
	}
	r.Close()
	got, _ := os.ReadFile(target)
	if !bytes.Equal(got, old) {
		t.Fatal("failed transfer changed target")
	}
	r, e = NewReceiver(filepath.Join(dir, "race"), false)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	r.Write([]byte("abc"))
	os.WriteFile(r.target, old, 0600)
	if e = r.Commit(3, "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"); e == nil {
		t.Fatal("concurrent target overwritten")
	}
	got, _ = os.ReadFile(r.target)
	if !bytes.Equal(got, old) {
		t.Fatal("no-replace lost race")
	}
	r.Close()
	files, _ := filepath.Glob(filepath.Join(dir, ".rmp-transfer-*"))
	if len(files) != 0 {
		t.Fatal(files)
	}
}
