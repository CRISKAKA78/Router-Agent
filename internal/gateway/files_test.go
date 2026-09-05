package gateway

import (
	"sync"
	"testing"
	"time"
)

func TestControlWinsAtChunkBoundary(t *testing.T) {
	var g priorityGate
	g.lock(true) // a chunk is already on the wire
	order := make(chan string, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); g.lock(true); order <- "chunk"; g.unlock() }()
	go func() { defer wg.Done(); g.lock(false); order <- "control"; g.unlock() }()
	deadline := time.Now().Add(time.Second)
	for {
		g.mu.Lock()
		pending := g.controls
		g.mu.Unlock()
		if pending > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("control did not queue")
		}
		time.Sleep(time.Millisecond)
	}
	g.unlock()
	if got := <-order; got != "control" {
		t.Fatalf("priority: %s", got)
	}
	if got := <-order; got != "chunk" {
		t.Fatal(got)
	}
	wg.Wait()
}
