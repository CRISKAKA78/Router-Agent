package task

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
)

type State string

const (
	StateReceived State = "received"
	StateQueued   State = "queued"
	StateRunning  State = "running"
	StateSuccess  State = "success"
	StateFailed   State = "failed"
	StateTimeout  State = "timeout"
	StateRejected State = "rejected"
)

var (
	ErrTaskNotFound = errors.New("task not found")
	ErrTaskRejected = errors.New("task rejected")
)

type ExecRequest struct {
	Command string
	Cwd     string
	Env     map[string]string
	Timeout time.Duration
}

type Spec struct {
	Params    json.RawMessage
	ID        string
	DeviceID  string
	Type      string
	CreatedAt int64
	Timeout   uint32
	Command   string
	Cwd       string
	Env       map[string]string
}

type Ack struct {
	ReplyTo  uint64 `json:"reply_to"`
	TaskID   string `json:"task_id"`
	Accepted bool   `json:"accepted"`
	State    string `json:"state"`
}

type Result struct {
	Details    json.RawMessage `json:"result"`
	TaskID     string          `json:"task_id"`
	Status     string          `json:"status"`
	StartedAt  int64           `json:"started_at"`
	FinishedAt int64           `json:"finished_at"`
	ExitCode   int             `json:"exit_code"`
	Stdout     string          `json:"stdout"`
	Stderr     string          `json:"stderr"`
	Truncated  bool            `json:"truncated"`
}

type Dispatch struct {
	SessionID string
	MessageID uint64
	Ack       *Ack
}

type Snapshot struct {
	Dispatches []Dispatch
	Spec       Spec
	State      State
	MessageID  uint64
	Ack        *Ack
	Result     *Result
}

type record struct {
	snapshot Snapshot
	done     chan struct{}
	closed   bool
}

type Service struct {
	mu      sync.Mutex
	records map[string]*record
}

func NewService() *Service {
	return &Service{records: make(map[string]*record)}
}

func newTaskID() (string, error) {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	data[6] = (data[6] & 0x0f) | 0x40
	data[8] = (data[8] & 0x3f) | 0x80
	encoded := make([]byte, 32)
	hex.Encode(encoded, data)
	return fmt.Sprintf("%s-%s-%s-%s-%s", encoded[0:8], encoded[8:12], encoded[12:16], encoded[16:20], encoded[20:32]), nil
}

func (s *Service) NewExec(deviceID string, request ExecRequest) (Spec, error) {
	if deviceID == "" {
		return Spec{}, errors.New("device_id is required")
	}
	if request.Timeout <= 0 || request.Timeout%time.Second != 0 || request.Timeout/time.Second > time.Duration(^uint32(0)) {
		return Spec{}, errors.New("timeout must be a positive whole number of seconds within uint32 range")
	}
	if strings.IndexByte(request.Command, 0) >= 0 || strings.IndexByte(request.Cwd, 0) >= 0 {
		return Spec{}, errors.New("command and cwd must not contain NUL")
	}
	taskID, err := newTaskID()
	if err != nil {
		return Spec{}, fmt.Errorf("generate task_id: %w", err)
	}
	environment := make(map[string]string, len(request.Env))
	for name, value := range request.Env {
		if name == "" || strings.Contains(name, "=") || strings.IndexByte(name, 0) >= 0 || strings.IndexByte(value, 0) >= 0 {
			return Spec{}, errors.New("env names must be non-empty without '=' or NUL, and values must not contain NUL")
		}
		environment[name] = value
	}
	spec := Spec{
		ID: taskID, DeviceID: deviceID, Type: "exec", CreatedAt: time.Now().Unix(),
		Timeout: uint32(request.Timeout / time.Second), Command: request.Command,
		Cwd: request.Cwd, Env: environment,
	}
	s.mu.Lock()
	s.records[taskID] = &record{
		snapshot: Snapshot{Spec: spec, State: StateReceived},
		done:     make(chan struct{}),
	}
	s.mu.Unlock()
	return spec, nil
}

// NewFile records immutable wire parameters; filesystem ownership stays in File Service.
func (s *Service) NewFile(deviceID, kind string, timeout time.Duration, params json.RawMessage) (Spec, error) {
	if deviceID == "" || (kind != "upload" && kind != "download") || timeout <= 0 || timeout%time.Second != 0 || timeout/time.Second > time.Duration(^uint32(0)) {
		return Spec{}, errors.New("invalid file task")
	}
	id, err := newTaskID()
	if err != nil {
		return Spec{}, err
	}
	spec := Spec{ID: id, DeviceID: deviceID, Type: kind, CreatedAt: time.Now().Unix(), Timeout: uint32(timeout / time.Second), Params: append(json.RawMessage(nil), params...)}
	s.mu.Lock()
	s.records[id] = &record{snapshot: Snapshot{Spec: spec, State: StateReceived}, done: make(chan struct{})}
	s.mu.Unlock()
	return spec, nil
}

