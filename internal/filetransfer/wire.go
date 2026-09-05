package filetransfer

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"routerprobe/internal/protocol"
	"strings"
	"unicode/utf8"
)

const MaxChunk = 512 * 1024

type Params struct {
	TransferID string `json:"transfer_id"`
	RemotePath string `json:"remote_path"`
	Size       int64  `json:"size,omitempty"`
	SHA256     string `json:"sha256,omitempty"`
	Mode       string `json:"mode,omitempty"`
	Overwrite  bool   `json:"overwrite,omitempty"`
	ResultName string `json:"result_name,omitempty"`
}

func (p Params) Wire(kind string) map[string]interface{} {
	m := map[string]interface{}{"transfer_id": p.TransferID, "remote_path": p.RemotePath}
	if kind == "upload" {
		m["size"] = p.Size
		m["sha256"] = p.SHA256
		m["mode"] = p.Mode
		m["overwrite"] = p.Overwrite
	} else {
		m["result_name"] = p.ResultName
	}
	return m
}
func UUID(s string) ([]byte, error) {
	if len(s) != 36 || s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' || strings.ToLower(s) != s {
		return nil, errors.New("invalid canonical UUID")
	}
	b, e := hex.DecodeString(strings.ReplaceAll(s, "-", ""))
	if e != nil || len(b) != 16 {
		return nil, errors.New("invalid UUID")
	}
	return b, nil
}
func Name(s string) bool {
	return len(s) > 0 && len(s) <= 255 && s != "." && s != ".." && !strings.ContainsAny(s, "/\\\x00") && utf8.ValidString(s)
}
func Digest(s string) bool {
	b, e := hex.DecodeString(s)
	return e == nil && len(b) == 32 && strings.ToLower(s) == s
}
func (p Params) Validate(kind string) error {
	if _, e := UUID(p.TransferID); e != nil {
		return e
	}
	if len(p.RemotePath) == 0 || p.RemotePath[0] != '/' || len(p.RemotePath) > 4096 || strings.ContainsRune(p.RemotePath, 0) || !utf8.ValidString(p.RemotePath) {
		return errors.New("invalid remote_path")
	}
	if kind == "download" {
		if !Name(p.ResultName) {
			return errors.New("invalid result_name")
		}
		return nil
	}
	if kind != "upload" || p.Size < 0 || !Digest(p.SHA256) || len(p.Mode) != 4 || p.Mode[0] != '0' || strings.Trim(p.Mode[1:], "01234567") != "" {
		return errors.New("invalid upload metadata")
	}
	return nil
}

type Begin struct {
	TransferID string  `json:"transfer_id"`
	TaskID     string  `json:"task_id"`
	Direction  string  `json:"direction"`
	Name       string  `json:"name"`
	RemotePath string  `json:"remote_path"`
	Size       int64   `json:"size"`
	SHA256     string  `json:"sha256"`
	ChunkSize  uint32  `json:"chunk_size"`
	Mode       *string `json:"mode,omitempty"`
	Overwrite  *bool   `json:"overwrite,omitempty"`
}
type End struct {
	TransferID string `json:"transfer_id"`
	Size       int64  `json:"size"`
	SHA256     string `json:"sha256"`
}
type Ack struct {
	ReplyTo    uint64 `json:"reply_to"`
	TransferID string `json:"transfer_id"`
	Status     string `json:"status"`
	Received   int64  `json:"received"`
	SHA256OK   *bool  `json:"sha256_ok,omitempty"`
}

func NewAck(reply uint64, id, status string, n int64) Ack {
	a := Ack{ReplyTo: reply, TransferID: id, Status: status, Received: n}
	if status != "ready" {
		b := status == "done"
		a.SHA256OK = &b
	}
	return a
}

