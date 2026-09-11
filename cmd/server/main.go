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

	"routerprobe/internal/api"
	"routerprobe/internal/forwarding"
	"routerprobe/internal/gateway"
	"routerprobe/internal/management"
	"routerprobe/internal/overlay"
	"routerprobe/internal/repository"
	"routerprobe/internal/tunnel"
)

func main() {
	httpAddress := flag.String("http-listen", ":8888", "trusted HTTP/WebSocket API listen address")
	apiConfig := api.Config{}
	flag.IntVar(&apiConfig.MaxRequests, "http-max-requests", 32, "maximum concurrent HTTP requests")
	flag.IntVar(&apiConfig.MaxClients, "http-max-websockets", 64, "maximum WebSocket clients")
	flag.IntVar(&apiConfig.IdempotencyCapacity, "http-idempotency-capacity", 4096, "retained mutation keys; full rejects new keys until process restart")
	flag.Int64Var(&apiConfig.MaxAssetBytes, "http-max-asset-bytes", 1<<30, "maximum streamed asset import bytes")
	flag.DurationVar(&apiConfig.RequestTimeout, "http-request-timeout", 30*time.Second, "HTTP request and creation preparation timeout")
	listenAddress := flag.String("listen", ":9000", "TCP listen address")
	heartbeatSeconds := flag.Int("heartbeat-interval", 30, "heartbeat interval in seconds (10-300)")
	maxControlPayload := flag.Uint("max-control-payload", 1024*1024, "maximum JSON control payload in bytes")
	fileChunkSize := flag.Uint("file-chunk-size", 64*1024, "negotiated file chunk size in bytes")
	repositoryDirectory := flag.String("repository-dir", repository.DefaultDirectory, "persistent local file/tool repository directory")
	templateFile := flag.String("probe-template-file", "", "template catalog path; default repository-dir/probe-templates/catalog.json")
	tunnelConfig := tunnel.Config{}
	flag.StringVar(&tunnelConfig.BindHost, "tunnel-bind", "::", "maintenance listeners bind IP")
	flag.StringVar(&tunnelConfig.AdvertisedHost, "tunnel-host", "47.119.168.150", "maintenance entry advertised host")
	flag.StringVar(&tunnelConfig.DataListen, "tunnel-data-listen", ":9001", "independent data listener")
	flag.StringVar(&tunnelConfig.DataHost, "tunnel-data-host", "47.119.168.150", "data IP or DNS host resolved by Server for Probe")
	flag.IntVar(&tunnelConfig.PortFirst, "tunnel-port-first", 20000, "first maintenance pool port")
	flag.IntVar(&tunnelConfig.PortLast, "tunnel-port-last", 20199, "last maintenance pool port")
	flag.IntVar(&tunnelConfig.PerMaintenance, "tunnel-session-connections", 8, "pending and active connections per maintenance")
	flag.IntVar(&tunnelConfig.PerDevice, "tunnel-device-connections", 8, "pending and active connections per device")
	flag.IntVar(&tunnelConfig.TotalConnections, "tunnel-total-connections", 512, "total pending and active connections")
	flag.IntVar(&tunnelConfig.MaxMaintenance, "tunnel-max-sessions", 64, "maximum unreleased maintenance sessions")
	flag.IntVar(&tunnelConfig.Handshakes, "tunnel-handshakes", 64, "maximum unpaired data handshakes")
	flag.IntVar(&tunnelConfig.History, "tunnel-history", 128, "retained closed maintenance snapshots")
	flag.DurationVar(&tunnelConfig.PendingTimeout, "tunnel-connect-timeout", 10*time.Second, "total stream establishment timeout")
	flag.DurationVar(&tunnelConfig.HandshakeTimeout, "tunnel-handshake-timeout", 5*time.Second, "unpaired data handshake timeout")
	flag.DurationVar(&tunnelConfig.PortReuseDelay, "tunnel-port-reuse-delay", 24*time.Hour, "port quarantine after local release")
	flag.DurationVar(&tunnelConfig.IdleTimeout, "tunnel-idle-timeout", 24*time.Hour, "whole-stream I/O idle timeout")
	easyTierConfigFile := flag.String("easytier-config", "", "local EasyTier Web API credential/bootstrap configuration file; optional")
	fc := forwarding.Config{}
	flag.StringVar(&fc.Binary, "forwarding-gost", "gost", "patched GOST executable; missing binary disables creation, not base Server")
	flag.StringVar(&fc.BindHost, "forwarding-bind", "0.0.0.0", "forwarding bind IPv4")
	flag.StringVar(&fc.Host, "forwarding-host", "47.119.168.150", "reachable relay and public endpoint host")
	flag.IntVar(&fc.PortFirst, "forwarding-port-first", 22000, "first forwarding endpoint/relay port")
	flag.IntVar(&fc.PortLast, "forwarding-port-last", 22399, "last forwarding endpoint/relay port")
	flag.Parse()
	easyTierConfig, configErr := overlay.LoadConfig(*easyTierConfigFile)
	if configErr != nil {
		fmt.Fprintln(os.Stderr, "invalid EasyTier local configuration")
		os.Exit(2)
	}

	if *maxControlPayload > uint(^uint32(0)) || *fileChunkSize > uint(^uint32(0)) {
		fmt.Fprintln(os.Stderr, "payload sizes must fit uint32")
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger := log.New(os.Stdout, "server ", log.LstdFlags|log.Lmicroseconds)
	err := api.Run(ctx, *listenAddress, *httpAddress, management.Config{EasyTier: easyTierConfig, Forwarding: &fc, TemplateFile: *templateFile, RepositoryDirectory: *repositoryDirectory, Tunnel: &tunnelConfig, Gateway: gateway.Config{
		HeartbeatInterval: time.Duration(*heartbeatSeconds) * time.Second,
		MaxControlPayload: uint32(*maxControlPayload),
		FileChunkSize:     uint32(*fileChunkSize),
		Logger:            logger,
	}}, apiConfig)
	if err != nil {
		logger.Printf("fatal=%v", err)
		os.Exit(1)
	}
}
