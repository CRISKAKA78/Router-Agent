package task

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestServiceAcceptedTaskLifecycle(t *testing.T) {
	service := NewService()
	spec, err := service.NewExec("device-1", ExecRequest{Command: "echo hello", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.MarkDispatched(spec.ID, "session-1", 2); err != nil {
		t.Fatal(err)
	}
	if err := service.HandleAck("device-1", "session-1", Ack{ReplyTo: 2, TaskID: spec.ID, Accepted: true, State: "queued"}); err != nil {
		t.Fatal(err)
	}
	result := Result{
		TaskID: spec.ID, Status: "success", StartedAt: 1, FinishedAt: 2,
		ExitCode: 0, Stdout: "hello\n",
	}
	if err := service.HandleResult("device-1", result); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	completed, err := service.WaitResult(ctx, spec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != "success" || completed.Stdout != "hello\n" {
		t.Fatalf("result = %#v", completed)
	}
	snapshot, err := service.Snapshot(spec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.State != StateSuccess || snapshot.MessageID != 2 || snapshot.Ack == nil || snapshot.Result == nil {
		t.Fatalf("snapshot = %#v", snapshot)
	}
}

func TestServiceRejectsMismatchedAckAndCompletesRejection(t *testing.T) {
	service := NewService()
	spec, err := service.NewExec("device-1", ExecRequest{Command: "true", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.MarkDispatched(spec.ID, "session-1", 7); err != nil {
		t.Fatal(err)
	}
	if err := service.HandleAck("device-1", "session-1", Ack{ReplyTo: 8, TaskID: spec.ID, Accepted: true, State: "queued"}); err == nil {
		t.Fatal("mismatched reply_to was accepted")
	}
	if err := service.HandleAck("device-1", "session-1", Ack{ReplyTo: 7, TaskID: spec.ID, Accepted: false, State: "rejected"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := service.WaitResult(ctx, spec.ID); !errors.Is(err, ErrTaskRejected) {
		t.Fatalf("WaitResult error = %v", err)
	}
}
