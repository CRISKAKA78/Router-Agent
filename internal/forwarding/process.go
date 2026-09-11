package forwarding

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

type Process struct {
	cmd    *exec.Cmd
	Done   chan struct{}
	Ready  chan struct{}
	once   sync.Once
	cancel context.CancelFunc
	dir    string
}

// StartGOST never invokes a shell. Its private config and key files are removed after Wait.
func StartGOST(ctx context.Context, binary string, config any, dir string, ready func(map[string]any) bool) (*Process, error) {
	b, e := json.Marshal(config)
	if e != nil {
		return nil, e
	}
	name := filepath.Join(dir, "gost.json")
	if e = os.WriteFile(name, b, 0600); e != nil {
		return nil, e
	}
	ctx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(ctx, binary, "-C", name)
	configureProcess(cmd)
	r, w, e := os.Pipe()
	if e != nil {
		cancel()
		return nil, e
	}
	cmd.Stdout = w
	cmd.Stderr = w
	cmd.WaitDelay = time.Second
	p := &Process{cmd: cmd, Done: make(chan struct{}), Ready: make(chan struct{}), cancel: cancel, dir: dir}
	started := make(chan error, 1)
	go func() {
		// Linux PDEATHSIG follows the creating OS thread, so keep that thread alive.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		if err := cmd.Start(); err != nil {
			w.Close()
			r.Close()
			cancel()
			started <- err
			return
		}
		w.Close()
		release, err := ownProcess(cmd)
		if err != nil {
			cmd.Process.Kill()
			cmd.Wait()
			r.Close()
			cancel()
			started <- err
			return
		}
		started <- nil
		cmd.Wait()
		release()
		cancel()
		os.RemoveAll(dir)
		close(p.Done)
	}()
	if err := <-started; err != nil {
		return nil, err
	}
	go func() {
		defer r.Close()
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 4096), 128<<10)
		for scanner.Scan() {
			var row map[string]any
			if json.Unmarshal(scanner.Bytes(), &row) == nil && ready != nil && ready(row) {
				p.once.Do(func() { close(p.Ready) })
			}
		}
		io.Copy(io.Discard, r)
	}()
	return p, nil
}
func (p *Process) Close() { p.cancel(); <-p.Done }
func Certificate(dir string) (string, error) {
	key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		return "", e
	}
	serial, e := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if e != nil {
		return "", e
	}
	cert := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "router-forwarding"}, DNSNames: []string{"router-forwarding"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().AddDate(2, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true, IsCA: true}
	der, e := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	if e != nil {
		return "", e
	}
	k, e := x509.MarshalECPrivateKey(key)
	if e != nil {
		return "", e
	}
	b := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if e = os.WriteFile(filepath.Join(dir, "cert.pem"), b, 0600); e != nil {
		return "", e
	}
	e = os.WriteFile(filepath.Join(dir, "key.pem"), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: k}), 0600)
	return string(b), e
}
func PrivateDir() (string, error) { return os.MkdirTemp("", "router-forwarding-") }
func BackendAvailable(binary string) bool {
	_, e := exec.LookPath(binary)
	return binary != "" && e == nil
}
