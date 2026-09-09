// Package catalogupgrade removes retired metadata from server-owned catalogs.
// It is deliberately not used by API, project import, or protocol decoders.
package catalogupgrade

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type object map[string]json.RawMessage

func edit(raw json.RawMessage, change func(object) bool) (json.RawMessage, bool) {
	var fields object
	if json.Unmarshal(raw, &fields) != nil || fields == nil || !change(fields) {
		return raw, false
	}
	b, _ := json.Marshal(fields)
	return b, true
}

func remove(fields object, keys ...string) bool {
	changed := false
	for _, key := range keys {
		if _, ok := fields[key]; ok {
			delete(fields, key)
			changed = true
		}
	}
	return changed
}

func template(fields object) bool {
	raw, changed := edit(fields["presentation"], func(p object) bool {
		return remove(p, "interface_aliases")
	})
	if changed {
		fields["presentation"] = raw
	}
	return changed
}

// Templates and Devices preserve unknown fields for the caller's strict
// decoder to reject. Only explicitly retired fields at known paths are removed.
// Callers validate the original JSON before invoking these functions.
func Templates(b []byte) ([]byte, bool) {
	return edit(b, func(c object) bool {
		var items []json.RawMessage
		if json.Unmarshal(c["templates"], &items) != nil {
			return false
		}
		changed := false
		for i, item := range items {
			updated, ok := edit(item, template)
			if ok {
				items[i], changed = updated, true
			}
		}
		if changed {
			c["templates"], _ = json.Marshal(items)
		}
		return changed
	})
}

func Devices(b []byte) ([]byte, bool) {
	return edit(b, func(c object) bool {
		updated, changed := edit(c["devices"], func(devices object) bool {
			changed := false
			for id, raw := range devices {
				updated, ok := edit(raw, func(p object) bool {
					changed := false
					for _, entry := range []struct {
						key string
						fn  func(object) bool
					}{
						{"bound_template", template},
						{"configuration", func(config object) bool {
							updated, ok := edit(config["template"], template)
							if ok {
								config["template"] = updated
							}
							return ok
						}},
						{"reported", func(r object) bool {
							return remove(r, "ReportIntervals", "Template", "Attributes", "CollectionErrors")
						}},
					} {
						updated, ok := edit(p[entry.key], entry.fn)
						if ok {
							p[entry.key], changed = updated, true
						}
					}
					return changed
				})
				if ok {
					devices[id], changed = updated, true
				}
			}
			return changed
		})
		if changed {
			c["devices"] = updated
		}
		return changed
	})
}

// Backup durably saves the exact original bytes before any catalog replacement.
// Unique names avoid overwriting a previous upgrade's recovery copy.
func Backup(path string, original []byte) (string, error) {
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".pre-upgrade-*.bak")
	if err != nil {
		return "", err
	}
	if _, err = f.Write(original); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return "", err
	}
	return f.Name(), closeErr
}
