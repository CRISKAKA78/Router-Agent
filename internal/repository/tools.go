package repository

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"time"
)

type ToolService struct{ s *Store }

func (t *ToolService) Create(name, description string) (Tool, error) {
	if !textOK(name, 255, false) || !textOK(description, 4096, true) {
		return Tool{}, errors.New("invalid tool metadata")
	}
	s := t.s
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Tool{}, ErrClosed
	}
	id, e := s.nextID()
	if e != nil {
		return Tool{}, e
	}
	v := Tool{ID: id, Name: name, Description: description, CreatedAt: time.Now().UTC()}
	c := cloneCatalog(s.data)
	c.Tools = append(c.Tools, v)
	if e = s.commit(c); e != nil {
		return Tool{}, e
	}
	return v, nil
}
func (s *Store) tool(id string) (Tool, error) {
	for _, t := range s.data.Tools {
		if t.ID == id {
			return t, nil
		}
	}
	return Tool{}, ErrNotFound
}
func (s *Store) version(toolID, version string) (Version, error) {
	for _, v := range s.data.Versions {
		if v.ToolID == toolID && v.Version == version {
			return v, nil
		}
	}
	return Version{}, ErrNotFound
}
func (t *ToolService) Get(id string) (Tool, error) {
	s := t.s
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return Tool{}, ErrClosed
	}
	return s.tool(id)
}
func (t *ToolService) List(includeArchived bool) ([]Tool, error) {
	s := t.s
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, ErrClosed
	}
	out := []Tool{}
	for _, v := range s.data.Tools {
		if includeArchived || !v.Archived {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
func (t *ToolService) Version(toolID, version string) (Version, error) {
	s := t.s
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return Version{}, ErrClosed
	}
	v, e := s.version(toolID, version)
	return cloneVersion(v), e
}
func (t *ToolService) Versions(toolID string, includeArchived bool) ([]Version, error) {
	s := t.s
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, ErrClosed
	}
	tool, e := s.tool(toolID)
	if e != nil {
		return nil, e
	}
	out := []Version{}
	for _, v := range s.data.Versions {
		if v.ToolID == toolID && (includeArchived || !v.Archived && !tool.Archived) {
			out = append(out, cloneVersion(v))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}
func specs(artifacts []Artifact) []ArtifactSpec {
	out := make([]ArtifactSpec, 0, len(artifacts))
	for _, a := range artifacts {
		out = append(out, ArtifactSpec{a.AssetID, a.Platform, a.Mode, a.Rules})
	}
	return out
}
func specKey(a ArtifactSpec) string { b, _ := json.Marshal(a); return string(b) }
func (t *ToolService) Publish(toolID, version string, input []ArtifactSpec) (Version, error) {
	if !textOK(version, 128, false) || len(input) == 0 || len(input) > 128 {
		return Version{}, errors.New("invalid version")
	}
	normalized := make([]ArtifactSpec, len(input))
	for i, a := range input {
		v, e := normalizeSpec(a)
		if e != nil {
			return Version{}, e
		}
		normalized[i] = v
	}
	sort.Slice(normalized, func(i, j int) bool { return specKey(normalized[i]) < specKey(normalized[j]) })
	s := t.s
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Version{}, ErrClosed
	}
	tool, e := s.tool(toolID)
	if e != nil {
		return Version{}, e
	}
	if previous, e := s.version(toolID, version); e == nil {
		if !reflect.DeepEqual(specs(previous.Artifacts), normalized) {
			return Version{}, ErrConflict
		}
		return cloneVersion(previous), nil
	}
	if tool.Archived {
		return Version{}, ErrArchived
	}
	for _, a := range normalized {
		asset, e := s.asset(a.AssetID)
		if e != nil {
			return Version{}, e
		}
		if asset.Archived {
			return Version{}, ErrArchived
		}
	}
	v := Version{ToolID: toolID, Version: version, CreatedAt: time.Now().UTC(), Artifacts: []Artifact{}}
	generated := map[string]bool{}
	for _, a := range normalized {
		var id string
		for {
			id, e = s.nextID()
			if e != nil {
				return Version{}, e
			}
			if !generated[id] {
				generated[id] = true
				break
			}
		}
		v.Artifacts = append(v.Artifacts, Artifact{id, a.AssetID, a.Platform, a.Mode, a.Rules})
	}
	c := cloneCatalog(s.data)
	c.Versions = append(c.Versions, v)
	if e = s.commit(c); e != nil {
		return Version{}, e
	}
	return cloneVersion(v), nil
}
func (t *ToolService) Archive(id string) error {
	s := t.s
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	v, e := s.tool(id)
	if e != nil {
		return e
	}
	if v.Archived {
		return nil
	}
	c := cloneCatalog(s.data)
	for i := range c.Tools {
		if c.Tools[i].ID == id {
			c.Tools[i].Archived = true
		}
	}
	return s.commit(c)
}
func (t *ToolService) ArchiveVersion(toolID, version string) error {
	s := t.s
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	v, e := s.version(toolID, version)
	if e != nil {
		return e
	}
	if v.Archived {
		return nil
	}
	c := cloneCatalog(s.data)
	for i := range c.Versions {
		if c.Versions[i].ToolID == toolID && c.Versions[i].Version == version {
			c.Versions[i].Archived = true
		}
	}
	return s.commit(c)
}

// WithArtifact rechecks all archive flags when admitting dispatch. The callback
// may use Device/Transfer services, but must not reenter Repository methods.
func (t *ToolService) WithArtifact(ctx context.Context, toolID, version, artifactID string, use func(Artifact, Asset, string) error) error {
	s := t.s
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return ErrClosed
	}
	tool, e := s.tool(toolID)
	if e != nil {
		return e
	}
	v, e := s.version(toolID, version)
	if e != nil {
		return e
	}
	if tool.Archived || v.Archived {
		return ErrArchived
	}
	for _, a := range v.Artifacts {
		if a.ID == artifactID {
			asset, e := s.asset(a.AssetID)
			if e != nil {
				return e
			}
			if asset.Archived {
				return ErrArchived
			}
			path := s.blobPath(asset.SHA256)
			if e = verifyContent(ctx, path, asset.Size, asset.SHA256); e != nil {
				return e
			}
			return use(cloneVersion(Version{Artifacts: []Artifact{a}}).Artifacts[0], asset, path)
		}
	}
	return ErrNotFound
}
