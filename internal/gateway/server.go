package gateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"routerprobe/internal/device"
	"routerprobe/internal/filetransfer"
	"routerprobe/internal/protocol"
	"routerprobe/internal/task"
	"routerprobe/internal/tunnel"
)

type Config struct {
	HeartbeatInterval  time.Duration
	MaxControlPayload  uint32
	FileChunkSize      uint32
	Logger             *log.Logger
	DeviceHistoryLimit int // ended sessions per device; zero selects 64
}

func (c Config) withDefaults() (Config, error) {
	if c.HeartbeatInterval == 0 {
		c.HeartbeatInterval = 30 * time.Second
	}
	if c.MaxControlPayload == 0 {
		c.MaxControlPayload = protocol.MaxControlPayload
	}
	if c.FileChunkSize == 0 {
		c.FileChunkSize = 64 * 1024
	}
	if c.Logger == nil {
		c.Logger = log.Default()
	}
	if c.HeartbeatInterval < 10*time.Second || c.HeartbeatInterval > 300*time.Second || c.HeartbeatInterval%time.Second != 0 {
		return Config{}, errors.New("heartbeat interval must be an integer from 10 to 300 seconds")
	}
	if c.MaxControlPayload < 1024 || c.MaxControlPayload > protocol.MaxControlPayload {
		return Config{}, errors.New("max control payload must be from 1024 to 1048576")
	}
	if c.FileChunkSize < 1024 || c.FileChunkSize > 512*1024 {
		return Config{}, errors.New("file chunk size must be from 1024 to 524288")
	}
	return c, nil
}

type EventType string

const (
	EventOnline       EventType = "online"
	EventDisconnected EventType = "disconnected"
)

type SessionEvent struct {
	Type      EventType
	DeviceID  string
	SessionID string
}

type session struct {
	tunnelQueue chan tunnelMessage
	tunnelDone  chan struct{}
	lifetime    chan struct{}
	done        chan struct{}
	deviceID    string
	sessionID   string
	transport   *connectionWriter
}

type connectionWriter struct {
	priority          priorityGate
	conn              net.Conn
	mu                sync.Mutex
	nextOutgoingID    uint64
	maxControlPayload uint32
	failed            error  // guarded by mu; a failed byte stream must never be reused
	onFailure         func() // installed before publication; does not acquire writer locks
}

// ErrDispatchUncertain means TASK bytes may have reached the Probe. CreateExec
// returns a non-empty task ID with this error; callers must retain that ID and
// must not automatically create a replacement side-effecting task.
var ErrDispatchUncertain = errors.New("task dispatch outcome is uncertain")

type Server struct {
	config Config

	mu           sync.Mutex
	listener     net.Listener
	sessions     map[string]*session
	connections  map[net.Conn]struct{}
	closed       bool
	events       chan SessionEvent
	tasks        *task.Service
	files        *filetransfer.Service
	devices      *device.Service
	wg           sync.WaitGroup
	tunnelStatus func(string, tunnel.Status) error // configured before Serve
}

func New(config Config) (*Server, error) {
	normalized, err := config.withDefaults()
	if err != nil {
		return nil, err
	}
	devices, err := device.New(normalized.DeviceHistoryLimit)
	if err != nil {
		return nil, err
	}
	server := &Server{
		config:      normalized,
		sessions:    make(map[string]*session),
		connections: make(map[net.Conn]struct{}),
		events:      make(chan SessionEvent, 128),
		tasks:       task.NewService(),
		devices:     devices,
	}
	server.files = filetransfer.New(server.tasks)
	return server, nil
}

func (s *Server) Events() <-chan SessionEvent {
	return s.events
}

// Devices exposes the Device Service query surface, independent of Gateway's
// transport registry and best-effort Events notifications.
func (s *Server) Devices() device.Query { return s.devices }

// Caller holds s.mu. The lock order is Gateway -> Device; Device never calls
// Gateway. Neither lock spans socket Close, writer acquisition, or file I/O.
func (s *Server) endSessionLocked(active *session, reason device.EndReason) bool {
	if s.sessions[active.deviceID] != active {
		return false
	}
	if s.closed {
		reason = device.ServerClosed
	}
	delete(s.sessions, active.deviceID)
	if active.lifetime != nil {
		close(active.lifetime)
	}
	s.devices.End(active.deviceID, active.sessionID, reason, time.Now())
	s.emit(SessionEvent{Type: EventDisconnected, DeviceID: active.deviceID, SessionID: active.sessionID})
	return true
}

