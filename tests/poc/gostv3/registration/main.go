// Command registration runs one isolated first-packet serial gateway.
// It is a PoC harness, not a Management Server endpoint or process supervisor.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"time"

	"routerprobe/internal/serialauth"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:0", "loopback TCP listen address for this PoC")
	backend := flag.String("backend", "", "fixed private loopback GOST TCP serial endpoint")
	minutes := flag.Int("minutes", 240, "lease minutes; 0 means unlimited until stopped")
	flag.Parse()
	if err := run(*listen, *backend, *minutes); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(addr, backend string, minutes int) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		return fmt.Errorf("PoC listener must be literal loopback")
	}
	if minutes < 0 || int64(minutes) > int64((1<<63-1)/time.Minute) {
		return fmt.Errorf("invalid lease minutes")
	}
	// Environment avoids putting the credential in process arguments or logs.
	token := os.Getenv("RMP_SERIAL_REGISTRATION_TOKEN")
	os.Unsetenv("RMP_SERIAL_REGISTRATION_TOKEN")
	if strings.TrimSpace(token) != token {
		return fmt.Errorf("invalid registration token")
	}
	cfg := serialauth.Config{Backend: backend, Token: token}
	if minutes > 0 {
		cfg.ExpiresAt = time.Now().Add(time.Duration(minutes) * time.Minute)
	}
	gate, err := serialauth.New(cfg)
	if err != nil {
		return err
	}
	defer gate.Close()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	fmt.Printf("LISTEN %s\n", ln.Addr())
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	return gate.Serve(ctx, ln)
}
