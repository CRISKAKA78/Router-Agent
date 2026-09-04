package task

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func TestReconnectCorrelationAndResultIdempotency(t *testing.T) {
	s := NewService()
	spec, _ := s.NewExec("device", ExecRequest{Command: "true", Timeout: time.Second})
	result := Result{TaskID: spec.ID, Status: "success", Stdout: "original", Details: json.RawMessage(`{"a":1,"b":2}`)}
	if s.HandleResult("device", result) == nil {
		t.Fatal("undispatched result accepted")
	}
	if err := s.MarkDispatched(spec.ID, "old", 2); err != nil {
		t.Fatal(err)
	}
	ack := Ack{TaskID: spec.ID, ReplyTo: 2, Accepted: true, State: "running"}
	if s.HandleAck("device", "new", ack) == nil {
		t.Fatal("old message ID matched a new session")
	}
	if s.HandleAck("other", "old", ack) == nil {
		t.Fatal("wrong device accepted")
	}
	if s.HandleResult("other", result) == nil {
		t.Fatal("wrong device result accepted")
	}
	// ACK was lost, but the dispatched business identity remains valid.
	if err := s.HandleResult("device", result); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkDispatched(spec.ID, "new", 2); err != nil {
		t.Fatal(err)
	}
	if err := s.HandleAck("device", "new", ack); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.HandleResult("device", result); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	reordered := result
	reordered.Details = json.RawMessage(`{"b":2,"a":1}`)
	if err := s.HandleResult("device", reordered); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Result){
		func(r *Result) { r.Status = "failed" }, func(r *Result) { r.Stdout = "changed" },
		func(r *Result) { r.Details = json.RawMessage(`{"a":2,"b":2}`) },
		func(r *Result) { r.Truncated = true }, func(r *Result) { r.ExitCode = 7 },
	} {
		conflict := result
		mutate(&conflict)
		if s.HandleResult("device", conflict) == nil {
			t.Fatal("conflicting result accepted")
		}
	}
	snapshot, _ := s.Snapshot(spec.ID)
	if snapshot.State != StateSuccess || snapshot.Result.Stdout != "original" || len(snapshot.Dispatches) != 2 {
		t.Fatalf("snapshot=%+v", snapshot)
	}
	snapshot.Result.Details[0] = 'X'
	snapshot.Dispatches[1].Ack.State = "rejected"
	got, err := s.WaitResult(context.Background(), spec.ID)
	if err != nil || !sameResult(got, result) {
		t.Fatalf("result=%+v err=%v", got, err)
	}
}

func TestRepeatedAckStatesAndRejectionAreFinal(t *testing.T) {
	for _, state := range []string{"queued", "running", "success", "failed", "timeout"} {
		t.Run(state, func(t *testing.T) {
			s := NewService()
			spec, _ := s.NewExec("device", ExecRequest{Timeout: time.Second})
			_ = s.MarkDispatched(spec.ID, "one", 2)
			ack := Ack{TaskID: spec.ID, ReplyTo: 2, Accepted: true, State: state}
			if err := s.HandleAck("device", "one", ack); err != nil {
				t.Fatal(err)
			}
			if err := s.HandleAck("device", "one", ack); err != nil {
				t.Fatal(err)
			}
			_ = s.MarkDispatched(spec.ID, "two", 2)
			older := ack
			older.State = "queued"
			if err := s.HandleAck("device", "two", older); err != nil {
				t.Fatal(err)
			}
			snapshot, _ := s.Snapshot(spec.ID)
			if snapshot.State != State(state) {
				t.Fatalf("state regressed: %s", snapshot.State)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
			defer cancel()
			if _, err := s.WaitResult(ctx, spec.ID); err == nil {
				t.Fatal("ACK completed result wait")
			}
		})
	}
	s := NewService()
	spec, _ := s.NewExec("device", ExecRequest{Timeout: time.Second})
	_ = s.MarkDispatched(spec.ID, "one", 2)
	if err := s.HandleAck("device", "one", Ack{TaskID: spec.ID, ReplyTo: 2, State: "rejected"}); err != nil {
		t.Fatal(err)
	}
	if s.HandleResult("device", Result{TaskID: spec.ID, Status: "success"}) == nil {
		t.Fatal("result replaced rejection")
	}
	_ = s.MarkDispatched(spec.ID, "two", 2)
	if s.HandleAck("device", "two", Ack{TaskID: spec.ID, ReplyTo: 2, Accepted: true, State: "queued"}) == nil {
		t.Fatal("ACK replaced rejection")
	}
}
