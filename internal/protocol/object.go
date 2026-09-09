package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// Reject duplicate members, null, non-object roots and trailing JSON. This is
// HTTP input validation only; it does not change the Probe decoder contract.
func ValidObject(b []byte) error { return validObject(b, false) }

// ValidStoredObject permits explicit unknown fields in durable observation snapshots.
func ValidStoredObject(b []byte) error { return validObject(b, true) }
func validObject(b []byte, allowNull bool) error {
	if !ValidUnicodeJSON(b) {
		return errors.New("invalid Unicode")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var value func(int) error
	value = func(depth int) error {
		if depth > 32 {
			return errors.New("JSON nesting limit")
		}
		t, e := d.Token()
		if e != nil {
			return e
		}
		if t == nil && !allowNull {
			return errors.New("null not supported")
		}
		if delimiter, ok := t.(json.Delim); ok {
			switch delimiter {
			case '{':
				seen := map[string]bool{}
				for d.More() {
					k, e := d.Token()
					if e != nil {
						return e
					}
					name, ok := k.(string)
					if !ok || seen[name] {
						return errors.New("duplicate member")
					}
					seen[name] = true
					if e = value(depth + 1); e != nil {
						return e
					}
				}
				end, e := d.Token()
				if e != nil || end != json.Delim('}') {
					return errors.New("object")
				}
			case '[':
				for d.More() {
					if e = value(depth + 1); e != nil {
						return e
					}
				}
				end, e := d.Token()
				if e != nil || end != json.Delim(']') {
					return errors.New("array")
				}
			default:
				return errors.New("delimiter")
			}
		} else if depth == 0 {
			return errors.New("object required")
		}
		return nil
	}
	b = bytes.TrimSpace(b)
	if len(b) == 0 || b[0] != '{' {
		return errors.New("object required")
	}
	if e := value(0); e != nil {
		return e
	}
	if _, e := d.Token(); e != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}
