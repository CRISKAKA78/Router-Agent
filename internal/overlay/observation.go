package overlay

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/netip"
	"sort"
	"strconv"
	"time"
)

// Upstream prost JSON permits integer counters as numbers or decimal strings.
type Counter uint64

func (v *Counter) UnmarshalJSON(b []byte) error {
	var s string
	if len(b) > 0 && b[0] == '"' {
		if e := json.Unmarshal(b, &s); e != nil {
			return e
		}
	} else {
		s = string(b)
	}
	n, e := strconv.ParseUint(s, 10, 64)
	*v = Counter(n)
	return e
}

type IPValue struct{ Address string }

func (v *IPValue) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}
	var s string
	if json.Unmarshal(b, &s) == nil {
		v.Address = s
		return nil
	}
	var m map[string]json.RawMessage
	if e := json.Unmarshal(b, &m); e != nil {
		return e
	}
	if a, ok := m["address"]; ok {
		return v.UnmarshalJSON(a)
	}
	if a, ok := m["addr"]; ok {
		var n Counter
		if e := json.Unmarshal(a, &n); e != nil {
			return e
		}
		if uint64(n) > 1<<32-1 {
			return ErrInvalid
		}
		var octets [4]byte
		binary.BigEndian.PutUint32(octets[:], uint32(n))
		v.Address = netip.AddrFrom4(octets).String()
	}
	return nil
}

type NATInfo struct {
	UDP int `json:"udp_nat_type"`
	TCP int `json:"tcp_nat_type"`
}
type Running struct {
	DevName      string  `json:"dev_name"`
	Running      bool    `json:"running"`
	ErrorMessage *string `json:"error_msg"`
	MyNode       struct {
		PeerID   uint32  `json:"peer_id"`
		IP       IPValue `json:"virtual_ipv4"`
		Version  string  `json:"version"`
		Hostname string  `json:"hostname"`
		NAT      NATInfo `json:"stun_info"`
	} `json:"my_node_info"`
	Peers []struct {
		PeerID      uint32 `json:"peer_id"`
		Connections []struct {
			PeerID uint32 `json:"peer_id"`
			Closed bool   `json:"is_closed"`
			Tunnel *struct {
				Type   string `json:"tunnel_type"`
				Remote struct {
					URL string `json:"url"`
				} `json:"remote_addr"`
			} `json:"tunnel"`
			Stats *struct {
				RX      Counter `json:"rx_bytes"`
				TX      Counter `json:"tx_bytes"`
				Latency Counter `json:"latency_us"`
			} `json:"stats"`
			Loss *float64 `json:"loss_rate"`
		} `json:"conns"`
	} `json:"peers"`
	Routes []struct {
		PeerID     uint32   `json:"peer_id"`
		IP         IPValue  `json:"ipv4_addr"`
		NextHop    uint32   `json:"next_hop_peer_id"`
		Cost       int      `json:"cost"`
		Hostname   string   `json:"hostname"`
		NAT        NATInfo  `json:"stun_info"`
		InstanceID string   `json:"inst_id"`
		ProxyCIDRs []string `json:"proxy_cidrs"`
	} `json:"routes"`
}
type Link struct {
	RemoteURL string   `json:"remote_url,omitempty"`
	PeerID    uint32   `json:"peer_id"`
	Transport string   `json:"transport"`
	RX        *uint64  `json:"rx_bytes"`
	TX        *uint64  `json:"tx_bytes"`
	LatencyMS *float64 `json:"latency_ms"`
	Loss      *float64 `json:"loss_rate"`
}
type Route struct {
	Hostname   string   `json:"hostname"`
	NAT        NATInfo  `json:"nat"`
	InstanceID string   `json:"instance_id"`
	PeerID     uint32   `json:"peer_id"`
	VirtualIP  string   `json:"virtual_ip"`
	NextHop    uint32   `json:"next_hop_peer_id"`
	Cost       int      `json:"cost"`
	ProxyCIDRs []string `json:"proxy_cidrs"`
}
type Observation struct {
	Hostname  string    `json:"hostname"`
	NAT       NATInfo   `json:"nat"`
	DeviceID  string    `json:"device_id"`
	State     string    `json:"state"`
	PeerID    uint32    `json:"peer_id"`
	VirtualIP string    `json:"virtual_ip"`
	Version   string    `json:"version"`
	Interface string    `json:"interface"`
	SampledAt time.Time `json:"sampled_at"`
	Stale     bool      `json:"stale"`
	Limited   bool      `json:"limited"`
	Error     string    `json:"error,omitempty"`
	Links     []Link    `json:"links"`
	Routes    []Route   `json:"routes"`
}

