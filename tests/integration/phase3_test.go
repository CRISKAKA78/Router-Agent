package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"routerprobe/internal/filetransfer"
	"routerprobe/internal/gateway"
	"routerprobe/internal/management"
	"routerprobe/internal/protocol"
	"routerprobe/internal/repository"
	"routerprobe/internal/task"
	"strings"
	"sync"
	"testing"
	"time"
)

func phase3Service(t *testing.T, g *gateway.Server) *management.Service {
	t.Helper()
	r, e := repository.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { r.Close() })
	return management.NewService(r, g.Devices(), g)
}
func phase3Tool(t *testing.T, s *management.Service, data []byte) (repository.Tool, repository.Version, repository.Asset) {
	t.Helper()
	a, e := s.Files().Import(context.Background(), "test-tool", bytes.NewReader(data))
	if e != nil {
		t.Fatal(e)
	}
	tool, e := s.Tools().Create("test-tool", "")
	if e != nil {
		t.Fatal(e)
	}
	v, e := s.Tools().Publish(tool.ID, "test-v1", []repository.ArtifactSpec{{AssetID: a.ID, Platform: "linux", Mode: "0755", Rules: repository.Rules{Arch: []string{"any"}, Libc: []string{"any"}, RequiredCapabilities: []string{"file"}}}})
	if e != nil {
		t.Fatal(e)
	}
	return tool, v, a
}
func completePhase3Download(t *testing.T, s *management.Service, id string) management.DownloadResult {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		r, e := s.CompleteDownload(context.Background(), id)
		if e == nil {
			return r
		}
		if !errors.Is(e, management.ErrNotCommitted) || time.Now().After(deadline) {
			t.Fatal(e)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestRepositoryRealProbeToolDeploymentAndDownload(t *testing.T) {
	var mu sync.Mutex
	var captured []protocol.Frame
	g, device, _ := relayedFileServer(t, func(fromProbe bool, f *protocol.Frame) bool {
		if !fromProbe && (f.Header.Type == protocol.TypeTask || f.Header.Type == protocol.TypeFileBegin) {
			copy := *f
			copy.Payload = append([]byte{}, f.Payload...)
			mu.Lock()
			captured = append(captured, copy)
			mu.Unlock()
		}
		return true
	})
	s := phase3Service(t, g)
	dir := t.TempDir()
	remote := filepath.Join(dir, "tool")
	marker := filepath.Join(dir, "marker")
	data := []byte("#!/bin/sh\nprintf executed > '" + marker + "'\n")
	tool, v, a := phase3Tool(t, s, data)
	op, e := s.Deploy(context.Background(), management.DeployRequest{DeviceID: device, ToolID: tool.ID, Version: v.Version, RemotePath: remote, Timeout: 10 * time.Second})
	if e != nil {
		t.Fatal(e)
	}
	if r := fileResult(t, g, op.TaskID); r.Status != "success" {
		t.Fatal(r)
	}
	if op.TransferID == "" || op.AssetID != a.ID || op.ArtifactID != v.Artifacts[0].ID {
		t.Fatal(op)
	}
	got, e := os.ReadFile(remote)
	if e != nil || !bytes.Equal(got, data) {
		t.Fatal(e)
	}
	st, e := os.Stat(remote)
	if e != nil || st.Mode().Perm() != 0755 {
		t.Fatal(st, e)
	}
	if _, e = os.Stat(marker); !os.IsNotExist(e) {
		t.Fatal("deploy executed tool", e)
	}
	result := runExec(t, g, device, task.ExecRequest{Command: "'" + remote + "'", Timeout: time.Second})
	if result.Status != "success" {
		t.Fatal(result)
	}
	if markerBytes, e := os.ReadFile(marker); e != nil || string(markerBytes) != "executed" {
		t.Fatal(e)
	}
	download, e := s.Download(context.Background(), management.DownloadRequest{DeviceID: device, RemotePath: remote, Name: "retrieved-tool", Timeout: 10 * time.Second})
	if e != nil {
		t.Fatal(e)
	}
	if r := fileResult(t, g, download.TaskID); r.Status != "success" {
		t.Fatal(r)
	}
	imported := completePhase3Download(t, s, download.TaskID)
	if imported.Asset.SHA256 != a.SHA256 || imported.Asset.ID == a.ID || !imported.File.Committed {
		t.Fatal(imported)
	}
	again, e := s.CompleteDownload(context.Background(), download.TaskID)
	if e != nil || again.Asset.ID != imported.Asset.ID {
		t.Fatal(again, e)
	}
	mu.Lock()
	frames := append([]protocol.Frame{}, captured...)
	mu.Unlock()
	for _, f := range frames {
		for _, forbidden := range []string{"asset_id", "tool_id", "artifact_id", "local_asset", "required_capabilities", "ExpectedSessionID"} {
			if strings.Contains(string(f.Payload), forbidden) {
				t.Fatal("repository leaked to Probe", string(f.Payload))
			}
		}
	}
	taskSnap, e := g.TaskSnapshot(op.TaskID)
	if e != nil {
		t.Fatal(e)
	}
	var params filetransfer.Params
	if e = json.Unmarshal(taskSnap.Spec.Params, &params); e != nil || params.TransferID != op.TransferID || params.SHA256 != a.SHA256 || params.Size != a.Size {
		t.Fatal(params, e)
	}
	// Archive must not disable querying/replaying an already accepted task.
	if e = s.Tools().ArchiveVersion(tool.ID, v.Version); e != nil {
		t.Fatal(e)
	}
	if e = s.Files().Archive(a.ID); e != nil {
		t.Fatal(e)
	}
	os.WriteFile(remote, []byte("preserve subsequent contents"), 0600)
	if e = s.ResendTask(context.Background(), op.TaskID); e != nil {
		t.Fatal(e)
	}
	if e = s.ResendTask(context.Background(), download.TaskID); e != nil {
		t.Fatal(e)
	}
	runExec(t, g, device, task.ExecRequest{Command: "true", Timeout: time.Second})
	got, _ = os.ReadFile(remote)
	if string(got) != "preserve subsequent contents" {
		t.Fatal("resend performed file side effect")
	}
	if _, e = s.Deploy(context.Background(), management.DeployRequest{DeviceID: device, ToolID: tool.ID, Version: v.Version, RemotePath: remote, Timeout: time.Second}); !errors.Is(e, repository.ErrArchived) {
		t.Fatal(e)
	}
}

func TestRepositoryRealProbeCompatibilityAndBinaryAsset(t *testing.T) {
	g, id, _ := fileServer(t)
	s := phase3Service(t, g)
	data := bytes.Repeat([]byte{0, 1, 128, 255}, 50000)
	tool, v, a := phase3Tool(t, s, data)
	d, e := s.Devices().Get(id)
	if e != nil {
		t.Fatal(e)
	}
	if d.Registration.Libc != "" || d.Registration.Model != "" || d.Registration.Kernel == "" {
		t.Fatal("real Probe must report kernel; libc/model still require a template")
	}
	for _, tc := range []struct {
		version, kernel string
		want            repository.Compatibility
	}{
		{"exact-kernel", d.Registration.Kernel, repository.Compatible},
		{"other-kernel", d.Registration.Kernel + "-different", repository.Incompatible},
	} {
		kernelSpec := repository.ArtifactSpec{AssetID: a.ID, Platform: "linux", Mode: "0755", Rules: repository.Rules{Arch: []string{d.Registration.Arch}, Libc: []string{"any"}, Kernels: []string{tc.kernel}}}
		if _, e := s.Tools().Publish(tool.ID, tc.version, []repository.ArtifactSpec{kernelSpec}); e != nil {
			t.Fatal(e)
		}
		matches, e := s.Compatibility(id, tool.ID, tc.version)
		if e != nil || len(matches) != 1 || matches[0].Match.Status != tc.want {
			t.Fatalf("kernel match %s: %#v %v", tc.version, matches, e)
		}
	}
	spec := repository.ArtifactSpec{AssetID: a.ID, Platform: "linux", Mode: "0755", Rules: repository.Rules{Arch: []string{d.Registration.Arch}, Libc: []string{"uclibc"}}}
	if _, e = s.Tools().Publish(tool.ID, "requires-libc", []repository.ArtifactSpec{spec}); e != nil {
		t.Fatal(e)
	}
	matches, e := s.Compatibility(id, tool.ID, "requires-libc")
	if e != nil || matches[0].Match.Status != repository.Unknown {
		t.Fatal(matches, e)
	}
	remote := filepath.Join(t.TempDir(), "binary")
	if op, e := s.Deploy(context.Background(), management.DeployRequest{DeviceID: id, ToolID: tool.ID, Version: "requires-libc", RemotePath: remote, Timeout: time.Second}); !errors.Is(e, management.ErrIncompatible) || op.TaskID != "" {
		t.Fatal(op, e)
	}
	op, e := s.Deploy(context.Background(), management.DeployRequest{DeviceID: id, ToolID: tool.ID, Version: v.Version, RemotePath: remote, Timeout: 10 * time.Second})
	if e != nil {
		t.Fatal(e)
	}
	if r := fileResult(t, g, op.TaskID); r.Status != "success" {
		t.Fatal(r)
	}
	got, e := os.ReadFile(remote)
	if e != nil || !bytes.Equal(got, data) {
		t.Fatal("binary asset", e)
	}
}

func TestRepositoryRealProbeCommitAckLossAndInterruptedDeployment(t *testing.T) {
	for _, which := range []string{"download-done-lost", "upload-interrupted"} {
		t.Run(which, func(t *testing.T) {
			var once sync.Once
			hit := make(chan struct{})
			g, id, _ := relayedFileServer(t, func(fromProbe bool, f *protocol.Frame) bool {
				match := !fromProbe && f.Header.Type == protocol.TypeFileChunk && which == "upload-interrupted"
				if which == "download-done-lost" && !fromProbe && f.Header.Type == protocol.TypeFileAck {
					var ack filetransfer.Ack
					json.Unmarshal(f.Payload, &ack)
					match = ack.Status == "done"
				}
				pass := true
				if match {
					once.Do(func() { close(hit); pass = false })
				}
				return pass
			})
			s := phase3Service(t, g)
			data := bytes.Repeat([]byte{0, 255, 17}, 100000)
			remote := filepath.Join(t.TempDir(), "file")
			var op management.Operation
			var e error
			if which == "download-done-lost" {
				os.WriteFile(remote, data, 0600)
				op, e = s.Download(context.Background(), management.DownloadRequest{DeviceID: id, RemotePath: remote, Name: "retained", Timeout: 10 * time.Second})
			} else {
				tool, v, _ := phase3Tool(t, s, data)
				op, e = s.Deploy(context.Background(), management.DeployRequest{DeviceID: id, ToolID: tool.ID, Version: v.Version, RemotePath: remote, Timeout: 10 * time.Second})
			}
			if e != nil {
				t.Fatal(e)
			}
			select {
			case <-hit:
			case <-time.After(5 * time.Second):
				t.Fatal("fault not reached")
			}
			result := fileResult(t, g, op.TaskID)
			if result.Status != "failed" {
				t.Fatal(result)
			}
			if which == "download-done-lost" {
				r := completePhase3Download(t, s, op.TaskID)
				if r.Task.State != task.StateFailed || !r.File.Committed || r.Asset.Size != int64(len(data)) {
					t.Fatal(r)
				}
			} else {
				if _, e = os.Stat(remote); !os.IsNotExist(e) {
					t.Fatal("partial tool published", e)
				}
			}
			if e = s.ResendTask(context.Background(), op.TaskID); e != nil {
				t.Fatal(e)
			}
			runExec(t, g, id, task.ExecRequest{Command: "true", Timeout: time.Second})
			after, e := s.Operation(op.TaskID)
			if e != nil || after.TransferID != op.TransferID || after.TaskID != op.TaskID {
				t.Fatal(after, e)
			}
		})
	}
}

func TestRepositoryDeviceDeclarationTCP(t *testing.T) {
	g, e := gateway.New(gateway.Config{Logger: log.New(io.Discard, "", 0)})
	if e != nil {
		t.Fatal(e)
	}
	defer g.Close()
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	go g.Serve(listener)
	s := phase3Service(t, g)
	tool, _, asset := phase3Tool(t, s, []byte("bytes"))
	_, e = s.Tools().Publish(tool.ID, "constrained", []repository.ArtifactSpec{{AssetID: asset.ID, Platform: "linux", Mode: "0755", Rules: repository.Rules{Arch: []string{"mipsel"}, Libc: []string{"uclibc"}, Models: []string{"F3"}, Kernels: []string{"3.10.14"}, RequiredCapabilities: []string{"file", "exec"}}}})
	if e != nil {
		t.Fatal(e)
	}
	for _, test := range []struct {
		libc string
		want repository.Compatibility
	}{{"uclibc", repository.Compatible}, {"glibc", repository.Incompatible}, {"", repository.Unknown}} {
		conn, e := net.Dial("tcp", listener.Addr().String())
		if e != nil {
			t.Fatal(e)
		}
		defer conn.Close()
		fields := map[string]interface{}{"device_id": "declaration", "probe_version": "test", "arch": "mipsel", "boot_id": "boot", "model": "F3", "kernel": "3.10.14", "capabilities": []string{"file", "exec", "managed_config_v1", "telemetry_v2"}}
		if test.libc != "" {
			fields["libc"] = test.libc
		}
		payload, _ := json.Marshal(fields)
		if e = protocol.WriteFrame(conn, protocol.Frame{Header: protocol.Header{Version: 1, Type: protocol.TypeRegister, MessageID: 1}, Payload: payload}); e != nil {
			t.Fatal(e)
		}
		if _, e = readProtocolFrame(conn); e != nil {
			t.Fatal(e)
		}
		waitOnline(t, g.Events(), "declaration", time.Second)
		got, e := s.Compatibility("declaration", tool.ID, "constrained")
		if e != nil || len(got) != 1 || got[0].Match.Status != test.want {
			t.Fatal(got, e)
		}
	}
}
