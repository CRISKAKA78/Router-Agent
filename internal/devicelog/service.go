// Package devicelog owns bounded device-log queries and file interpretation.
package devicelog

import (
	"compress/gzip"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"path"
	"regexp"
	"strconv"
	"strings"
)

var ErrInvalid = errors.New("invalid device log request")
var ErrRead = errors.New("device log read failed")
var ErrBusy = errors.New("device log query rate limited")

const Capability = "device_logs_v1"

var archiveName = regexp.MustCompile(`^FF_BKDATA_[0-9]{4}-[0-9]{2}-[0-9]{2}\.txt(?:\.gz)?$`)

func ValidPath(s string) bool {
	return strings.HasPrefix(s, "/") && len(s) <= 1024 && !strings.ContainsRune(s, 0)
}

type Params map[string]string

func (p Params) Validate() error {
	switch p["action"] {
	case "enable_live":
		if len(p) == 1 {
			return nil
		}
	case "snapshot", "release":
		if len(p) == 2 && ValidPath(p["path"]) && (p["action"] == "release" || archiveName.MatchString(path.Base(p["path"]))) {
			return nil
		}
	case "history_settings":
		n, e := strconv.ParseUint(p["interval"], 10, 16)
		if len(p) == 4 && e == nil && n > 0 && len(p["interval"]) <= 5 && (p["enabled"] == "0" || p["enabled"] == "1") && (p["persist"] == "0" || p["persist"] == "1") {
			return nil
		}
	}
	return ErrInvalid
}

type Query struct {
	Event      string `json:"event"`
	RequestID  string `json:"request_id"`
	Operation  string `json:"operation"`
	Directory  string `json:"directory"`
	Generation string `json:"generation"`
	Offset     uint64 `json:"offset"`
}

func (q Query) Validate() error {
	if len(q.Generation) > 64 || q.Offset > 1<<53 || (q.Directory != "" && !ValidPath(q.Directory)) {
		return ErrInvalid
	}
	if q.Operation != "live" && q.Operation != "status" && q.Operation != "history" {
		return ErrInvalid
	}
	if q.Operation != "live" && (q.Generation != "" || q.Offset != 0) {
		return ErrInvalid
	}
	if q.Operation != "history" && q.Directory != "" {
		return ErrInvalid
	}
	return nil
}

type Settings struct {
	DebuglogEnable string `json:"debuglog_enable"`
	SyslogdEnable  string `json:"syslogd_enable"`
	LogSaveEn      string `json:"log_save_en"`
	LogSaveItv     string `json:"log_save_itv"`
}
type Live struct {
	State      string `json:"state"`
	Generation string `json:"generation"`
	Start      uint64 `json:"start"`
	Offset     uint64 `json:"offset"`
	Gap        bool   `json:"gap"`
	Data       []byte `json:"data"`
}
type File struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Size     uint64 `json:"size"`
	Modified int64  `json:"modified"`
	Cached   bool   `json:"cached"`
}
type Directory struct {
	Path  string `json:"path"`
	State string `json:"state"`
}
type History struct {
	Files       []File      `json:"files"`
	Directories []Directory `json:"directories"`
	Limited     bool        `json:"limited"`
}
type Result struct {
	SessionID string `json:"session_id"`
	Value     any    `json:"value"`
}
type Backend interface {
	QueryDeviceLog(context.Context, string, string, Query) (json.RawMessage, error)
	CreateDeviceLog(context.Context, string, string, Params) (string, error)
}
type Service struct{ backend Backend }

