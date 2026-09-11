// Package forwarding owns session-scoped LAN and serial mappings, separate from Maintenance.
package forwarding

import (
	"context"
	"errors"
	"net"
	"routerprobe/internal/serialauth"
	"time"
)

var ErrUnavailable = errors.New("forwarding unavailable: install configured GOST/agent and reconnect Probe")
var ErrNotFound = errors.New("forwarding not found")
var ErrInvalid = errors.New("invalid forwarding request")
var ErrCapacity = errors.New("forwarding capacity exhausted")

type Request struct {
	DeviceID     string `json:"device_id"`
	Kind         string `json:"kind"`
	Protocol     string `json:"protocol"`
	Interface    string `json:"interface"`
	TargetIP     string `json:"target_ip"`
	TargetPort   int    `json:"target_port"`
	Serial       string `json:"serial"`
	Baud         int    `json:"baud"`
	DataBits     int    `json:"data_bits"`
	StopBits     int    `json:"stop_bits"`
	Parity       string `json:"parity"`
	LeaseMinutes *int   `json:"lease_minutes"`
}

func (q *Request) Normalize() error {
	if q.DeviceID == "" {
		return ErrInvalid
	}
	if q.LeaseMinutes == nil {
		n := 240
		q.LeaseMinutes = &n
	}
	if *q.LeaseMinutes < 0 || *q.LeaseMinutes > 525600 {
		return ErrInvalid
	}
	if q.Kind == "lan" {
		ip := net.ParseIP(q.TargetIP)
		if (q.Protocol != "tcp" && q.Protocol != "udp") || q.Interface == "" || len(q.Interface) > 64 || ip == nil || ip.To4() == nil || !ip.IsGlobalUnicast() || ip.IsLoopback() || q.TargetPort < 1 || q.TargetPort > 65535 {
			return ErrInvalid
		}
		q.TargetIP = ip.String()
		q.Serial = ""
		q.Baud = 0
		q.DataBits = 0
		q.StopBits = 0
		q.Parity = ""
	} else if q.Kind == "serial" {
		if q.Protocol != "tcp" || q.Serial == "" || len(q.Serial) > 128 {
			return ErrInvalid
		}
		if q.Baud == 0 {
			q.Baud = 115200
		}
		if q.DataBits == 0 {
			q.DataBits = 8
		}
		if q.StopBits == 0 {
			q.StopBits = 1
		}
		if q.Parity == "" {
			q.Parity = "none"
		}
		if q.Baud < 50 || q.Baud > 4000000 || q.DataBits != 8 || q.StopBits != 1 || (q.Parity != "none" && q.Parity != "odd" && q.Parity != "even") {
			return ErrInvalid
		}
		q.Interface = ""
		q.TargetIP = ""
		q.TargetPort = 0
	} else {
		return ErrInvalid
	}
	return nil
}

type Command struct {
	ID          string  `json:"id"`
	SessionID   string  `json:"session_id"`
	Op          string  `json:"op"`
	Request     Request `json:"request"`
	Relay       string  `json:"relay,omitempty"`
	Secret      string  `json:"secret,omitempty"`
	Certificate string  `json:"certificate,omitempty"`
	Listen      string  `json:"listen,omitempty"`
}
type Interface struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}
type Inventory struct {
	Interfaces []Interface `json:"interfaces"`
	Serials    []string    `json:"serials"`
	Backend    bool        `json:"backend"`
}
type Status struct {
	ID        string     `json:"id"`
	SessionID string     `json:"session_id"`
	State     string     `json:"state"`
	Reason    string     `json:"reason,omitempty"`
	SourceIP  string     `json:"source_ip,omitempty"`
	Inventory *Inventory `json:"inventory,omitempty"`
}
type Binding struct {
	ID      string
	Done    <-chan struct{}
	Enqueue func(context.Context, Command) error
}
type Control interface{ BindForwarding(string) (Binding, error) }
type Snapshot struct {
	ID string `json:"id"`
	Request
	SessionID      string               `json:"session_id"`
	State          string               `json:"state"`
	Reason         string               `json:"reason"`
	Host           string               `json:"host"`
	Port           int                  `json:"port"`
	SourceIP       string               `json:"source_ip"`
	CreatedAt      time.Time            `json:"created_at"`
	ExpiresAt      *time.Time           `json:"expires_at"`
	ClosedAt       *time.Time           `json:"closed_at"`
	ReusableAfter  *time.Time           `json:"reusable_after"`
	Released       bool                 `json:"released"`
	DeviceReleased bool                 `json:"device_released"`
	SerialAuth     *serialauth.Snapshot `json:"serial_auth,omitempty"`
}
type Created struct {
	Mapping      Snapshot `json:"mapping"`
	Registration string   `json:"registration,omitempty"`
}
