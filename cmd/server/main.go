package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"routerprobe/internal/gateway"
)

func main() {
	listenAddress := flag.String("listen", ":9000", "TCP listen address")
	heartbeatSeconds := flag.Int("heartbeat-interval", 30, "heartbeat interval in seconds (10-300)")
	maxControlPayload := flag.Uint("max-control-payload", 1024*1024, "maximum JSON control payload in bytes")
	fileChunkSize := flag.Uint("file-chunk-size", 64*1024, "negotiated file chunk size in bytes")
	flag.Parse()

	if *maxControlPayload > uint(^uint32(0)) || *fileChunkSize > uint(^uint32(0)) {
		fmt.Fprintln(os.Stderr, "payload sizes must fit uint32")
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger := log.New(os.Stdout, "server ", log.LstdFlags|log.Lmicroseconds)
	err := gateway.Run(ctx, *listenAddress, gateway.Config{
		HeartbeatInterval: time.Duration(*heartbeatSeconds) * time.Second,
		MaxControlPayload: uint32(*maxControlPayload),
		FileChunkSize:     uint32(*fileChunkSize),
		Logger:            logger,
	})
	if err != nil {
		logger.Printf("fatal=%v", err)
		os.Exit(1)
	}
}