func (s *Server) endSession(active *session, reason device.EndReason) {
	s.mu.Lock()
	ended := s.endSessionLocked(active, reason)
	s.mu.Unlock()
	if ended {
		s.config.Logger.Printf("state=DISCONNECTED device_id=%s session_id=%s", active.deviceID, active.sessionID)
	}
}

// Timestamp activity in the same order as publication/end. In particular an
// old Reader must not record activity after a replacement's end timestamp was
// chosen but before the Device Service applies that replacement.
func (s *Server) recordActivity(active *session) time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	at := time.Now()
	if s.sessions[active.deviceID] == active {
		s.devices.Seen(active.deviceID, active.sessionID, at)
	}
	return at
}

func (s *Server) Serve(listener net.Listener) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return net.ErrClosed
	}
	s.listener = listener
	s.mu.Unlock()

	for {
		conn, err := listener.Accept()
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed || errors.Is(err, net.ErrClosed) {
				return nil
			}
			var temporary interface{ Temporary() bool }
			if errors.As(err, &temporary) && temporary.Temporary() {
				continue
			}
			return err
		}
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			_ = conn.Close()
			return nil
		}
		s.connections[conn] = struct{}{}
		s.wg.Add(1)
		s.mu.Unlock()
		go func() {
			defer s.wg.Done()
			defer func() {
				s.mu.Lock()
				delete(s.connections, conn)
				s.mu.Unlock()
			}()
			s.handleConnection(conn)
		}()
	}
}

func (s *Server) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	for _, active := range s.sessions {
		s.endSessionLocked(active, device.ServerClosed)
	}
	listener := s.listener
	connections := make([]net.Conn, 0, len(s.connections))
	for conn := range s.connections {
		connections = append(connections, conn)
	}
	s.mu.Unlock()

	if listener != nil {
		_ = listener.Close()
	}
	for _, conn := range connections {
		_ = conn.Close()
	}
	s.wg.Wait()
	s.files.Close()
	return nil
}

func (s *Server) Disconnect(deviceID string) bool {
	s.mu.Lock()
	active := s.sessions[deviceID]
	if active != nil {
		s.endSessionLocked(active, device.RequestedDisconnect)
	}
	s.mu.Unlock()
	if active == nil {
		return false
	}
	return active.transport.conn.Close() == nil
}

func (s *Server) CreateExec(ctx context.Context, deviceID string, request task.ExecRequest) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	s.mu.Lock()
	active := s.sessions[deviceID]
	s.mu.Unlock()
	if active == nil {
		return "", fmt.Errorf("device %q is offline", deviceID)
	}
	spec, err := s.tasks.NewExec(deviceID, request)
	if err != nil {
		return "", err
	}
	messageID, err := s.dispatchExec(active, spec)
	if err != nil {
		if messageID != 0 {
			return spec.ID, err
		}
		s.tasks.Remove(spec.ID)
		return "", err
	}
	return spec.ID, nil
}

// ResendTask retransmits the saved specification under the same business ID.
// It never creates a replacement task, even after an uncertain write.
func (s *Server) ResendTask(ctx context.Context, taskID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	snapshot, err := s.tasks.Snapshot(taskID)
	if err != nil {
		return err
	}
	if snapshot.State == task.StateRejected {
		return task.ErrTaskRejected
	}
	s.mu.Lock()
	active := s.sessions[snapshot.Spec.DeviceID]
	s.mu.Unlock()
	if active == nil {
		return fmt.Errorf("device %q is offline", snapshot.Spec.DeviceID)
	}
	_, err = s.dispatchExec(active, snapshot.Spec)
	return err
}

func (s *Server) dispatchExec(active *session, spec task.Spec) (uint64, error) {
	return s.dispatchChecked(active, spec, false)
}

func (s *Server) dispatchChecked(active *session, spec task.Spec, requireCurrent bool) (uint64, error) {
	wireExec := taskMessage{TaskID: spec.ID, Type: spec.Type, CreatedAt: spec.CreatedAt, Timeout: spec.Timeout, Params: execTaskParams{Command: spec.Command, Cwd: spec.Cwd, Env: spec.Env}}
	var wire interface{} = wireExec
	if spec.Type != "exec" {
		wire = map[string]interface{}{"task_id": spec.ID, "type": spec.Type, "created_at": spec.CreatedAt, "timeout": spec.Timeout, "params": spec.Params}
	}
	messageID, err := active.transport.sendJSON(protocol.TypeTask, 0, wire, func(messageID uint64) error {
		if requireCurrent {
			s.mu.Lock()
			defer s.mu.Unlock()
			if s.sessions[spec.DeviceID] != active {
				return ErrSessionChanged
			}
		}
		return s.tasks.MarkDispatched(spec.ID, active.sessionID, messageID)
	})
	if err != nil {
		if messageID != 0 {
			return messageID, fmt.Errorf("%w: task_id=%s session_id=%s message_id=%d: %w",
				ErrDispatchUncertain, spec.ID, active.sessionID, messageID, err)
		}
		return 0, fmt.Errorf("dispatch task: %w", err)
	}
	s.config.Logger.Printf("sent=TASK device_id=%s session_id=%s task_id=%s type=%s", spec.DeviceID, active.sessionID, spec.ID, spec.Type)
	return messageID, nil
}

