package overlay

import (
	"context"
	"errors"
	"time"
)

func newOperation(n Network, m Member, action string) Operation {
	return Operation{ID: UUID(), NetworkID: n.ID, DeviceID: m.DeviceID, Action: action, Revision: n.Revision, MemberRevision: m.ConfigRevision, State: "queued", Step: "queued", TaskIDs: []string{}, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
}
func (s *Service) launch(op Operation, n Network, m Member, secret string) {
	if s.active == nil {
		s.active = map[string]bool{}
	}
	s.active[op.ID] = true
	s.wg.Add(1)
	go s.run(op, n, m, secret)
}

// Caller holds mu; a late report may not write over a newer member intent.
func (s *Service) currentOperation(op Operation) bool {
	for _, m := range s.data.Networks[op.NetworkID].Network.Members {
		if m.DeviceID == op.DeviceID {
			return m.OperationID == op.ID
		}
	}
	return false
}
func (s *Service) memberBusy(m Member) bool {
	op := s.data.Operations[m.OperationID]
	return s.active[op.ID] || op.State == "queued" || op.State == "running" || op.State == "uncertain"
}
func (s *Service) evidenceReadable(op Operation) bool {
	if s.driver.TasksSettled(op.TaskIDs) {
		return true
	}
	evidence, ok := s.driver.(interface{ TaskEvidence([]string) string })
	if !ok || evidence.TaskEvidence(op.TaskIDs) != "missing" {
		return false
	}
	// Completed bootstrap is known from the durable operation phase, even when
	// transient Task snapshots are lost on server restart. Never invent RESULTs.
	return op.Step == "controller_apply" || op.Step == "verify_runtime" || op.Step == "verify_configuration"
}
func (s *Service) confirm(ctx context.Context, n Network, m Member, secret string) error {
	checker, ok := s.controller.(interface {
		Confirm(context.Context, string, string, map[string]any) error
	})
	if !ok {
		return ErrUncertain
	}
	if e := checker.Confirm(ctx, m.MachineID, m.InstanceID, EngineConfig(n, m, secret)); e != nil {
		return ErrUncertain
	}
	return nil
}
func (s *Service) waitAnchor(ctx context.Context, n Network, m Member, report Reporter) error {
	if e := report("waiting_static_member", ""); e != nil {
		return e
	}
	deadline := time.NewTimer(45 * time.Second)
	defer deadline.Stop()
	for {
		// Refresh desired state: a cancelled/stopped anchor must not admit new DHCP nodes.
		current, e := s.Get(n.ID)
		if e != nil {
			return e
		}
		for _, a := range current.Members {
			if a.DeviceID == m.DeviceID || a.VirtualIP == "" || a.Desired != "start" {
				continue
			}
			s.mu.Lock()
			op := s.data.Operations[a.OperationID]
			s.mu.Unlock()
			if op.State != "succeeded" && op.State != "reconciled" {
				continue
			}
			r, e := s.controller.Collect(ctx, a.MachineID, a.InstanceID)
			if e == nil && r.Running && r.MyNode.IP.Address == a.VirtualIP {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return ErrConflict
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// Periodic read-only reconciliation replaces the normal need for a user to
// repeatedly click a recovery button. Recovery never replays uncertain writes.
func (s *Service) maintain() {
	for _, op := range s.Operations("") {
		if op.State != "uncertain" {
			continue
		}
		s.mu.Lock()
		active := s.active[op.ID] || !s.currentOperation(op)
		s.mu.Unlock()
		if active {
			continue
		}
		ctx, cancel := context.WithTimeout(s.ctx, 8*time.Second)
		_, _ = s.Reconcile(ctx, op.ID)
		cancel()
		if s.ctx.Err() != nil {
			return
		}
	}
	for _, n := range s.List() {
		for _, m := range n.Members {
			if m.Desired != "start" || m.AppliedRevision == 0 || !s.driver.Online(m.DeviceID) {
				continue
			}
			s.mu.Lock()
			busy := s.memberBusy(m)
			if s.recoveryAfter == nil {
				s.recoveryAfter = map[string]time.Time{}
			}
			after := s.recoveryAfter[m.DeviceID]
			s.mu.Unlock()
			if busy || time.Now().Before(after) {
				continue
			}
			ctx, cancel := context.WithTimeout(s.ctx, 8*time.Second)
			r, e := s.controller.Collect(ctx, m.MachineID, m.InstanceID)
			cancel()
			if e == nil && r.Running {
				continue
			}
			if e != nil && !errors.Is(e, ErrNotFound) && !errors.Is(e, ErrUpstream) {
				continue
			}
			s.mu.Lock()
			s.recoveryAfter[m.DeviceID] = time.Now().Add(time.Minute)
			s.mu.Unlock()
			// Start retains both UUIDs; Bootstrap inspects before installing/starting.
			// Engine absence is repaired; a running instance is never periodically reapplied.
			_, _ = s.start(n.ID, JoinRequest{DeviceID: m.DeviceID}, "start", true)
		}
	}
}

func (s *Service) waitAddress(ctx context.Context, n Network, m Member, r Running) (Running, error) {
	deadline := time.NewTimer(20 * time.Second)
	defer deadline.Stop()
	for {
		if addressMatches(n, m, r) == nil {
			return r, nil
		}
		select {
		case <-ctx.Done():
			return r, ErrUncertain
		case <-deadline.C:
			return r, ErrUncertain
		case <-time.After(250 * time.Millisecond):
		}
		next, e := s.controller.Collect(ctx, m.MachineID, m.InstanceID)
		if e == nil && next.Running {
			r = next
		}
	}
}
