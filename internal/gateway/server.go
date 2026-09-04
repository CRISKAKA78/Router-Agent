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

	"routerprobe/internal/protocol"
)

type Config struct {
	HeartbeatInterval time.Duration
	MaxControlPayload uint32
	FileChunkSize     uint32
	Logger            *log.Logger
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
	deviceID  string
	sessionID string
	conn      net.Conn
}

type Server struct {
	config Config

	mu          sync.Mutex
	listener    net.Listener
	sessions    map[string]*session
	connections map[net.Conn]struct{}
	closed      bool
	events      chan SessionEvent
	wg          sync.WaitGroup
}

func New(config Config) (*Server, error) {
	normalized, err := config.withDefaults()
	if err != nil {
		return nil, err
	}
	return &Server{
		config:      normalized,
		sessions:    make(map[string]*session),
		connections: make(map[net.Conn]struct{}),
		events:      make(chan SessionEvent, 128),
	}, nil
}

func (s *Server) Events() <-chan SessionEvent {
	return s.events
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
		s.mu.Unlock()
		s.wg.Add(1)
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
	return nil
}

func (s *Server) Disconnect(deviceID string) bool {
	s.mu.Lock()
	active := s.sessions[deviceID]
	s.mu.Unlock()
	if active == nil {
		return false
	}
	return active.conn.Close() == nil
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

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()
	decoder := protocol.NewDecoder(s.config.MaxControlPayload)
	buffer := make([]byte, 32*1024)
	var active *session
	registered := false
	nextIncomingID := uint64(1)
	nextOutgoingID := uint64(1)
	lastSeen := time.Now()

	defer func() {
		if active == nil {
			return
		}
		s.mu.Lock()
		if s.sessions[active.deviceID] == active {
			delete(s.sessions, active.deviceID)
			s.mu.Unlock()
			s.emit(SessionEvent{Type: EventDisconnected, DeviceID: active.deviceID, SessionID: active.sessionID})
			s.config.Logger.Printf("state=DISCONNECTED device_id=%s session_id=%s", active.deviceID, active.sessionID)
			return
		}
		s.mu.Unlock()
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
				nextIncomingID++
				if frame.Header.Flags != 0 {
					_ = s.sendError(conn, &nextOutgoingID, frame.Header.MessageID, "INVALID_PAYLOAD", "request flags must be zero")
					return
				}

				if !registered {
					if frame.Header.Type != protocol.TypeRegister {
						_ = s.sendError(conn, &nextOutgoingID, frame.Header.MessageID, "UNSUPPORTED_TYPE", "REGISTER must be the first message")
						return
					}
					register, err := parseRegister(frame.Payload)
					if err != nil {
						failure := registerAckFailure{
							ReplyTo: frame.Header.MessageID, Success: false, ErrorCode: "INVALID_REGISTER",
							Message: err.Error(), RetryAfter: 30,
						}
						_ = s.sendJSON(conn, &nextOutgoingID, protocol.TypeRegisterAck, protocol.FlagResponse, failure)
						return
					}
					sessionID, err := newSessionID()
					if err != nil {
						s.config.Logger.Printf("session_id_error=%v", err)
						return
					}
					candidate := &session{deviceID: register.DeviceID, sessionID: sessionID, conn: conn}
					s.mu.Lock()
					previous := s.sessions[register.DeviceID]
					s.sessions[register.DeviceID] = candidate
					s.mu.Unlock()
					if previous != nil {
						_ = previous.conn.Close()
					}
					active = candidate
					lastSeen = time.Now()
					ack := registerAckSuccess{
						ReplyTo: frame.Header.MessageID, Success: true, SessionID: sessionID,
						HeartbeatInterval: int64(s.config.HeartbeatInterval / time.Second),
						ServerTime:        time.Now().Unix(), MaxControlPayload: s.config.MaxControlPayload,
						FileChunkSize: s.config.FileChunkSize,
					}
					if err := s.sendJSON(conn, &nextOutgoingID, protocol.TypeRegisterAck, protocol.FlagResponse, ack); err != nil {
						return
					}
					registered = true
					s.emit(SessionEvent{Type: EventOnline, DeviceID: active.deviceID, SessionID: active.sessionID})
					s.config.Logger.Printf("state=ONLINE device_id=%s session_id=%s", active.deviceID, active.sessionID)
					continue
				}

				switch frame.Header.Type {
				case protocol.TypeHeartbeat:
					if err := validateHeartbeat(frame.Payload); err != nil {
						_ = s.sendError(conn, &nextOutgoingID, frame.Header.MessageID, "INVALID_PAYLOAD", err.Error())
						return
					}
					lastSeen = time.Now()
					ack := heartbeatAck{ReplyTo: frame.Header.MessageID, ServerTime: time.Now().Unix()}
					if err := s.sendJSON(conn, &nextOutgoingID, protocol.TypeHeartbeatAck, protocol.FlagResponse, ack); err != nil {
						return
					}
				default:
					_ = s.sendError(conn, &nextOutgoingID, frame.Header.MessageID, "UNSUPPORTED_TYPE", fmt.Sprintf("message type 0x%02X is not supported", frame.Header.Type))
					return
				}
			}
			if decodeErr != nil {
				var frameErr *protocol.FrameError
				if errors.As(decodeErr, &frameErr) && frameErr.Header != nil && frameErr.Header.MessageID != 0 {
					_ = s.sendError(conn, &nextOutgoingID, frameErr.Header.MessageID, frameErr.Code, frameErr.Detail)
				}
				s.config.Logger.Printf("protocol_error=%v remote=%s", decodeErr, conn.RemoteAddr())
				return
			}
		}
		if readErr != nil {
			return
		}
	}
}

func (s *Server) sendError(conn net.Conn, nextMessageID *uint64, replyTo uint64, errorCode, message string) error {
	return s.sendJSON(conn, nextMessageID, protocol.TypeError, protocol.FlagResponse, errorResponse{
		ReplyTo: replyTo,
		Code:    errorCode,
		Message: message,
	})
}

func (s *Server) sendJSON(conn net.Conn, nextMessageID *uint64, messageType uint8, flags uint16, value interface{}) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	messageID := *nextMessageID
	if messageID == 0 {
		return errors.New("message_id exhausted")
	}
	frame := protocol.Frame{
		Header:  protocol.Header{Version: protocol.Version1, Type: messageType, Flags: flags, MessageID: messageID},
		Payload: payload,
	}
	_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := protocol.WriteFrame(conn, frame); err != nil {
		return err
	}
	if messageID == ^uint64(0) {
		return errors.New("message_id exhausted")
	}
	*nextMessageID = messageID + 1
	return nil
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
