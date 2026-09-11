package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type client struct {
	conn   *websocket.Conn
	queue  chan string
	topics map[string]bool
}

func (a *Server) poll() {
	defer a.wg.Done()
	ticker := time.NewTicker(a.config.PollInterval)
	defer ticker.Stop()
	previous := a.app.Revisions()
	lastMaintenance := ""
	lastForwarding := ""
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			now := a.app.Revisions()
			for i, topic := range []string{"devices", "tasks", "files"} {
				if now[i] != previous[i] {
					a.broadcast(topic)
				}
			}
			previous = now
			if service := a.app.Forwarding(); service != nil {
				b, _ := json.Marshal(service.List())
				current := string(b)
				if current != lastForwarding {
					a.broadcast("forwardings")
					lastForwarding = current
				}
			}
			if service := a.app.Maintenance(); service != nil {
				b, _ := json.Marshal(service.List())
				current := string(b)
				if current != lastMaintenance {
					a.broadcast("maintenance")
					lastMaintenance = current
				}
			}
		}
	}
}
func (a *Server) broadcast(topic string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for c := range a.clients {
		if c.conn == nil || !c.topics[topic] {
			continue
		}
		select {
		case c.queue <- topic:
		default:
			c.conn.Close()
		}
	}
}
func (a *Server) events(w http.ResponseWriter, r *http.Request) {
	topics := map[string]bool{}
	q := r.URL.Query().Get("topics")
	if q == "" {
		q = "devices,tasks,files,maintenance,forwardings"
	}
	for _, v := range strings.Split(q, ",") {
		switch v {
		case "devices", "tasks", "files", "maintenance", "forwardings":
			topics[v] = true
		default:
			write(w, invalid())
			return
		}
	}
	c := &client{queue: make(chan string, 8), topics: topics}
	a.mu.Lock()
	if a.closed || len(a.clients) >= a.config.MaxClients {
		a.mu.Unlock()
		write(w, response{status: 503, code: "capacity_exhausted"})
		return
	}
	a.clients[c] = struct{}{}
	a.mu.Unlock()
	defer func() { a.mu.Lock(); delete(a.clients, c); a.mu.Unlock() }()
	up := websocket.Upgrader{HandshakeTimeout: a.config.WriteTimeout, ReadBufferSize: 1024, WriteBufferSize: 1024, Error: func(w http.ResponseWriter, r *http.Request, status int, reason error) {
		write(w, response{status: status, code: "websocket_upgrade_failed"})
	}}
	conn, e := up.Upgrade(w, r, nil)
	if e != nil {
		return
	}
	defer conn.Close()
	a.mu.Lock()
	c.conn = conn
	closed := a.closed
	a.mu.Unlock()
	if closed {
		return
	}
	conn.SetReadLimit(1024)
	conn.SetReadDeadline(time.Now().Add(45 * time.Second))
	conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(45 * time.Second)) })
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		defer conn.Close()
		for {
			_, _, e := conn.ReadMessage()
			if e != nil {
				return
			}
			conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(1008, "read-only events"), time.Now().Add(a.config.WriteTimeout))
			return
		}
	}()
	defer func() { conn.Close(); <-readDone }()
	seq := uint64(0)
	send := func(kind, topic string) error {
		seq++
		conn.SetWriteDeadline(time.Now().Add(a.config.WriteTimeout))
		return conn.WriteJSON(object{"type": kind, "topic": topic, "sequence": strconv.FormatUint(seq, 10), "time": time.Now().UTC()})
	}
	if send("resync_required", "") != nil {
		return
	}
	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-readDone:
			return
		case topic := <-c.queue:
			if send("resource_changed", topic) != nil {
				return
			}
		case <-ping.C:
			if conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(a.config.WriteTimeout)) != nil {
				return
			}
		}
	}
}
