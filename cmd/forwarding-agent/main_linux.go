// Optional Probe sidecar. stdin EOF means the owning control Session ended.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"os"
	"os/signal"
	"path/filepath"
	"routerprobe/internal/forwardagent"
	"routerprobe/internal/forwarding"
	"sync"
	"syscall"
)

func main() {
	session := flag.String("session", "", "owning Probe session")
	binary := flag.String("gost", "", "patched GOST executable")
	flag.Parse()
	if *session == "" {
		os.Exit(2)
	}
	if *binary == "" {
		exe, _ := os.Executable()
		*binary = filepath.Join(filepath.Dir(exe), "gost")
	}
	var mu sync.Mutex
	enc := json.NewEncoder(os.Stdout)
	a := forwardagent.New(*session, *binary, func(s forwarding.Status) { mu.Lock(); defer mu.Unlock(); enc.Encode(s) })
	defer a.Close()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	go func() { <-stop; os.Stdin.Close() }()
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 4096), 64<<10)
	for scanner.Scan() {
		var c forwarding.Command
		if json.Unmarshal(scanner.Bytes(), &c) != nil {
			break
		}
		a.Handle(c)
	}
}