func (s *Server) WaitTaskResult(ctx context.Context, taskID string) (task.Result, error) {
	return s.tasks.WaitResult(ctx, taskID)
}

func (s *Server) TaskSnapshot(taskID string) (task.Snapshot, error) {
	return s.tasks.Snapshot(taskID)
}

func (s *Server) emit(event SessionEvent) {
	select {
	case s.events <- event:
	default:
	}
}

func newSessionID() (string, error) {
	data := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, data); err != nil {
		return "", err
	}
	return "sess_" + hex.EncodeToString(data), nil
}

type registerAckSuccess struct {
	ReplyTo           uint64 `json:"reply_to"`
	Success           bool   `json:"success"`
	SessionID         string `json:"session_id"`
	HeartbeatInterval int64  `json:"heartbeat_interval"`
	ServerTime        int64  `json:"server_time"`
	MaxControlPayload uint32 `json:"max_control_payload"`
	FileChunkSize     uint32 `json:"file_chunk_size"`
}

type registerAckFailure struct {
	ReplyTo    uint64 `json:"reply_to"`
	Success    bool   `json:"success"`
	ErrorCode  string `json:"error_code"`
	Message    string `json:"message,omitempty"`
	RetryAfter int    `json:"retry_after,omitempty"`
}

type heartbeatAck struct {
	ReplyTo    uint64 `json:"reply_to"`
	ServerTime int64  `json:"server_time"`
}

