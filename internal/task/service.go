package task

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type State string

const (
	StateReceived State = "received"
	StateQueued   State = "queued"
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
	TaskID     string `json:"task_id"`
	Status     string `json:"status"`
	StartedAt  int64  `json:"started_at"`
	FinishedAt int64  `json:"finished_at"`
	ExitCode   int    `json:"exit_code"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	Truncated  bool   `json:"truncated"`
}

type Snapshot struct {
	Spec      Spec
	State     State
	MessageID uint64
	Ack       *Ack
	Result    *Result
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

func (s *Service) MarkDispatched(taskID string, messageID uint64) error {
	if messageID == 0 {
		return errors.New("message_id must not be zero")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[taskID]
	if !ok {
		return ErrTaskNotFound
	}
	if record.snapshot.MessageID != 0 {
		return errors.New("task was already dispatched")
	}
	record.snapshot.MessageID = messageID
	return nil
}

func (s *Service) Remove(taskID string) {
	s.mu.Lock()
	delete(s.records, taskID)
	s.mu.Unlock()
}

func (s *Service) HandleAck(deviceID string, ack Ack) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[ack.TaskID]
	if !ok {
		return ErrTaskNotFound
	}
	if record.snapshot.Spec.DeviceID != deviceID {
		return errors.New("TASK_ACK device does not match task device")
	}
	if record.snapshot.MessageID == 0 || ack.ReplyTo != record.snapshot.MessageID {
		return errors.New("TASK_ACK reply_to does not match TASK message_id")
	}
	if record.snapshot.Ack != nil {
		return errors.New("duplicate TASK_ACK")
	}
	if ack.Accepted && ack.State != "queued" {
		return errors.New("accepted TASK_ACK state must be queued")
	}
	if !ack.Accepted && ack.State != "rejected" {
		return errors.New("rejected TASK_ACK state must be rejected")
	}
	copy := ack
	record.snapshot.Ack = &copy
	if ack.Accepted {
		record.snapshot.State = StateQueued
		return nil
	}
	record.snapshot.State = StateRejected
	s.closeLocked(record)
	return nil
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
	if record.snapshot.Ack == nil || !record.snapshot.Ack.Accepted {
		return errors.New("TASK_RESULT received before accepted TASK_ACK")
	}
	if record.snapshot.Result != nil {
		return errors.New("duplicate TASK_RESULT")
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
	copy := result
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
		output.Result = &copy
	}
	return output
}
