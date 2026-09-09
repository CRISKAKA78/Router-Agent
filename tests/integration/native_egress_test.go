package integration

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNativeEgressHTTPS(t *testing.T) {
	probe := os.Getenv("RMP_PROBE_BIN")
	if probe == "" {
		t.Skip("requires the real Linux Probe build")
	}
	binary := filepath.Join(filepath.Dir(probe), "egress_tests")
	if _, err := os.Stat(binary); err != nil {
		t.Fatal(err)
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Native egress test CA"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "root.pem")
	if err := os.WriteFile(root, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0600); err != nil {
		t.Fatal(err)
	}
	leaf := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "localhost"}, DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("::1")}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, leaf, ca, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert := tls.Certificate{Certificate: [][]byte{der, caDER}, PrivateKey: key}
	t.Run("handshake closes before certificate", func(t *testing.T) {
		listener, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer listener.Close()
		go func() {
			connection, err := listener.Accept()
			if err == nil {
				connection.Close()
			}
		}()
		_, port, _ := net.SplitHostPort(listener.Addr().String())
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, binary, "localhost", port, root, "4", "700")
		command.Env = []string{"PATH="}
		output, err := command.CombinedOutput()
		if err != nil || !strings.Contains(string(output), `"reason":"tls_handshake_failed"`) {
			t.Fatalf("incomplete handshake must not be reported as certificate rejection: %s %v", output, err)
		}
	})
	for _, tc := range []struct {
		name, family, bind, body, host, want string
		delay                                time.Duration
		chunk                                bool
		cancel                               bool
	}{
		{name: "ipv4", family: "4", bind: "127.0.0.1:0", body: "192.0.2.9", host: "localhost", want: "192.0.2.9"},
		{name: "ipv6 chunked", family: "6", bind: "[::1]:0", body: "2001:db8::9", host: "::1", want: "2001:db8::9", chunk: true},
		{name: "family mismatch", family: "4", bind: "127.0.0.1:0", body: "2001:db8::9", host: "localhost", want: "invalid_ip"},
		{name: "wrong certificate hostname", family: "4", bind: "127.0.0.1:0", body: "192.0.2.9", host: "127.0.0.1", want: "tls_certificate_failed"},
		{name: "oversized", family: "4", bind: "127.0.0.1:0", body: strings.Repeat("a", 9000), host: "localhost", want: "invalid_http_response"},
		{name: "timeout", family: "4", bind: "127.0.0.1:0", host: "localhost", want: "timeout", delay: time.Second},
		{name: "cancellation", family: "4", bind: "127.0.0.1:0", host: "localhost", want: "cancelled", delay: time.Second, cancel: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.delay > 0 {
					select {
					case <-r.Context().Done():
						return
					case <-time.After(tc.delay):
					}
				}
				if tc.chunk {
					w.(http.Flusher).Flush()
				}
				_, _ = io.WriteString(w, tc.body)
			}))
			_ = server.Listener.Close()
			server.Listener, err = net.Listen("tcp", tc.bind)
			if err != nil {
				t.Fatal(err)
			}
			server.TLS = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12}
			server.Config.ErrorLog = log.New(io.Discard, "", 0)
			server.StartTLS()
			defer server.Close()
			_, port, _ := net.SplitHostPort(server.Listener.Addr().String())
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			args := []string{tc.host, port, root, tc.family, "700"}
			if tc.cancel {
				args = append(args, "150")
			}
			command := exec.CommandContext(ctx, binary, args...)
			command.Env = []string{"PATH="}
			started := time.Now()
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("%v: %s", err, output)
			}
			var result struct{ Value, Reason string }
			if err := json.Unmarshal(output, &result); err != nil {
				t.Fatalf("%v: %s", err, output)
			}
			if result.Value != tc.want && result.Reason != tc.want {
				t.Fatalf("want %q, got %s", tc.want, output)
			}
			if result.Value != "" && result.Reason != "" {
				t.Fatalf("success with failure: %s", output)
			}
			if time.Since(started) > 2*time.Second {
				t.Fatal("request did not respect its bounded lifetime")
			}
		})
	}
}