func observe(device string, r Running, now time.Time) Observation {
	o := Observation{Hostname: r.MyNode.Hostname, NAT: r.MyNode.NAT, DeviceID: device, State: "stopped", PeerID: r.MyNode.PeerID, VirtualIP: r.MyNode.IP.Address, Version: r.MyNode.Version, Interface: r.DevName, SampledAt: now, Links: []Link{}, Routes: []Route{}}
	if r.Running {
		o.State = "running"
	}
	if r.ErrorMessage != nil && *r.ErrorMessage != "" {
		o.State = "error"
		o.Error = "engine_error"
	} // No raw errors/secrets.
	for _, p := range r.Peers {
		for _, c := range p.Connections {
			if c.Closed {
				continue
			}
			if len(o.Links) >= 128 {
				o.Limited = true
				break
			}
			l := Link{PeerID: p.PeerID, Loss: c.Loss}
			if c.Tunnel != nil {
				l.Transport = c.Tunnel.Type
				l.RemoteURL = c.Tunnel.Remote.URL
			}
			if c.Stats != nil {
				rx, tx, lat := uint64(c.Stats.RX), uint64(c.Stats.TX), float64(c.Stats.Latency)/1000
				l.RX = &rx
				l.TX = &tx
				l.LatencyMS = &lat
			}
			o.Links = append(o.Links, l)
		}
	}
	for _, r := range r.Routes {
		if len(o.Routes) >= 128 {
			o.Limited = true
			break
		}
		cidrs := r.ProxyCIDRs
		if len(cidrs) > 32 {
			cidrs = cidrs[:32]
			o.Limited = true
		}
		if cidrs == nil {
			cidrs = []string{}
		}
		o.Routes = append(o.Routes, Route{PeerID: r.PeerID, VirtualIP: r.IP.Address, NextHop: r.NextHop, Cost: r.Cost, ProxyCIDRs: cidrs, Hostname: r.Hostname, NAT: r.NAT, InstanceID: r.InstanceID})
	}
	return o
}

type Node struct {
	Hostname         string  `json:"hostname"`
	NAT              NATInfo `json:"nat"`
	ID               string  `json:"id"`
	DeviceID         string  `json:"device_id,omitempty"`
	PeerID           uint32  `json:"peer_id"`
	VirtualIP        string  `json:"virtual_ip"`
	State            string  `json:"state"`
	ManagementOnline bool    `json:"management_online"`
	External         bool    `json:"external"`
}
type Edge struct {
	Source        string    `json:"source"`
	Target        string    `json:"target"`
	Transport     string    `json:"transport"`
	ReportedBy    string    `json:"reported_by"`
	ObservedAt    time.Time `json:"observed_at"`
	ConfirmedBoth bool      `json:"confirmed_both"`
	Stale         bool      `json:"stale"`
	Link          Link      `json:"link"`
}
type Topology struct {
	Nodes        []Node        `json:"nodes"`
	Edges        []Edge        `json:"edges"`
	Observations []Observation `json:"observations"`
	Limited      bool          `json:"limited"`
}

func topology(n Network, observations map[string]Observation, online func(string) bool, now time.Time) Topology {
	t := Topology{Nodes: []Node{}, Edges: []Edge{}, Observations: []Observation{}}
	peers := map[uint32]string{}
	known := map[string]bool{}
	for _, m := range n.Members {
		o, ok := observations[m.DeviceID]
		if !ok {
			o = Observation{DeviceID: m.DeviceID, State: "unknown", Stale: true, Links: []Link{}, Routes: []Route{}}
		}
		o.Stale = o.Stale || now.Sub(o.SampledAt) > 35*time.Second
		t.Observations = append(t.Observations, o)
		state := o.State
		if o.Stale {
			state = "unknown"
		}
		ip := o.VirtualIP
		if ip == "" {
			ip = m.VirtualIP
		}
		id := "device:" + m.DeviceID
		t.Nodes = append(t.Nodes, Node{ID: id, DeviceID: m.DeviceID, PeerID: o.PeerID, VirtualIP: ip, State: state, ManagementOnline: online(m.DeviceID), External: false, Hostname: o.Hostname, NAT: o.NAT})
		known[id] = true
		if o.PeerID != 0 && !o.Stale {
			if previous, exists := peers[o.PeerID]; exists && previous != id {
				peers[o.PeerID] = ""
				t.Limited = true
			} else {
				peers[o.PeerID] = id
			}
		}
		t.Limited = t.Limited || o.Limited
	}
	// Route-only nodes must remain visible, including nodes reached via relay.
	for _, o := range t.Observations {
		for _, r := range o.Routes {
			if r.PeerID == 0 {
				continue
			}
			id := peers[r.PeerID]
			if id == "" && !o.Stale && r.InstanceID != "" {
				for _, m := range n.Members {
					if m.InstanceID == r.InstanceID {
						id = "device:" + m.DeviceID
						peers[r.PeerID] = id
						break
					}
				}
			}
			if id == "" {
				id = fmt.Sprintf("peer:%d", r.PeerID)
			}
			if !known[id] {
				if len(t.Nodes) >= 512 {
					t.Limited = true
					break
				}
				known[id] = true
				t.Nodes = append(t.Nodes, Node{ID: id, PeerID: r.PeerID, VirtualIP: r.VirtualIP, State: "observed_peer", External: true, Hostname: r.Hostname, NAT: r.NAT})
			}
		}
	}
	reported := map[string]bool{}
	for _, o := range t.Observations {
		if !o.Stale {
			for _, l := range o.Links {
				reported[fmt.Sprintf("%d/%d", o.PeerID, l.PeerID)] = true
			}
		}
	}
	for _, o := range t.Observations {
		for _, l := range o.Links {
			if len(t.Edges) >= 512 {
				t.Limited = true
				break
			}
			target := peers[l.PeerID]
			if target == "" {
				target = fmt.Sprintf("peer:%d", l.PeerID)
			}
			if !known[target] {
				known[target] = true
				t.Nodes = append(t.Nodes, Node{ID: target, PeerID: l.PeerID, State: "observed_peer", External: true})
			}
			t.Edges = append(t.Edges, Edge{"device:" + o.DeviceID, target, l.Transport, o.DeviceID, o.SampledAt, !o.Stale && reported[fmt.Sprintf("%d/%d", l.PeerID, o.PeerID)], o.Stale, l})
		}
	}
	sort.Slice(t.Nodes, func(i, j int) bool { return t.Nodes[i].ID < t.Nodes[j].ID })
	return t
}
