// Package management composes Repository, Device and existing Task/File services.
// It does not encode Probe messages or own a second transfer implementation.
package management

import (
	"context"
	"errors"
	"fmt"
	"os"
	"routerprobe/internal/device"
	"routerprobe/internal/filetransfer"
	"routerprobe/internal/repository"
	"routerprobe/internal/task"
	"sync"
	"time"
)

var (
	ErrOffline      = errors.New("device offline")
	ErrIncompatible = errors.New("no compatible artifact")
	ErrAmbiguous    = errors.New("multiple compatible artifacts; specify artifact_id")
	ErrNotCommitted = errors.New("download not committed and released")
)

type FileTasks interface {
	CreateUpload(context.Context, string, filetransfer.UploadRequest) (string, error)
	CreateDownload(context.Context, string, filetransfer.DownloadRequest) (string, error)
	FileSnapshot(string) (filetransfer.Snapshot, error)
	TaskSnapshot(string) (task.Snapshot, error)
	WaitTaskResult(context.Context, string) (task.Result, error)
	ResendTask(context.Context, string) error
}
type Operation struct {
	TaskID, TransferID, DeviceID, SessionID string
	ToolID, Version, ArtifactID, AssetID    string
}
type Candidate struct {
	Artifact repository.Artifact
	Match    repository.MatchResult
}
type DeployRequest struct {
	DeviceID, ToolID, Version, ArtifactID, RemotePath string
	Overwrite                                         bool
	Timeout                                           time.Duration
}
type UploadRequest struct {
	DeviceID, AssetID, RemotePath, Mode string
	Overwrite                           bool
	Timeout                             time.Duration
}
type DownloadRequest struct {
	DeviceID, RemotePath, Name string
	Timeout                    time.Duration
}
type DownloadResult struct {
	Asset repository.Asset
	File  filetransfer.Snapshot
	Task  task.Snapshot
}
type download struct {
	mu         sync.Mutex
	path, name string
	assetID    string
	cleaned    bool
}
type Service struct {
	repo       *repository.Store
	devices    device.Query
	tasks      FileTasks
	mu         sync.RWMutex
	operations map[string]Operation
	downloads  map[string]*download
}

