// Package repository owns persistent file assets and tool catalog metadata.
// It has no TCP, task or transfer lifecycle responsibilities.
package repository

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrNotFound   = errors.New("repository item not found")
	ErrArchived   = errors.New("repository item archived")
	ErrConflict   = errors.New("repository identity conflict")
	ErrReferenced = errors.New("asset referenced by an active version")
	ErrClosed     = errors.New("repository closed")
	ErrIntegrity  = errors.New("repository content integrity mismatch")
)

type Asset struct {
	ID        string    `json:"asset_id"`
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	SHA256    string    `json:"sha256"`
	CreatedAt time.Time `json:"created_at"`
	Archived  bool      `json:"archived"`
}
type Tool struct {
	ID          string    `json:"tool_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	Archived    bool      `json:"archived"`
}
type Rules struct {
	Arch                 []string `json:"arch"` // ["any"] must be explicit, never mixed with other values.
	Libc                 []string `json:"libc"`
	Models               []string `json:"models"`
	Kernels              []string `json:"kernels"`
	RequiredCapabilities []string `json:"required_capabilities"`
}
type Artifact struct {
	ID       string `json:"artifact_id"`
	AssetID  string `json:"asset_id"`
	Platform string `json:"platform"`
	Mode     string `json:"mode"`
	Rules    Rules  `json:"rules"`
}
type Version struct {
	ToolID    string     `json:"tool_id"`
	Version   string     `json:"version"`
	Artifacts []Artifact `json:"artifacts"`
	CreatedAt time.Time  `json:"created_at"`
	Archived  bool       `json:"archived"`
}

// ArtifactSpec omits the server-generated stable artifact identity.
type ArtifactSpec struct {
	AssetID, Platform, Mode string
	Rules                   Rules
}
type catalog struct {
	SchemaVersion int       `json:"schema_version"`
	Assets        []Asset   `json:"assets"`
	Tools         []Tool    `json:"tools"`
	Versions      []Version `json:"versions"`
}

func uuid() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = b[6]&15 | 64
	b[8] = b[8]&63 | 128
	h := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s-%s-%s", h[:8], h[8:12], h[12:16], h[16:20], h[20:]), nil
}
func validID(s string) bool {
	if len(s) != 36 || s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return false
	}
	h := strings.ReplaceAll(s, "-", "")
	b, e := hex.DecodeString(h)
	return e == nil && len(b) == 16 && strings.ToLower(s) == s && b[6]>>4 == 4 && b[8]>>6 == 2
}
func textOK(s string, max int, empty bool) bool {
	return (empty || s != "") && len(s) <= max && utf8.ValidString(s) && !strings.ContainsRune(s, 0)
}
func normalizeArch(s string) string {
	switch s {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	}
	return s
}
func normalizeSet(in []string, kind string, required bool) ([]string, error) {
	if len(in) > 32 || required && len(in) == 0 {
		return nil, fmt.Errorf("invalid %s constraints", kind)
	}
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, v := range in {
		if !textOK(v, 128, false) {
			return nil, fmt.Errorf("invalid %s value", kind)
		}
		if kind == "arch" || kind == "libc" || kind == "capabilities" {
			for _, c := range v {
				if c < 33 || c > 126 {
					return nil, fmt.Errorf("invalid %s token", kind)
				}
			}
		}
		if kind == "arch" {
			v = normalizeArch(v)
		}
		if kind == "libc" {
			v = strings.ToLower(v)
		}
		if required && v == "any" && len(in) != 1 {
			return nil, fmt.Errorf("any must stand alone")
		}
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out, nil
}
func normalizeRules(r Rules) (Rules, error) {
	out := Rules{}
	var err error
	for _, x := range []struct {
		in       []string
		out      *[]string
		name     string
		required bool
	}{{r.Arch, &out.Arch, "arch", true}, {r.Libc, &out.Libc, "libc", true}, {r.Models, &out.Models, "model", false}, {r.Kernels, &out.Kernels, "kernel", false}, {r.RequiredCapabilities, &out.RequiredCapabilities, "capabilities", false}} {
		*x.out, err = normalizeSet(x.in, x.name, x.required)
		if err != nil {
			return Rules{}, err
		}
	}
	return out, nil
}
func normalizeSpec(a ArtifactSpec) (ArtifactSpec, error) {
	if !validID(a.AssetID) || a.Platform != "linux" || len(a.Mode) != 4 || a.Mode[0] != '0' {
		return a, errors.New("invalid artifact asset/platform/mode")
	}
	for _, c := range a.Mode[1:] {
		if c < '0' || c > '7' {
			return a, errors.New("invalid artifact mode")
		}
	}
	r, e := normalizeRules(a.Rules)
	a.Rules = r
	return a, e
}
func cloneVersion(v Version) Version {
	v.Artifacts = append([]Artifact{}, v.Artifacts...)
	for i := range v.Artifacts {
		r := &v.Artifacts[i].Rules
		r.Arch = append([]string{}, r.Arch...)
		r.Libc = append([]string{}, r.Libc...)
		r.Models = append([]string{}, r.Models...)
		r.Kernels = append([]string{}, r.Kernels...)
		r.RequiredCapabilities = append([]string{}, r.RequiredCapabilities...)
	}
	return v
}
func cloneCatalog(c catalog) catalog {
	c.Assets = append([]Asset{}, c.Assets...)
	c.Tools = append([]Tool{}, c.Tools...)
	c.Versions = append([]Version{}, c.Versions...)
	for i := range c.Versions {
		c.Versions[i] = cloneVersion(c.Versions[i])
	}
	return c
}