func New(b Backend) *Service { return &Service{backend: b} }
func (s *Service) Read(ctx context.Context, id, session string, q Query) (Result, error) {
	if session == "" || q.Validate() != nil {
		return Result{}, ErrInvalid
	}
	raw, e := s.backend.QueryDeviceLog(ctx, id, session, q)
	if e != nil {
		return Result{}, e
	}
	v, e := Decode(q.Operation, raw)
	return Result{session, v}, e
}
func (s *Service) Create(ctx context.Context, id, session string, p Params) (string, error) {
	if session == "" || p.Validate() != nil {
		return "", ErrInvalid
	}
	return s.backend.CreateDeviceLog(ctx, id, session, p)
}
func Decode(op string, raw []byte) (any, error) {
	bad := errors.New("invalid device log response")
	switch op {
	case "status":
		var v Settings
		if json.Unmarshal(raw, &v) != nil || len(v.DebuglogEnable+v.SyslogdEnable+v.LogSaveEn+v.LogSaveItv) > 128 {
			return nil, bad
		}
		return v, nil
	case "live":
		var v struct {
			Live
			Hex string `json:"data_hex"`
		}
		if json.Unmarshal(raw, &v) != nil || len(v.Hex) > 16384 || len(v.Generation) > 64 || v.Offset < v.Start || (v.State != "ok" && v.State != "waiting_for_file") {
			return nil, bad
		}
		b, e := hex.DecodeString(v.Hex)
		if e != nil || v.Offset-v.Start != uint64(len(b)) {
			return nil, bad
		}
		v.Data = b
		return v.Live, nil
	case "history":
		var v History
		if json.Unmarshal(raw, &v) != nil || v.Files == nil || v.Directories == nil || len(v.Files) > 128 || len(v.Directories) > 3 {
			return nil, bad
		}
		for _, f := range v.Files {
			if !ValidPath(f.Path) || !archiveName.MatchString(f.Name) || path.Base(f.Path) != f.Name || f.Modified < 0 {
				return nil, bad
			}
		}
		for _, d := range v.Directories {
			if !ValidPath(d.Path) {
				return nil, bad
			}
		}
		return v, nil
	}
	return nil, bad
}

type Preview struct {
	Text         string `json:"text"`
	Truncated    bool   `json:"truncated"`
	Bytes        uint64 `json:"bytes"`
	ParserStatus string `json:"parser_status"`
}

// Reads all gzip members to verify CRC/footer, with a hard decompression bound.
// No vendor semantics: never claims a time range or base-station analysis.
func ReadPreview(ctx context.Context, name string, input io.Reader) (Preview, error) {
	const max = 64 << 20
	const display = 512 << 10
	var gz *gzip.Reader
	if strings.HasSuffix(strings.ToLower(name), ".gz") {
		var e error
		gz, e = gzip.NewReader(input)
		if e != nil {
			return Preview{}, e
		}
		defer gz.Close()
		input = gz
	}
	output := make([]byte, 0, display)
	buf := make([]byte, 8192)
	var size uint64
	for {
		if e := ctx.Err(); e != nil {
			return Preview{}, e
		}
		n, e := input.Read(buf)
		size += uint64(n)
		if size > max {
			return Preview{}, errors.New("decompressed log exceeds 64 MiB")
		}
		if len(output) < display {
			end := min(n, display-len(output))
			output = append(output, buf[:end]...)
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			return Preview{}, e
		}
	}
	return Preview{strings.ToValidUTF8(string(output), "�"), size > display, size, "awaiting_vendor_samples"}, nil
}

// CopyText validates the entire stream before the caller publishes any output.
func CopyText(ctx context.Context, name string, input io.Reader, output io.Writer) error {
	if strings.HasSuffix(strings.ToLower(name), ".gz") {
		r, e := gzip.NewReader(input)
		if e != nil {
			return e
		}
		defer r.Close()
		input = r
	}
	var total int64
	buffer := make([]byte, 8192)
	for {
		if e := ctx.Err(); e != nil {
			return e
		}
		n, e := input.Read(buffer)
		total += int64(n)
		if total > 64<<20 {
			return errors.New("decompressed log exceeds 64 MiB")
		}
		if n > 0 {
			if _, err := output.Write(buffer[:n]); err != nil {
				return err
			}
		}
		if e == io.EOF {
			return nil
		}
		if e != nil {
			return e
		}
	}
}