func NewService(repo *repository.Store, devices device.Query, tasks FileTasks) *Service {
	return &Service{repo: repo, devices: devices, tasks: tasks, operations: map[string]Operation{}, downloads: map[string]*download{}}
}
func (s *Service) Files() *repository.FileService                { return s.repo.Files() }
func (s *Service) Tools() *repository.ToolService                { return s.repo.Tools() }
func (s *Service) Devices() device.Query                         { return s.devices }
func (s *Service) TaskSnapshot(id string) (task.Snapshot, error) { return s.tasks.TaskSnapshot(id) }
func (s *Service) FileSnapshot(id string) (filetransfer.Snapshot, error) {
	return s.tasks.FileSnapshot(id)
}
func (s *Service) WaitTaskResult(ctx context.Context, id string) (task.Result, error) {
	return s.tasks.WaitTaskResult(ctx, id)
}
func (s *Service) ResendTask(ctx context.Context, id string) error {
	return s.tasks.ResendTask(ctx, id)
}
func (s *Service) Operation(id string) (Operation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.operations[id]
	if !ok {
		return Operation{}, task.ErrTaskNotFound
	}
	return v, nil
}
func (s *Service) save(v Operation, dispatchErr error) (Operation, error) {
	if v.TaskID == "" {
		return Operation{}, dispatchErr
	}
	f, e := s.tasks.FileSnapshot(v.TaskID)
	if e == nil {
		v.TransferID = f.TransferID
	}
	s.mu.Lock()
	s.operations[v.TaskID] = v
	s.mu.Unlock()
	return v, errors.Join(dispatchErr, e)
}
func (s *Service) online(id string) (device.Snapshot, error) {
	d, e := s.devices.Get(id)
	if e != nil {
		return d, e
	}
	if d.Status != device.Online || d.CurrentSession == nil {
		return d, ErrOffline
	}
	for _, cap := range d.Registration.Capabilities {
		if cap == "file" {
			return d, nil
		}
	}
	return d, fmt.Errorf("%w: file capability missing", ErrIncompatible)
}
func (s *Service) Compatibility(deviceID, toolID, version string) ([]Candidate, error) {
	d, e := s.devices.Get(deviceID)
	if e != nil {
		return nil, e
	}
	t, e := s.Tools().Get(toolID)
	if e != nil {
		return nil, e
	}
	v, e := s.Tools().Version(toolID, version)
	if e != nil {
		return nil, e
	}
	if t.Archived || v.Archived {
		return nil, repository.ErrArchived
	}
	out := make([]Candidate, 0, len(v.Artifacts))
	for _, a := range v.Artifacts {
		out = append(out, Candidate{a, repository.Match(a, d.Registration)})
	}
	return out, nil
}
func (s *Service) Deploy(ctx context.Context, q DeployRequest) (Operation, error) {
	if e := ctx.Err(); e != nil {
		return Operation{}, e
	}
	d, e := s.online(q.DeviceID)
	if e != nil {
		return Operation{}, e
	}
	v, e := s.Tools().Version(q.ToolID, q.Version)
	if e != nil {
		return Operation{}, e
	}
	var selected string
	for _, a := range v.Artifacts {
		if q.ArtifactID != "" && a.ID != q.ArtifactID {
			continue
		}
		if repository.Match(a, d.Registration).Status == repository.Compatible {
			if selected != "" {
				return Operation{}, ErrAmbiguous
			}
			selected = a.ID
		}
	}
	if selected == "" {
		return Operation{}, ErrIncompatible
	}
	op := Operation{DeviceID: q.DeviceID, SessionID: d.CurrentSession.ID, ToolID: q.ToolID, Version: q.Version, ArtifactID: selected}
	e = s.Tools().WithArtifact(ctx, q.ToolID, q.Version, selected, func(a repository.Artifact, asset repository.Asset, path string) error {
		op.AssetID = asset.ID
		id, err := s.tasks.CreateUpload(ctx, q.DeviceID, filetransfer.UploadRequest{SourcePath: path, RemotePath: q.RemotePath, Mode: a.Mode, Overwrite: q.Overwrite, Timeout: q.Timeout, ExpectedSessionID: op.SessionID, Expected: &filetransfer.ContentMetadata{Size: asset.Size, SHA256: asset.SHA256}})
		op.TaskID = id
		return err
	})
	return s.save(op, e)
}
func (s *Service) Upload(ctx context.Context, q UploadRequest) (Operation, error) {
	d, e := s.online(q.DeviceID)
	if e != nil {
		return Operation{}, e
	}
	op := Operation{DeviceID: q.DeviceID, SessionID: d.CurrentSession.ID, AssetID: q.AssetID}
	e = s.Files().WithAsset(ctx, q.AssetID, func(a repository.Asset, path string) error {
		var err error
		op.TaskID, err = s.tasks.CreateUpload(ctx, q.DeviceID, filetransfer.UploadRequest{SourcePath: path, RemotePath: q.RemotePath, Mode: q.Mode, Overwrite: q.Overwrite, Timeout: q.Timeout, ExpectedSessionID: op.SessionID, Expected: &filetransfer.ContentMetadata{Size: a.Size, SHA256: a.SHA256}})
		return err
	})
	return s.save(op, e)
}
func (s *Service) Download(ctx context.Context, q DownloadRequest) (Operation, error) {
	d, e := s.online(q.DeviceID)
	if e != nil {
		return Operation{}, e
	}
	path, e := s.repo.NewDownloadTarget()
	if e != nil {
		return Operation{}, e
	}
	id, e := s.tasks.CreateDownload(ctx, q.DeviceID, filetransfer.DownloadRequest{RemotePath: q.RemotePath, ResultName: q.Name, TargetPath: path, Timeout: q.Timeout, ExpectedSessionID: d.CurrentSession.ID})
	if id == "" {
		cleanup := s.repo.ReleaseDownloadTarget(path)
		return Operation{}, errors.Join(e, cleanup)
	}
	s.mu.Lock()
	s.downloads[id] = &download{path: path, name: q.Name}
	s.mu.Unlock()
	return s.save(Operation{TaskID: id, DeviceID: q.DeviceID, SessionID: d.CurrentSession.ID}, e)
}

// CompleteDownload explicitly imports the local commit, independently of RESULT.
// Repeated calls return the same asset identity, including after archival.
func (s *Service) CompleteDownload(ctx context.Context, id string) (DownloadResult, error) {
	s.mu.RLock()
	d := s.downloads[id]
	s.mu.RUnlock()
	if d == nil {
		return DownloadResult{}, task.ErrTaskNotFound
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	f, e := s.tasks.FileSnapshot(id)
	if e != nil {
		return DownloadResult{}, e
	}
	t, e := s.tasks.TaskSnapshot(id)
	if e != nil {
		return DownloadResult{}, e
	}
	out := DownloadResult{File: f, Task: t}
	if d.assetID != "" {
		out.Asset, e = s.Files().Get(d.assetID)
		return out, e
	}
	if !f.Committed || !f.Released || f.LocalPath != d.path {
		return out, ErrNotCommitted
	}
	file, e := os.Open(d.path)
	if e != nil {
		return out, e
	}
	a, e := s.Files().ImportVerified(ctx, d.name, file, f.Size, f.SHA256)
	file.Close()
	if e != nil {
		return out, e
	}
	d.assetID = a.ID
	out.Asset = a
	s.mu.Lock()
	op := s.operations[id]
	op.AssetID = a.ID
	s.operations[id] = op
	s.mu.Unlock()
	e = s.repo.ReleaseDownloadTarget(d.path)
	d.cleaned = e == nil
	return out, e
}

// CleanupDownload removes only this process's released staging data. A committed
// file must first be imported; callers cannot discard an unimported commit here.
func (s *Service) CleanupDownload(id string) error {
	s.mu.RLock()
	d := s.downloads[id]
	s.mu.RUnlock()
	if d == nil {
		return task.ErrTaskNotFound
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.cleaned {
		return nil
	}
	f, e := s.tasks.FileSnapshot(id)
	if e != nil {
		return e
	}
	if !f.Released || f.Committed && d.assetID == "" {
		return ErrNotCommitted
	}
	e = s.repo.ReleaseDownloadTarget(d.path)
	d.cleaned = e == nil
	return e
}
