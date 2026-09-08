// Package routerconfig defines the shared, bounded configuration command contract.
package routerconfig

import (
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"
)

var ErrInvalid = errors.New("invalid router configuration parameters")
var ErrUnsupported = errors.New("probe does not support router_config")

var nvramKey = regexp.MustCompile(`^[A-Za-z0-9_./:][A-Za-z0-9_./:-]*$`)
var uciKey = regexp.MustCompile(`^[A-Za-z0-9_]+\.([A-Za-z0-9_]+|@[A-Za-z0-9_]+\[-?[0-9]+\])\.[A-Za-z0-9_]+$`)
var uciPackage = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

type Params struct {
	Backend   string  `json:"backend"`
	Operation string  `json:"operation"`
	Key       *string `json:"key,omitempty"`
	Value     *string `json:"value,omitempty"`
	Package   *string `json:"package,omitempty"`
}

func ValidKey(backend, key string) bool {
	switch backend {
	case "nvram":
		return len(key) <= 128 && nvramKey.MatchString(key)
	case "uci":
		return len(key) <= 256 && uciKey.MatchString(key)
	}
	return false
}

func (p Params) Validate() error {
	if p.Backend != "nvram" && p.Backend != "uci" {
		return ErrInvalid
	}
	if p.Operation == "commit" {
		if p.Key != nil || p.Value != nil {
			return ErrInvalid
		}
		if p.Backend == "nvram" {
			if p.Package != nil {
				return ErrInvalid
			}
			return nil
		}
		if p.Package == nil || len(*p.Package) > 256 || !uciPackage.MatchString(*p.Package) {
			return ErrInvalid
		}
		return nil
	}
	if p.Package != nil || p.Key == nil || !ValidKey(p.Backend, *p.Key) {
		return ErrInvalid
	}
	switch p.Operation {
	case "get", "delete":
		if p.Value != nil {
			return ErrInvalid
		}
	case "set":
		if p.Value == nil || len(*p.Value) > 4096 || !utf8.ValidString(*p.Value) || strings.ContainsRune(*p.Value, 0) {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}
