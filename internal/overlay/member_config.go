package overlay

import (
	"net/netip"
	"strings"
)

// MemberConfig is a desired instance configuration, not a promise of hot reload.
type MemberConfig struct {
	Hostname      string   `json:"hostname"`
	VirtualIP     string   `json:"virtual_ip"`
	SystemForward bool     `json:"system_forward"`
	LazyP2P       bool     `json:"lazy_p2p"`
	NeedP2P       bool     `json:"need_p2p"`
	P2POnly       bool     `json:"p2p_only"`
	DisableP2P    bool     `json:"disable_p2p"`
	ProxyCIDRs    []string `json:"proxy_cidrs"`
	ManualRoutes  bool     `json:"enable_manual_routes"`
	Routes        []string `json:"routes"`
}

func DefaultMemberConfig(name string) MemberConfig {
	return MemberConfig{Hostname: name, SystemForward: true, ManualRoutes: true, ProxyCIDRs: []string{}, Routes: []string{}}
}
func (c *MemberConfig) Validate(n Network, device string) error {
	c.Hostname = strings.TrimSpace(c.Hostname)
	if c.Hostname == "" {
		c.Hostname = device
	}
	if len(c.Hostname) > 128 || strings.ContainsAny(c.Hostname, "\x00\r\n") {
		return ErrInvalid
	}
	if c.VirtualIP != "" {
		p, e := netip.ParsePrefix(n.CIDR)
		ip, x := netip.ParseAddr(c.VirtualIP)
		if e != nil || x != nil || !ip.Is4() || !p.Contains(ip) || p.Addr() == ip {
			return ErrInvalid
		}
		a := ip.As4()
		v := uint32(a[0])<<24 | uint32(a[1])<<16 | uint32(a[2])<<8 | uint32(a[3])
		mask := uint32(1)<<uint(32-p.Bits()) - 1
		if v&mask == mask {
			return ErrInvalid
		}
		for _, m := range n.Members {
			if m.DeviceID != device && m.VirtualIP == c.VirtualIP {
				return ErrConflict
			}
		}
	}
	for _, list := range []*[]string{&c.ProxyCIDRs, &c.Routes} {
		if len(*list) > 32 {
			return ErrInvalid
		}
		seen := map[string]bool{}
		for _, v := range *list {
			p, e := netip.ParsePrefix(v)
			if e != nil || !p.Addr().Is4() || p.Masked().String() != v || p.Bits() == 0 || seen[v] {
				return ErrInvalid
			}
			seen[v] = true
		}
		if *list == nil {
			*list = []string{}
		}
	}
	if !c.ManualRoutes && len(c.Routes) > 0 {
		return ErrInvalid
	}
	return nil
}
func hasAnchor(n Network, except string) bool {
	for _, m := range n.Members {
		if m.DeviceID != except && m.VirtualIP != "" {
			return true
		}
	}
	return false
}
func hasDHCP(n Network, except string) bool {
	for _, m := range n.Members {
		if m.DeviceID != except && m.VirtualIP == "" {
			return true
		}
	}
	return false
}

// Password is fetched explicitly; it never enters network DTOs or observations.
func (s *Service) Password(id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.data.Networks[id]
	if !ok {
		return "", ErrNotFound
	}
	return n.Secret, nil
}
func (s *Service) CreateConfigured(q Spec, password string) (Network, error) {
	if len(password) < 1 || len(password) > 128 || strings.ContainsAny(password, "\x00\r\n") {
		return Network{}, ErrInvalid
	}
	if len(q.Routes) > 0 {
		return Network{}, ErrInvalid
	}
	if len(q.PeerURLs) == 0 {
		q.PeerURLs = []string{"tcp://47.119.168.150:11010", "udp://47.119.168.150:11010"}
	}
	if q.MTU == 0 {
		q.MTU = 1380
	}
	return s.create(q, password, 2)
}

// Configuration changes and their operation are committed together. Stopped
// members remain stopped; a running member gets one instance-scoped reapply.
func (s *Service) ConfigureMember(id, device string, cfg MemberConfig, revision uint64) (Member, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Member{}, ErrConflict
	}
	sn, ok := s.data.Networks[id]
	sn = clone(sn)
	if !ok {
		return Member{}, ErrNotFound
	}
	index := -1
	for i, m := range sn.Network.Members {
		if m.DeviceID == device {
			index = i
		}
	}
	if index < 0 {
		return Member{}, ErrNotFound
	}
	m := &sn.Network.Members[index]
	if m.ConfigRevision != revision || s.memberBusy(*m) {
		return Member{}, ErrConflict
	}
	if e := cfg.Validate(sn.Network, device); e != nil {
		return Member{}, e
	}
	if cfg.VirtualIP == "" && !hasAnchor(sn.Network, device) && (sn.Network.Profile >= 2 || hasDHCP(sn.Network, device)) {
		return Member{}, ErrConflict
	}
	if m.Desired == "start" {
		if e := s.driver.ValidateDevice(device); e != nil {
			return Member{}, e
		}
		if len(s.data.Operations) >= 2048 {
			return Member{}, ErrConflict
		}
	}
	m.Config = &cfg
	m.VirtualIP = cfg.VirtualIP
	m.ConfigRevision++
	data := clone(s.data)
	var op Operation
	if m.Desired == "start" {
		op = newOperation(sn.Network, *m, "start")
		m.OperationID = op.ID
		data.Operations[op.ID] = op
	}
	data.Networks[id] = sn
	if e := s.commit(data); e != nil {
		return Member{}, e
	}
	if op.ID != "" {
		s.launch(op, clone(sn.Network), *m, sn.Secret)
	}
	return clone(*m), nil
}

func addressMatches(n Network, m Member, r Running) error {
	ip, e := netip.ParseAddr(r.MyNode.IP.Address)
	prefix, _ := netip.ParsePrefix(n.CIDR)
	if e != nil || !prefix.Contains(ip) || (m.VirtualIP != "" && m.VirtualIP != ip.String()) {
		return ErrUncertain
	}
	return nil
}