func (s *Service) MarkDispatched(taskID, sessionID string, messageID uint64) error {
	if messageID == 0 || sessionID == "" {
		return errors.New("message_id must not be zero")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[taskID]
	if !ok {
		return ErrTaskNotFound
	}
	for _, dispatch := range record.snapshot.Dispatches {
		if dispatch.SessionID == sessionID && dispatch.MessageID == messageID {
			return errors.New("duplicate dispatch correlation")
		}
	}
	record.snapshot.Dispatches = append(record.snapshot.Dispatches, Dispatch{SessionID: sessionID, MessageID: messageID})
	record.snapshot.MessageID = messageID
	return nil
}

func (s *Service) Remove(taskID string) {
	s.mu.Lock()
	delete(s.records, taskID)
	s.mu.Unlock()
}

func (s *Service) HandleAck(deviceID, sessionID string, ack Ack) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[ack.TaskID]
	if !ok {
		return ErrTaskNotFound
	}
	snapshot := &record.snapshot
	if snapshot.Spec.DeviceID != deviceID {
		return errors.New("TASK_ACK device does not match task device")
	}
	var attempt *Dispatch
	for i := range snapshot.Dispatches {
		candidate := &snapshot.Dispatches[i]
		if candidate.SessionID == sessionID && candidate.MessageID == ack.ReplyTo {
			attempt = candidate
			break
		}
	}
	if attempt == nil {
		return errors.New("TASK_ACK does not match session/message/task dispatch")
	}
	if attempt.Ack != nil {
		if *attempt.Ack == ack {
			return nil
		}
		return errors.New("conflicting TASK_ACK for dispatch")
	}
	incoming := State(ack.State)
	if ack.Accepted {
		if incoming != StateQueued && incoming != StateRunning && !terminal(incoming) {
			return errors.New("invalid accepted TASK_ACK state")
		}
		if snapshot.State == StateRejected {
			return errors.New("accepted ACK after rejection")
		}
		if terminal(snapshot.State) && terminal(incoming) && snapshot.State != incoming {
			return errors.New("conflicting task terminal state")
		}
	} else {
		if incoming != StateRejected {
			return errors.New("invalid rejected TASK_ACK state")
		}
		if snapshot.Ack != nil && snapshot.Ack.Accepted || snapshot.Result != nil {
			return errors.New("rejection cannot replace accepted task")
		}
	}
	copy := ack
	attempt.Ack = &copy
	snapshot.Ack = &copy
	if !ack.Accepted {
		snapshot.State = StateRejected
		s.closeLocked(record)
	} else if !terminal(snapshot.State) && (incoming != StateQueued || snapshot.State != StateRunning) {
		snapshot.State = incoming
	}
	return nil
}

func terminal(state State) bool {
	return state == StateSuccess || state == StateFailed || state == StateTimeout
}

// JSON object key order is not business identity. UseNumber preserves integer
// precision rather than rounding through float64. Unknown top-level wire fields
// never enter Result, while the defined result object participates in equality.
func resultDetails(raw json.RawMessage) interface{} {
	if len(raw) == 0 {
		raw = json.RawMessage("{}")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value interface{}
	if decoder.Decode(&value) != nil {
		return string(raw)
	}
	return value
}

func sameResult(a, b Result) bool {
	ad, bd := resultDetails(a.Details), resultDetails(b.Details)
	a.Details, b.Details = nil, nil
	return reflect.DeepEqual(a, b) && reflect.DeepEqual(ad, bd)
}

func (s *Service) HandleResult(deviceID string, result Result) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[result.TaskID]
	if !ok {
		return ErrTaskNotFound
	}
	if record.snapshot.Spec.DeviceID != deviceID {
		return errors.New("TASK_RESULT device does not match task device")
	}
	if len(record.snapshot.Dispatches) == 0 || record.snapshot.State == StateRejected {
		return errors.New("TASK_RESULT for undispatched or rejected task")
	}
	if record.snapshot.Result != nil {
		if sameResult(*record.snapshot.Result, result) {
			return nil
		}
		return errors.New("conflicting TASK_RESULT")
	}
	var state State
	switch result.Status {
	case "success":
		state = StateSuccess
	case "failed":
		state = StateFailed
	case "timeout":
		state = StateTimeout
	default:
		return errors.New("TASK_RESULT status is invalid")
	}
	if terminal(record.snapshot.State) && record.snapshot.State != state {
		return errors.New("TASK_RESULT conflicts with terminal ACK")
	}
	copy := result
	copy.Details = append(json.RawMessage(nil), result.Details...)
	record.snapshot.Result = &copy
	record.snapshot.State = state
	s.closeLocked(record)
	return nil
}

func (s *Service) closeLocked(record *record) {
	if !record.closed {
		close(record.done)
		record.closed = true
	}
}

func (s *Service) Snapshot(taskID string) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[taskID]
	if !ok {
		return Snapshot{}, ErrTaskNotFound
	}
	return cloneSnapshot(record.snapshot), nil
}

func (s *Service) WaitResult(ctx context.Context, taskID string) (Result, error) {
	s.mu.Lock()
	record, ok := s.records[taskID]
	if !ok {
		s.mu.Unlock()
		return Result{}, ErrTaskNotFound
	}
	done := record.done
	s.mu.Unlock()

	select {
	case <-ctx.Done():
		return Result{}, ctx.Err()
	case <-done:
	}

	snapshot, err := s.Snapshot(taskID)
	if err != nil {
		return Result{}, err
	}
	if snapshot.State == StateRejected {
		return Result{}, ErrTaskRejected
	}
	if snapshot.Result == nil {
		return Result{}, errors.New("task completed without TASK_RESULT")
	}
	return *snapshot.Result, nil
}

func cloneSnapshot(input Snapshot) Snapshot {
	output := input
	output.Spec.Params = append(json.RawMessage(nil), input.Spec.Params...)
	output.Dispatches = append([]Dispatch(nil), input.Dispatches...)
	for i := range output.Dispatches {
		if output.Dispatches[i].Ack != nil {
			copy := *output.Dispatches[i].Ack
			output.Dispatches[i].Ack = &copy
		}
	}
	output.Spec.Env = make(map[string]string, len(input.Spec.Env))
	for name, value := range input.Spec.Env {
		output.Spec.Env[name] = value
	}
	if input.Ack != nil {
		copy := *input.Ack
		output.Ack = &copy
	}
	if input.Result != nil {
		copy := *input.Result
		copy.Details = append(json.RawMessage(nil), input.Result.Details...)
		output.Result = &copy
	}
	return output
}
