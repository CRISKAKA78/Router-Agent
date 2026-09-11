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
	mu            sync.Mutex
	path          string
	lock          *os.File
	config        Config
	controller    Controller
	driver        Driver
	data          catalog
	observations  map[string]map[string]Observation
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	closed        bool
	revision      atomic.Uint64
	active        map[string]bool
	recoveryAfter map[string]time.Time
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
		if !ValidUUID(id) || sn.Network.ID != id || sn.Network.Spec.Validate() != nil || (len(sn.Secret) < 1 || len(sn.Secret) > 128) || len(sn.Network.Members) > 64 {
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
func (s *Service) Create(q Spec) (Network, error) { return s.create(q, "", 0) }
func (s *Service) create(q Spec, password string, profile int) (Network, error) {
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
	n := Network{ID: id, Spec: q, Profile: profile, Revision: 1, CreatedAt: time.Now().UTC(), Members: []Member{}}
	if password == "" {
		password = hex.EncodeToString(secret[:])
	}
	data := clone(s.data)
	data.Networks[id] = storedNetwork{n, password}
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
	if s.closed || revision != old.Network.Revision || s.busy(old.Network) {
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
	if old.Network.Profile >= 2 {
		if len(q.Routes) > 0 {
			return Network{}, ErrInvalid
		}
		if len(q.PeerURLs) == 0 {
			q.PeerURLs = []string{"tcp://47.119.168.150:11010", "udp://47.119.168.150:11010"}
		}
		if q.MTU == 0 {
			q.MTU = 1380
		}
	}
	old.Network.Spec = q
	old.Network.Revision++
	data := clone(s.data)
	data.Networks[id] = old
	return clone(old.Network), s.commit(data)
}
func (s *Service) Start(id string, q JoinRequest, action string) (Operation, error) {
	return s.start(id, q, action, false)
}
func (s *Service) start(id string, q JoinRequest, action string, recovery bool) (Operation, error) {
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
	sn = clone(sn)
	if !ok {
		return Operation{}, ErrNotFound
	}
	if s.closed || len(s.data.Operations) >= 2048 {
		return Operation{}, ErrConflict
	}
	index := -1
	for i, m := range sn.Network.Members {
		if m.DeviceID == q.DeviceID {
			index = i
		}
	}
	if recovery && (index < 0 || sn.Network.Members[index].Desired != "start" || sn.Network.Members[index].AppliedRevision == 0) {
		return Operation{}, ErrConflict
	}
	if index < 0 && action == "stop" {
		return Operation{}, ErrNotFound
	}
	if index >= 0 {
		op := s.data.Operations[sn.Network.Members[index].OperationID]
		if op.State == "queued" || op.State == "running" || s.active[op.ID] {
			return Operation{}, ErrConflict
		}
		if op.State == "uncertain" && (action != "stop" || !s.evidenceReadable(op)) {
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
			machineID = MachineID(q.DeviceID)
		}
		if sn.Network.Profile >= 2 && q.VirtualIP == "" && !hasAnchor(sn.Network, "") {
			return Operation{}, ErrConflict
		}
		member := Member{DeviceID: q.DeviceID, MachineID: machineID, InstanceID: UUID(), VirtualIP: q.VirtualIP}
		if sn.Network.Profile >= 2 {
			cfg := DefaultMemberConfig(q.DeviceID)
			if names, ok := s.driver.(interface{ DeviceName(string) string }); ok {
				cfg.Hostname = names.DeviceName(q.DeviceID)
			}
			cfg.VirtualIP = q.VirtualIP
			member.Config = &cfg
			member.ConfigRevision = 1
		}
		sn.Network.Members = append(sn.Network.Members, member)
		index = len(sn.Network.Members) - 1
	} else if q.VirtualIP != "" && q.VirtualIP != sn.Network.Members[index].VirtualIP {
		return Operation{}, ErrConflict
	}
	op := newOperation(sn.Network, sn.Network.Members[index], action)
	op.Recovery = recovery
	m := &sn.Network.Members[index]
	previousOperation := m.OperationID
	m.OperationID = op.ID
	m.Desired = action
	data := clone(s.data)
	data.Networks[id] = sn
	data.Operations[op.ID] = op
	if old, ok := data.Operations[previousOperation]; ok && old.State == "uncertain" {
		old.SupersededBy = op.ID
		data.Operations[old.ID] = old
	}
	data.Machines[q.DeviceID] = m.MachineID
	if e := s.commit(data); e != nil {
		return Operation{}, e
	}
	s.launch(op, clone(sn.Network), *m, sn.Secret)
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
	if !s.currentOperation(op) {
		return ErrConflict
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
	delete(s.active, id)
	defer s.mu.Unlock()
	data := clone(s.data)
	op := data.Operations[id]
	if !s.currentOperation(op) {
		return
	}
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
					m.AppliedConfigRevision = op.MemberRevision
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
	defer func() { s.mu.Lock(); delete(s.active, op.ID); s.mu.Unlock() }()
	ctx, cancel := context.WithTimeout(s.ctx, 3*time.Minute)
	defer cancel()
	report := func(step, id string) error { return s.report(op.ID, step, id) }
	e := report("preparing", "")
	if e == nil && op.Action == "start" && n.Profile >= 2 && m.VirtualIP == "" {
		e = s.waitAnchor(ctx, n, m, report)
	}
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

	if e == nil && op.Recovery {
		r, x := s.controller.Collect(ctx, m.MachineID, m.InstanceID)
		if x == nil && r.Running {
			e = s.confirm(ctx, n, m, secret)
			if e == nil && n.Profile >= 2 {
				r, e = s.waitAddress(ctx, n, m, r)
			}
			s.setObservation(n.ID, m.DeviceID, r, nil)
			s.finish(op.ID, e)
			return
		}
		if x != nil && !errors.Is(x, ErrNotFound) {
			e = x
		}
	}
	if e == nil {
		e = report("controller_apply", "")
	}
	if e == nil {
		if op.Action == "start" {
			// Saving config is not hot reload. Stop only this instance before reapplying.
			if m.AppliedRevision > 0 && !op.Recovery {
				e = s.controller.Stop(ctx, m.MachineID, m.InstanceID)
				if e == nil {
					_, e = s.waitRuntime(ctx, m.MachineID, m.InstanceID, false)
				}
			}
			if e == nil {
				e = s.controller.Apply(ctx, m.MachineID, m.InstanceID, EngineConfig(n, m, secret))
			}
		} else {
			e = s.controller.Stop(ctx, m.MachineID, m.InstanceID)
		}
	}
	if e == nil {
		e = report("verify_runtime", "")
		if e == nil {
			var r Running
			r, e = s.waitRuntime(ctx, m.MachineID, m.InstanceID, op.Action == "start")
			if e == nil {
				if op.Action == "start" && n.Profile >= 2 {
					r, e = s.waitAddress(ctx, n, m, r)
				}
				s.setObservation(n.ID, m.DeviceID, r, nil)
			} else {
				e = ErrUncertain
			}
		}
	}
	if e == nil && op.Action == "start" {
		e = report("verify_configuration", "")
		if e == nil {
			e = s.confirm(ctx, n, m, secret)
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
			s.maintain()
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
	if m.OperationID != op.ID || !s.evidenceReadable(op) {
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
	if op.Action == "start" && sn.Network.Profile >= 2 {
		if e := addressMatches(sn.Network, m, r); e != nil {
			return op, e
		}
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
	if current.State != "uncertain" || !s.currentOperation(current) || s.active[current.ID] {
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
	if current.Action == "start" {
		n := data.Networks[current.NetworkID]
		for i := range n.Network.Members {
			member := &n.Network.Members[i]
			if member.DeviceID == current.DeviceID && member.OperationID == current.ID {
				member.AppliedRevision = current.Revision
				member.AppliedConfigRevision = current.MemberRevision
			}
		}
		data.Networks[current.NetworkID] = n
	}
	return current, s.commit(data)
}
func (s *Service) RemoveMember(id, device string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sn, ok := s.data.Networks[id]
	sn = clone(sn)
	if !ok {
		return ErrNotFound
	}
	if s.closed {
		return ErrConflict
	}
	index := -1
	for i, m := range sn.Network.Members {
		if m.DeviceID == device {
			index = i
			if s.memberBusy(m) {
				return ErrConflict
			}
			if m.VirtualIP != "" && !hasAnchor(sn.Network, device) && hasDHCP(sn.Network, device) {
				return ErrConflict
			}
			if m.Desired != "stop" {
				return ErrConflict
			}
			op := s.data.Operations[m.OperationID]
			if (op.State != "succeeded" && op.State != "reconciled") || op.Action != "stop" {
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

// Enabling an EasyTier instance is asynchronous. Poll only observations, never
// repeat the save/enable mutation because the first runtime snapshot is empty.
func (s *Service) waitRuntime(parent context.Context, machine, instance string, running bool) (Running, error) {
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	for {
		if ctx.Err() != nil {
			return Running{}, ErrUncertain
		}
		r, err := s.controller.Collect(ctx, machine, instance)
		if !running && errors.Is(err, ErrNotFound) {
			return Running{}, nil
		}
		if err == nil {
			if r.ErrorMessage != nil && *r.ErrorMessage != "" {
				return r, ErrUncertain
			}
			if r.Running == running {
				return r, nil
			}
		}
		select {
		case <-ctx.Done():
			return Running{}, ErrUncertain
		case <-time.After(200 * time.Millisecond):
		}
	}
}