type errorResponse struct {
	ReplyTo uint64 `json:"reply_to"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type execTaskParams struct {
	Command string            `json:"command"`
	Cwd     string            `json:"cwd,omitempty"`
	Env     map[string]string `json:"env"`
}

type taskMessage struct {
	TaskID    string         `json:"task_id"`
	Type      string         `json:"type"`
	CreatedAt int64          `json:"created_at"`
	Timeout   uint32         `json:"timeout"`
	Params    execTaskParams `json:"params"`
}

func (w *connectionWriter) sendJSON(messageType uint8, flags uint16, value interface{}, beforeWrite func(uint64) error) (uint64, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return 0, err
	}
	if uint64(len(payload)) > uint64(w.maxControlPayload) {
		return 0, fmt.Errorf("payload length %d exceeds %d", len(payload), w.maxControlPayload)
	}
	w.priority.lock(false)
	defer w.priority.unlock()
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.failed != nil {
		return 0, w.failed
	}
	messageID := w.nextOutgoingID
	if messageID == 0 || messageID == ^uint64(0) {
		return 0, w.failLocked(errors.New("message_id exhausted"))
	}
	if err := w.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return 0, w.failLocked(err)
	}
	if beforeWrite != nil {
		if err := beforeWrite(messageID); err != nil {
			return 0, err
		}
	}
	frame := protocol.Frame{
		Header:  protocol.Header{Version: protocol.Version1, Type: messageType, Flags: flags, MessageID: messageID},
		Payload: payload,
	}
	if err := protocol.WriteFrame(w.conn, frame); err != nil {
		// A nonzero ID means a transport write was attempted, even when the
		// transport reports zero bytes. Delivery cannot safely be inferred.
		return messageID, w.failLocked(err)
	}
	w.nextOutgoingID = messageID + 1
	return messageID, nil
}

func (w *connectionWriter) failLocked(err error) error {
	w.failed = err
	if w.onFailure != nil {
		w.onFailure()
	}
	_ = w.conn.Close()
	return err
}

func (s *Server) handleConnection(conn net.Conn) {
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.SetWriteBuffer(64 * 1024)
	}
	defer conn.Close()
	writer := &connectionWriter{conn: conn, nextOutgoingID: 1, maxControlPayload: s.config.MaxControlPayload}
	decoder := protocol.NewDecoder(s.config.MaxControlPayload)
	buffer := make([]byte, 32*1024)
	var active *session
	done := make(chan struct{})
	defer close(done)
	registered := false
	nextIncomingID := uint64(1)
	lastSeen := time.Now()
	endReason := device.ProtocolError

	defer func() {
		if active == nil {
			return
		}
		s.endSession(active, endReason)
		// Unblock the transport worker before joining it; Maintenance release does
		// not wait for this control-session-owned worker.
		_ = conn.Close()
		if active.tunnelDone != nil {
			<-active.tunnelDone
		}
	}()

	for {
		deadline := lastSeen.Add(30 * time.Second)
		if registered {
			deadline = lastSeen.Add(3 * s.config.HeartbeatInterval)
		}
		_ = conn.SetReadDeadline(deadline)
		n, readErr := conn.Read(buffer)
		if n > 0 {
			frames, decodeErr := decoder.Feed(buffer[:n])
			for _, frame := range frames {
				if frame.Header.MessageID != nextIncomingID {
					s.config.Logger.Printf("protocol_error=INVALID_MESSAGE_ID remote=%s got=%d want=%d", conn.RemoteAddr(), frame.Header.MessageID, nextIncomingID)
					return
				}
				if nextIncomingID == ^uint64(0) {
					return // Never wrap and accept the reserved message_id zero.
				}
				nextIncomingID++
				if !registered {
					if frame.Header.Flags != 0 {
						_ = s.sendError(writer, frame.Header.MessageID, "INVALID_PAYLOAD", "REGISTER flags must be zero")
						return
					}
					if frame.Header.Type != protocol.TypeRegister {
						_ = s.sendError(writer, frame.Header.MessageID, "UNSUPPORTED_TYPE", "REGISTER must be the first message")
						return
					}
					register, err := parseRegister(frame.Payload)
					if err != nil {
						failure := registerAckFailure{
							ReplyTo: frame.Header.MessageID, Success: false, ErrorCode: "INVALID_REGISTER",
							Message: err.Error(), RetryAfter: 30,
						}
						_, _ = writer.sendJSON(protocol.TypeRegisterAck, protocol.FlagResponse, failure, nil)
						return
					}
					sessionID, err := newSessionID()
					if err != nil {
						s.config.Logger.Printf("session_id_error=%v", err)
						return
					}
					candidate := &session{deviceID: register.DeviceID, sessionID: sessionID, transport: writer, done: done, lifetime: make(chan struct{})}
					writer.onFailure = func() { s.endSession(candidate, device.WriteError) }
					ack := registerAckSuccess{
						ReplyTo: frame.Header.MessageID, Success: true, SessionID: sessionID,
						HeartbeatInterval: int64(s.config.HeartbeatInterval / time.Second),
						ServerTime:        time.Now().Unix(), MaxControlPayload: s.config.MaxControlPayload,
						FileChunkSize: s.config.FileChunkSize,
					}
					if _, err := writer.sendJSON(protocol.TypeRegisterAck, protocol.FlagResponse, ack, nil); err != nil {
						return
					}
					// Publish only after the complete registration response. The
					// registry lock orders concurrent replacements without holding
					// a server-wide lock across network writes or Close.
					s.mu.Lock()
					if s.closed {
						s.mu.Unlock()
						return
					}
					previous := s.sessions[register.DeviceID]
					for _, capability := range register.Capabilities {
						if capability == "tunnel" {
							candidate.tunnelQueue = make(chan tunnelMessage, 64)
							candidate.tunnelDone = make(chan struct{})
							go s.runTunnelControl(candidate)
							break
						}
					}
					if previous != nil && previous.lifetime != nil {
						close(previous.lifetime)
					}
					s.sessions[register.DeviceID] = candidate
					active = candidate
					registered = true
					lastSeen = time.Now()
					s.devices.Publish(device.Registration(register), sessionID, lastSeen)
					s.emit(SessionEvent{Type: EventOnline, DeviceID: active.deviceID, SessionID: active.sessionID})
					s.mu.Unlock()
					if previous != nil {
						_ = previous.transport.conn.Close()
					}
					s.config.Logger.Printf("state=ONLINE device_id=%s session_id=%s", active.deviceID, active.sessionID)
					continue
				}

				switch frame.Header.Type {
				case protocol.TypeTunnelStatus:
					var status tunnel.Status
					if frame.Header.Flags != 0 || !protocol.ValidUnicodeJSON(frame.Payload) || json.Unmarshal(frame.Payload, &status) != nil || s.tunnelStatus == nil {
						return
					}
					if err := s.tunnelStatus(active.sessionID, status); err != nil {
						return
					}
					lastSeen = s.recordActivity(active)
				case protocol.TypeHeartbeat:
					if frame.Header.Flags != 0 {
						_ = s.sendError(writer, frame.Header.MessageID, "INVALID_PAYLOAD", "HEARTBEAT flags must be zero")
						return
					}
					if err := validateHeartbeat(frame.Payload); err != nil {
						_ = s.sendError(writer, frame.Header.MessageID, "INVALID_PAYLOAD", err.Error())
						return
					}
					lastSeen = s.recordActivity(active)
					ack := heartbeatAck{ReplyTo: frame.Header.MessageID, ServerTime: time.Now().Unix()}
					if _, err := writer.sendJSON(protocol.TypeHeartbeatAck, protocol.FlagResponse, ack, nil); err != nil {
						return
					}
				case protocol.TypeTaskAck:
					if frame.Header.Flags != protocol.FlagResponse {
						_ = s.sendError(writer, frame.Header.MessageID, "INVALID_PAYLOAD", "TASK_ACK must set RESPONSE only")
						return
					}
					ack, err := parseTaskAck(frame.Payload)
					if err != nil {
						_ = s.sendError(writer, frame.Header.MessageID, "INVALID_PAYLOAD", err.Error())
						return
					}
					if err := s.tasks.HandleAck(active.deviceID, active.sessionID, ack); err != nil {
						_ = s.sendError(writer, frame.Header.MessageID, "INVALID_PAYLOAD", err.Error())
						return
					}
					lastSeen = s.recordActivity(active)
					s.config.Logger.Printf("received=TASK_ACK device_id=%s task_id=%s accepted=%t", active.deviceID, ack.TaskID, ack.Accepted)
					if err := s.files.OnAck(active.sessionID, ack); err != nil {
						return
					}
				case protocol.TypeFileBegin, protocol.TypeFileChunk, protocol.TypeFileEnd, protocol.TypeFileAck:
					if err := s.files.Route(active.sessionID, frame); err != nil {
						_ = s.sendError(writer, frame.Header.MessageID, "TRANSFER_ERROR", err.Error())
						return
					}
					lastSeen = s.recordActivity(active)
				case protocol.TypeTaskResult:
					if frame.Header.Flags != 0 {
						_ = s.sendError(writer, frame.Header.MessageID, "INVALID_PAYLOAD", "TASK_RESULT flags must be zero")
						return
					}
					result, err := parseTaskResult(frame.Payload)
					if err != nil {
						_ = s.sendError(writer, frame.Header.MessageID, "INVALID_PAYLOAD", err.Error())
						return
					}
					if err := s.files.ValidateResult(result); err != nil {
						_ = s.sendError(writer, frame.Header.MessageID, "INVALID_PAYLOAD", err.Error())
						return
					}
					if err := s.tasks.HandleResult(active.deviceID, result); err != nil {
						_ = s.sendError(writer, frame.Header.MessageID, "INVALID_PAYLOAD", err.Error())
						return
					}
					lastSeen = s.recordActivity(active)
					s.config.Logger.Printf("received=TASK_RESULT device_id=%s task_id=%s status=%s exit_code=%d", active.deviceID, result.TaskID, result.Status, result.ExitCode)
				default:
					_ = s.sendError(writer, frame.Header.MessageID, "UNSUPPORTED_TYPE", fmt.Sprintf("message type 0x%02X is not supported", frame.Header.Type))
					return
				}
			}
			if decodeErr != nil {
				var frameErr *protocol.FrameError
				if errors.As(decodeErr, &frameErr) && frameErr.Header != nil && frameErr.Header.MessageID != 0 {
					_ = s.sendError(writer, frameErr.Header.MessageID, frameErr.Code, frameErr.Detail)
				}
				s.config.Logger.Printf("protocol_error=%v remote=%s", decodeErr, conn.RemoteAddr())
				return
			}
		}
		if readErr != nil {
			endReason = device.Disconnected
			var timeout net.Error
			if errors.As(readErr, &timeout) && timeout.Timeout() {
				endReason = device.HeartbeatTimeout
			}
			return
		}
	}
}

func (s *Server) sendError(writer *connectionWriter, replyTo uint64, errorCode, message string) error {
	_, err := writer.sendJSON(protocol.TypeError, protocol.FlagResponse, errorResponse{
		ReplyTo: replyTo,
		Code:    errorCode,
		Message: message,
	}, nil)
	return err
}

func Run(ctx context.Context, address string, config Config) error {
	server, err := New(config)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	server.config.Logger.Printf("listening=%s", listener.Addr())
	go func() {
		<-ctx.Done()
		_ = server.Close()
	}()
	return server.Serve(listener)
}
