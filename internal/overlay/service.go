package overlay

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type Reporter func(step, taskID string) error
type Driver interface {
	ValidateDevice(string) error
	Online(string) bool
	Bootstrap(context.Context, Member, Reporter) error
	TasksSettled([]string) bool
}
type storedNetwork struct {
	Network Network `json:"network"`
	Secret  string  `json:"secret"`
}
type catalog struct {
	Machines   map[string]string        `json:"machines"`
	Schema     int                      `json:"schema"`
	Networks   map[string]storedNetwork `json:"networks"`
	Operations map[string]Operation     `json:"operations"`
}
type Service struct {
	mu           sync.Mutex
	path         string
	lock         *os.File
	config       Config
	controller   Controller
	driver       Driver
	data         catalog
	observations map[string]map[string]Observation
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	closed       bool
	revision     atomic.Uint64
}

func clone[T any](v T) T { b, _ := json.Marshal(v); var out T; _ = json.Unmarshal(b, &out); return out }
func Open(file string, c Config, driver Driver, controller Controller) (*Service, error) {
	if e := c.Validate(); e != nil {
		return nil, e
	}
	if driver == nil {
		return nil, ErrInvalid
	}
	if e := os.MkdirAll(filepath.Dir(file), 0700); e != nil {
		return nil, e
	}
	lock, e := lockDirectory(file + ".lock")
	if e != nil {
		return nil, e
	}
	fail := func(e error) (*Service, error) { lock.Close(); return nil, e }
	data := catalog{Schema: 1, Networks: map[string]storedNetwork{}, Operations: map[string]Operation{}}
	if b, e := os.ReadFile(file); e == nil {
		if len(b) > 8*1024*1024 {
			return fail(ErrInvalid)
		}
		if e = json.Unmarshal(b, &data); e != nil || data.Schema != 1 || data.Networks == nil || data.Operations == nil {
			return fail(ErrInvalid)
		}
	} else if !os.IsNotExist(e) {
		return fail(e)
	}
	if data.Machines == nil {
		data.Machines = map[string]string{}
	}
	for _, n := range data.Networks {
		for _, m := range n.Network.Members {
			data.Machines[m.DeviceID] = m.MachineID
		}
	}
	if len(data.Networks) > 64 || len(data.Operations) > 2048 {
		return fail(ErrInvalid)
	}
	for id, sn := range data.Networks {
		if !ValidUUID(id) || sn.Network.ID != id || sn.Network.Spec.Validate() != nil || len(sn.Secret) != 64 || len(sn.Network.Members) > 64 {
			return fail(ErrInvalid)
		}
		for _, m := range sn.Network.Members {
			if m.DeviceID == "" || !ValidUUID(m.MachineID) || !ValidUUID(m.InstanceID) {
				return fail(ErrInvalid)
			}
		}
	}
	for id, op := range data.Operations {
		if op.State == "queued" || op.State == "running" {
			op.State = "uncertain"
			op.Error = "server_restarted_reconcile_required"
			op.UpdatedAt = time.Now().UTC()
			data.Operations[id] = op
		}
	}
	if controller == nil {
		controller, e = NewWebClient(c)
		if e != nil {
			return fail(e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{path: file, lock: lock, config: c, controller: controller, driver: driver, data: data, observations: map[string]map[string]Observation{}, ctx: ctx, cancel: cancel}
	if e = s.save(data); e != nil {
		cancel()
		return fail(e)
	}
	s.wg.Add(1)
	go s.poll()
	return s, nil
}
func (s *Service) save(c catalog) error {
	b, e := json.Marshal(c)
	if e != nil {
		return e
	}
	if len(b) > 8*1024*1024 {
		return ErrConflict
	}
	f, e := os.CreateTemp(filepath.Dir(s.path), ".networks-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(b); e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	return replaceFile(f.Name(), s.path)
}
func (s *Service) commit(c catalog) error {
	if s.closed {
		return context.Canceled
	}
	if e := s.save(c); e != nil {
		return e
	}
	s.data = c
	s.revision.Add(1)
	return nil
}
func (s *Service) Revision() uint64 { return s.revision.Load() }
func (s *Service) Status() Status {
	return Status{s.config.APIURL != "", "easytier", EngineVersion, s.config.ToolID != "", []string{"tcp://0.0.0.0:11010", "udp://0.0.0.0:11010"}, true, []string{}}
}
func (s *Service) List() []Network {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := []Network{}
	for _, n := range s.data.Networks {
		v = append(v, clone(n.Network))
	}
	sort.Slice(v, func(i, j int) bool { return v[i].CreatedAt.Before(v[j].CreatedAt) })
	return v
}
func (s *Service) Get(id string) (Network, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.data.Networks[id]
	if !ok {
		return Network{}, ErrNotFound
	}
	return clone(n.Network), nil
}
func (s *Service) Create(q Spec) (Network, error) {
	if e := q.Validate(); e != nil {
		return Network{}, e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.data.Networks) >= 64 {
		return Network{}, ErrConflict
	}
	for _, n := range s.data.Networks {
		if n.Network.Name == q.Name {
			return Network{}, ErrConflict
		}
	}
	id := UUID()
	var secret [32]byte
	if _, e := rand.Read(secret[:]); e != nil {
		return Network{}, e
	}
	n := Network{id, q, 1, time.Now().UTC(), []Member{}}
	data := clone(s.data)
	data.Networks[id] = storedNetwork{n, hex.EncodeToString(secret[:])}
	return clone(n), s.commit(data)
}
func (s *Service) busy(n Network) bool {
	for _, m := range n.Members {
		if op, ok := s.data.Operations[m.OperationID]; ok && (op.State == "queued" || op.State == "running" || op.State == "uncertain") {
			return true
		}
	}
	return false
}
func (s *Service) Update(id string, q Spec, revision uint64) (Network, error) {
	if e := q.Validate(); e != nil {
		return Network{}, e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.data.Networks[id]
	if !ok {
		return Network{}, ErrNotFound
	}
	if revision != old.Network.Revision || s.busy(old.Network) {
		return Network{}, ErrConflict
	}
	for other, n := range s.data.Networks {
		if other != id && n.Network.Name == q.Name {
			return Network{}, ErrConflict
		}
	}
	prefix, _ := netip.ParsePrefix(q.CIDR)
	for _, m := range old.Network.Members {
		if m.VirtualIP != "" {
			ip, _ := netip.ParseAddr(m.VirtualIP)
			if !prefix.Contains(ip) {
				return Network{}, ErrConflict
			}
		}
	}
	old.Network.Spec = q
	old.Network.Revision++
	data := clone(s.data)
	data.Networks[id] = old
	return clone(old.Network), s.commit(data)
}
func (s *Service) Start(id string, q JoinRequest, action string) (Operation, error) {
	if action != "start" && action != "stop" {
		return Operation{}, ErrInvalid
	}
	if s.config.APIURL == "" {
		return Operation{}, ErrDisabled
	}
	if q.DeviceID == "" || len(q.DeviceID) > 128 {
		return Operation{}, ErrInvalid
	}
	if action == "start" {
		if e := s.driver.ValidateDevice(q.DeviceID); e != nil {
			return Operation{}, e
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sn, ok := s.data.Networks[id]
	if !ok {
		return Operation{}, ErrNotFound
	}
	if len(s.data.Operations) >= 2048 {
		return Operation{}, ErrConflict
	}
	index := -1
	for i, m := range sn.Network.Members {
		if m.DeviceID == q.DeviceID {
			index = i
		}
	}
	if index < 0 && action == "stop" {
		return Operation{}, ErrNotFound
	}
	if index >= 0 {
		op := s.data.Operations[sn.Network.Members[index].OperationID]
		if op.State == "queued" || op.State == "running" || op.State == "uncertain" {
			return Operation{}, ErrConflict
		}
	}
	if index < 0 {
		if len(sn.Network.Members) >= 64 {
			return Operation{}, ErrConflict
		}
		for _, other := range s.data.Networks {
			for _, m := range other.Network.Members {
				if m.DeviceID == q.DeviceID {
					return Operation{}, ErrConflict
				}
			}
		}
		if q.VirtualIP != "" {
			p, _ := netip.ParsePrefix(sn.Network.CIDR)
			ip, e := netip.ParseAddr(q.VirtualIP)
			if e != nil || !ip.Is4() || !p.Contains(ip) || ip == p.Addr() {
				return Operation{}, ErrInvalid
			}
			a := ip.As4()
			bits := p.Bits()
			host := uint32(a[0])<<24 | uint32(a[1])<<16 | uint32(a[2])<<8 | uint32(a[3])
			if host&((1<<uint(32-bits))-1) == ((1 << uint(32-bits)) - 1) {
				return Operation{}, ErrInvalid
			}
			for _, m := range sn.Network.Members {
				if m.VirtualIP == q.VirtualIP {
					return Operation{}, ErrConflict
				}
			}
		}
		machineID := s.data.Machines[q.DeviceID]
		if machineID == "" {
			machineID = UUID()
		}
		sn.Network.Members = append(sn.Network.Members, Member{DeviceID: q.DeviceID, MachineID: machineID, InstanceID: UUID(), VirtualIP: q.VirtualIP})
		index = len(sn.Network.Members) - 1
	} else if q.VirtualIP != "" && q.VirtualIP != sn.Network.Members[index].VirtualIP {
		return Operation{}, ErrConflict
	}
	op := Operation{ID: UUID(), NetworkID: id, DeviceID: q.DeviceID, Action: action, Revision: sn.Network.Revision, State: "queued", Step: "queued", TaskIDs: []string{}, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	m := &sn.Network.Members[index]
	m.OperationID = op.ID
	m.Desired = action
	data := clone(s.data)
	data.Networks[id] = sn
	data.Operations[op.ID] = op
	data.Machines[q.DeviceID] = m.MachineID
	if e := s.commit(data); e != nil {
		return Operation{}, e
	}
	s.wg.Add(1)
	go s.run(op, clone(sn.Network), *m, sn.Secret)
	return op, nil
}
func (s *Service) report(id, step, taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data := clone(s.data)
	op, ok := data.Operations[id]
	if !ok {
		return ErrNotFound
	}
	op.State = "running"
	op.Step = step
	op.UpdatedAt = time.Now().UTC()
	if taskID != "" {
		op.TaskIDs = append(op.TaskIDs, taskID)
	}
	data.Operations[id] = op
	return s.commit(data)
}
func (s *Service) finish(id string, e error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data := clone(s.data)
	op := data.Operations[id]
	op.State = "succeeded"
	if e == nil {
		op.Step = "complete"
	}
	op.Error = ""
	op.UpdatedAt = time.Now().UTC()
	if e != nil {
		op.State = "failed"
		op.Error = "operation_failed"
		if uncertain(e) {
			op.State = "uncertain"
			op.Error = "reconcile_required"
		}
		if errors.Is(e, ErrUpstream) {
			op.Error = "controller_unavailable"
		}
		if errors.Is(e, ErrDisabled) {
			op.Error = "package_or_controller_not_configured"
		}
	}
	if e == nil {
		n := data.Networks[op.NetworkID]
		for i := range n.Network.Members {
			m := &n.Network.Members[i]
			if m.DeviceID == op.DeviceID && m.OperationID == op.ID {
				if op.Action == "start" {
					m.AppliedRevision = op.Revision
				}
			}
		}
		data.Networks[op.NetworkID] = n
	}
	data.Operations[id] = op
	if x := s.commit(data); x != nil {
		op.State = "uncertain"
		op.Error = "persistence_failed"
		s.data.Operations[id] = op
		s.revision.Add(1)
	}
}
func (s *Service) run(op Operation, n Network, m Member, secret string) {
	defer s.wg.Done()
	ctx, cancel := context.WithTimeout(s.ctx, 3*time.Minute)
	defer cancel()
	report := func(step, id string) error { return s.report(op.ID, step, id) }
	e := report("preparing", "")
	if e == nil && op.Action == "start" {
		e = s.driver.Bootstrap(ctx, m, report)
	}
	if e == nil && op.Action == "start" {
		if ready, ok := s.controller.(interface {
			Ready(context.Context, string) error
		}); ok {
			deadline := time.Now().Add(20 * time.Second)
			for {
				e = ready.Ready(ctx, m.MachineID)
				if e == nil || time.Now().After(deadline) {
					break
				}
				select {
				case <-ctx.Done():
					e = ctx.Err()
				case <-time.After(500 * time.Millisecond):
				}
				if ctx.Err() != nil {
					break
				}
			}
		}
	}
	if e == nil {
		e = report("controller_apply", "")
	}
	if e == nil {
		if op.Action == "start" {
			e = s.controller.Apply(ctx, m.MachineID, m.InstanceID, EngineConfig(n, m, secret))
		} else {
			e = s.controller.Stop(ctx, m.MachineID, m.InstanceID)
		}
	}
	if e == nil {
		e = report("verify_runtime", "")
		if e == nil {
			var r Running
			r, e = s.controller.Collect(ctx, m.MachineID, m.InstanceID)
			if op.Action == "stop" && errors.Is(e, ErrNotFound) {
				e = nil
			} else if e == nil && (r.Running != (op.Action == "start") || (r.ErrorMessage != nil && *r.ErrorMessage != "")) {
				e = ErrUncertain
			}
			if e == nil {
				s.setObservation(n.ID, m.DeviceID, r, nil)
			} else {
				e = ErrUncertain
			}
		}
	}
	s.finish(op.ID, e)
}
func (s *Service) Operations(id string) []Operation {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := []Operation{}
	for _, op := range s.data.Operations {
		if id == "" || op.NetworkID == id {
			v = append(v, clone(op))
		}
	}
	sort.Slice(v, func(i, j int) bool { return v[i].CreatedAt.After(v[j].CreatedAt) })
	return v
}
func (s *Service) Topology(id string) (Topology, error) {
	n, e := s.Get(id)
	if e != nil {
		return Topology{}, e
	}
	s.mu.Lock()
	obs := clone(s.observations[id])
	s.mu.Unlock()
	return topology(n, obs, s.driver.Online, time.Now().UTC()), nil
}
func (s *Service) setObservation(network, device string, r Running, e error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.observations[network] == nil {
		s.observations[network] = map[string]Observation{}
	}
	if e == nil {
		s.observations[network][device] = observe(device, r, time.Now().UTC())
	} else {
		o, ok := s.observations[network][device]
		if !ok {
			o = Observation{DeviceID: device, State: "unknown", Links: []Link{}, Routes: []Route{}}
		}
		o.Stale = true
		o.Error = "controller_observation_unavailable"
		s.observations[network][device] = o
	}
	s.revision.Add(1)
}
func (s *Service) Refresh(ctx context.Context) {
	if s.config.APIURL == "" {
		return
	}
	type item struct {
		network string
		member  Member
	}
	jobs := make(chan item)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				r, e := s.controller.Collect(ctx, j.member.MachineID, j.member.InstanceID)
				if errors.Is(e, ErrNotFound) {
					e = nil
				}
				s.setObservation(j.network, j.member.DeviceID, r, e)
			}
		}()
	}
loop:
	for _, n := range s.List() {
		for _, m := range n.Members {
			select {
			case jobs <- item{n.ID, m}:
			case <-ctx.Done():
				break loop
			}
		}
	}
	close(jobs)
	wg.Wait()
}
func (s *Service) poll() {
	defer s.wg.Done()
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-t.C:
			ctx, cancel := context.WithTimeout(s.ctx, 8*time.Second)
			s.Refresh(ctx)
			cancel()
		}
	}
}

// Reconcile is read-only against the engine. It never retries a mutating call.
// An observed state is reported, not treated as proof that all old steps ran.
func (s *Service) Reconcile(ctx context.Context, id string) (Operation, error) {
	s.mu.Lock()
	op, ok := s.data.Operations[id]
	if !ok {
		s.mu.Unlock()
		return Operation{}, ErrNotFound
	}
	if op.State != "uncertain" {
		s.mu.Unlock()
		return Operation{}, ErrConflict
	}
	sn := s.data.Networks[op.NetworkID]
	var m Member
	for _, x := range sn.Network.Members {
		if x.DeviceID == op.DeviceID {
			m = x
		}
	}
	s.mu.Unlock()
	if !s.driver.TasksSettled(op.TaskIDs) {
		return Operation{}, ErrUncertain
	}
	r, e := s.controller.Collect(ctx, m.MachineID, m.InstanceID)
	if op.Action == "stop" && errors.Is(e, ErrNotFound) {
		e = nil
	}
	if e != nil {
		return Operation{}, e
	}
	s.setObservation(op.NetworkID, m.DeviceID, r, nil)
	if r.Running != (op.Action == "start") || r.ErrorMessage != nil && *r.ErrorMessage != "" {
		return op, ErrUncertain
	}
	// The original revision must still be present upstream. Runtime alone does
	// not prove that a timed-out save/enable pair applied the requested config.
	if op.Action == "start" {
		checker, ok := s.controller.(interface {
			Confirm(context.Context, string, string, map[string]any) error
		})
		if !ok {
			return op, ErrUncertain
		}
		if e := checker.Confirm(ctx, m.MachineID, m.InstanceID, EngineConfig(sn.Network, m, sn.Secret)); e != nil {
			return op, e
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data := clone(s.data)
	current := data.Operations[id]
	if current.State != "uncertain" {
		return Operation{}, ErrConflict
	}
	current.State = "reconciled"
	current.Step = "runtime_observed"
	current.Error = ""
	if r.Running {
		current.Step = "observed_running_configuration_confirmed"
	} else {
		current.Step = "observed_stopped"
	}
	current.UpdatedAt = time.Now().UTC()
	data.Operations[id] = current
	return current, s.commit(data)
}
func (s *Service) RemoveMember(id, device string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sn, ok := s.data.Networks[id]
	if !ok {
		return ErrNotFound
	}
	if s.busy(sn.Network) {
		return ErrConflict
	}
	index := -1
	for i, m := range sn.Network.Members {
		if m.DeviceID == device {
			index = i
			if m.Desired != "stop" {
				return ErrConflict
			}
			op := s.data.Operations[m.OperationID]
			if op.State != "succeeded" || op.Action != "stop" {
				return ErrConflict
			}
		}
	}
	if index < 0 {
		return ErrNotFound
	}
	sn.Network.Members = append(sn.Network.Members[:index], sn.Network.Members[index+1:]...)
	data := clone(s.data)
	data.Networks[id] = sn
	return s.commit(data)
}
func (s *Service) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sn, ok := s.data.Networks[id]
	if !ok {
		return ErrNotFound
	}
	if len(sn.Network.Members) > 0 {
		return ErrConflict
	}
	data := clone(s.data)
	delete(data.Networks, id)
	return s.commit(data)
}
func (s *Service) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.cancel()
	s.mu.Unlock()
	s.wg.Wait()
	if w, ok := s.controller.(*WebClient); ok {
		w.Close()
	}
	return s.lock.Close()
}