// Required fields are checked before decoding so zero, false and absent differ.
func Decode(data []byte, out interface{}, fields ...string) error {
	var m map[string]json.RawMessage
	if !protocol.ValidUnicodeJSON(data) || json.Unmarshal(data, &m) != nil || m == nil {
		return errors.New("invalid file JSON object")
	}
	// Decode top-level keys once to reject duplicates, including escaped aliases.
	d := json.NewDecoder(bytes.NewReader(data))
	_, _ = d.Token()
	seen := map[string]bool{}
	for d.More() {
		key, e := d.Token()
		if e != nil {
			return e
		}
		k, ok := key.(string)
		if !ok || seen[k] {
			return errors.New("duplicate file JSON key")
		}
		seen[k] = true
		var raw json.RawMessage
		if e = d.Decode(&raw); e != nil {
			return e
		}
	}
	for _, k := range fields {
		v, ok := m[k]
		if !ok || string(v) == "null" {
			return errors.New("missing file field: " + k)
		}
	}
	return json.Unmarshal(data, out)
}

func ParseAck(data []byte) (Ack, error) {
	var a Ack
	e := Decode(data, &a, "reply_to", "transfer_id", "status", "received")
	if e != nil {
		return a, e
	}
	if _, e = UUID(a.TransferID); e != nil {
		return a, e
	}
	var m map[string]json.RawMessage
	_ = json.Unmarshal(data, &m)
	_, has := m["sha256_ok"]
	if v, ok := m["message"]; ok {
		var message string
		if string(v) == "null" || json.Unmarshal(v, &message) != nil || len(message) > 512 {
			return a, errors.New("invalid ACK message")
		}
	}
	if a.ReplyTo == 0 || a.Received < 0 {
		return a, errors.New("invalid file ACK")
	}
	switch a.Status {
	case "ready":
		if has || a.Received != 0 {
			return a, errors.New("ready must omit sha256_ok")
		}
	case "done", "failed":
		if a.SHA256OK == nil || *a.SHA256OK != (a.Status == "done") {
			return a, errors.New("invalid sha256_ok")
		}
	default:
		return a, errors.New("invalid ACK status")
	}
	return a, nil
}
func ParseBegin(data []byte, limit uint32) (Begin, error) {
	var b Begin
	e := Decode(data, &b, "transfer_id", "task_id", "direction", "name", "remote_path", "size", "sha256", "chunk_size")
	if e != nil {
		return b, e
	}
	if _, e = UUID(b.TransferID); e != nil {
		return b, e
	}
	p := Params{TransferID: b.TransferID, RemotePath: b.RemotePath, ResultName: b.Name}
	if p.Validate("download") != nil || b.TaskID == "" || len(b.TaskID) > 128 || b.Size < 0 || !Digest(b.SHA256) || b.ChunkSize == 0 || b.ChunkSize > limit {
		return b, errors.New("invalid FILE_BEGIN")
	}
	if b.Direction == "device_to_server" {
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(data, &fields)
		_, mode := fields["mode"]
		_, overwrite := fields["overwrite"]
		if mode || overwrite {
			return b, errors.New("download mode/overwrite forbidden")
		}
	} else if b.Direction == "server_to_device" {
		if b.Mode == nil || b.Overwrite == nil {
			return b, errors.New("upload mode/overwrite required")
		}
	} else {
		return b, errors.New("invalid direction")
	}
	return b, nil
}
func Chunk(id string, off uint64, data []byte) ([]byte, error) {
	u, e := UUID(id)
	if e != nil {
		return nil, e
	}
	if len(data) == 0 || len(data) > MaxChunk {
		return nil, errors.New("invalid chunk length")
	}
	b := make([]byte, 28+len(data))
	copy(b, u)
	binary.BigEndian.PutUint64(b[16:], off)
	binary.BigEndian.PutUint32(b[24:], uint32(len(data)))
	copy(b[28:], data)
	return b, nil
}
func ParseChunk(b []byte, id string, off int64, size int64, limit uint32) ([]byte, error) {
	u, e := UUID(id)
	if e != nil {
		return nil, e
	}
	if len(b) < 29 || string(b[:16]) != string(u) || off < 0 || size < off {
		return nil, errors.New("invalid chunk identity")
	}
	n := binary.BigEndian.Uint32(b[24:])
	offset := binary.BigEndian.Uint64(b[16:])
	if offset > math.MaxInt64 || int64(offset) != off || n == 0 || n > limit || uint64(n) != uint64(len(b)-28) || int64(n) > size-off {
		return nil, errors.New("invalid chunk offset/length")
	}
	return b[28:], nil
}
