// Package overlay owns the desired networks and the EasyTier configuration-service adapter.
// It never carries user traffic or exposes upstream credentials to an HTTP client.
package overlay

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

var (
	ErrInvalid   = errors.New("invalid network request")
	ErrNotFound  = errors.New("network resource not found")
	ErrConflict  = errors.New("network operation conflicts with current state")
	ErrDisabled  = errors.New("EasyTier configuration service is not configured")
	ErrUpstream  = errors.New("EasyTier configuration service unavailable")
	ErrUncertain = errors.New("network operation outcome is uncertain; reconcile before retrying")
)

const EngineVersion = "2.6.4"

type Config struct {
	APIURL           string `json:"api_url"`
	Username         string `json:"username"`
	Password         string `json:"password"`
	ConfigServerURL  string `json:"config_server_url"`
	ToolID           string `json:"tool_id"`
	ToolVersion      string `json:"tool_version"`
	InstallDirectory string `json:"install_directory"`
}

func LoadConfig(file string) (Config, error) {
	var c Config
	if file == "" {
		return c, nil
	}
	b, e := os.ReadFile(file)
	if e != nil {
		return c, e
	}
	if len(b) > 16384 {
		return c, ErrInvalid
	}
	d := json.NewDecoder(strings.NewReader(string(b)))
	d.DisallowUnknownFields()
	if e = d.Decode(&c); e != nil {
		return c, ErrInvalid
	}
	if d.Decode(new(any)) != io.EOF {
		return c, ErrInvalid
	}
	return c, c.Validate()
}
func (c *Config) Validate() error {
	if c.APIURL == "" {
		if c.Username != "" || c.Password != "" || c.ConfigServerURL != "" {
			return ErrInvalid
		}
		return nil
	}
	u, e := url.Parse(c.APIURL)
	if e != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || (u.Scheme != "http" && u.Scheme != "https") {
		return ErrInvalid
	}
	ip, e := netip.ParseAddr(u.Hostname())
	if e != nil || !ip.IsLoopback() {
		return fmt.Errorf("%w: api_url must use a literal loopback IP", ErrInvalid)
	}
	c.APIURL = strings.TrimRight(c.APIURL, "/")
	if c.Username == "" || c.Password == "" || len(c.Username) > 128 || len(c.Password) > 1024 || !ValidConfigServer(c.ConfigServerURL) {
		return ErrInvalid
	}
	if c.InstallDirectory == "" {
		c.InstallDirectory = "/tmp/root/router-agent-network"
	}
	if !ValidDirectory(c.InstallDirectory) {
		return ErrInvalid
	}
	if c.ToolVersion == "" {
		c.ToolVersion = EngineVersion
	}
	return nil
}
func ValidConfigServer(s string) bool {
	u, e := url.Parse(s)
	return e == nil && len(s) <= 512 && u.User == nil && u.Hostname() != "" && u.Port() != "" && u.RawQuery == "" && u.Fragment == "" && strings.HasPrefix(u.Path, "/") && len(u.Path) > 1 && !strings.ContainsAny(s, "\x00\r\n \t") && (u.Scheme == "tcp" || u.Scheme == "udp" || u.Scheme == "ws" || u.Scheme == "wss")
}
func ValidDirectory(s string) bool {
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("/-_.", c)) {
			return false
		}
	}
	return len(s) >= 8 && len(s) <= 160 && strings.HasPrefix(s, "/") && path.Clean(s) == s && s != "/tmp/root" && s != "/tmp" && !strings.ContainsAny(s, "\x00\r\n \t\\")
}
func UUID() string {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic(e)
	}
	b[6] = b[6]&15 | 64
	b[8] = b[8]&63 | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
func ValidUUID(s string) bool {
	if len(s) != 36 || s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return false
	}
	b, e := hex.DecodeString(strings.ReplaceAll(s, "-", ""))
	return e == nil && len(b) == 16 && strings.ToLower(s) == s
}

type Spec struct {
	Name     string   `json:"name"`
	CIDR     string   `json:"cidr"`
	PeerURLs []string `json:"peer_urls"`
	Routes   []string `json:"routes"`
}

func (q *Spec) Validate() error {
	q.Name = strings.TrimSpace(q.Name)
	if q.Name == "" || len(q.Name) > 128 || strings.ContainsAny(q.Name, "\x00\r\n") || len(q.PeerURLs) > 16 || len(q.Routes) > 32 {
		return ErrInvalid
	}
	if q.CIDR == "" {
		q.CIDR = "10.144.144.0/24"
	}
	p, e := netip.ParsePrefix(q.CIDR)
	if e != nil || !p.Addr().Is4() || !p.Addr().IsPrivate() || p.Bits() < 16 || p.Bits() > 29 || p.Masked().String() != q.CIDR {
		return ErrInvalid
	}
	for _, s := range q.PeerURLs {
		u, e := url.Parse(s)
		if e != nil || len(s) > 256 || u.User != nil || u.Hostname() == "" || u.Port() == "" || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "tcp" && u.Scheme != "udp" && u.Scheme != "ws" && u.Scheme != "wss" && u.Scheme != "quic") || strings.ContainsAny(s, "\x00\r\n \t") {
			return ErrInvalid
		}
	}
	for _, s := range q.Routes {
		p, e := netip.ParsePrefix(s)
		if e != nil || !p.Addr().Is4() || p.Masked().String() != s || p.Bits() == 0 {
			return ErrInvalid
		}
	}
	if q.PeerURLs == nil {
		q.PeerURLs = []string{}
	}
	if q.Routes == nil {
		q.Routes = []string{}
	}
	return nil
}

type Member struct {
	DeviceID        string `json:"device_id"`
	MachineID       string `json:"machine_id"`
	InstanceID      string `json:"instance_id"`
	VirtualIP       string `json:"virtual_ip"` // Empty preserves official DHCP default.
	Desired         string `json:"desired"`
	AppliedRevision uint64 `json:"applied_revision"`
	OperationID     string `json:"operation_id,omitempty"`
}
type Network struct {
	ID string `json:"network_id"`
	Spec
	Revision  uint64    `json:"revision"`
	CreatedAt time.Time `json:"created_at"`
	Members   []Member  `json:"members"`
}
type Operation struct {
	ID        string    `json:"operation_id"`
	NetworkID string    `json:"network_id"`
	DeviceID  string    `json:"device_id"`
	Action    string    `json:"action"`
	Revision  uint64    `json:"revision"`
	State     string    `json:"state"`
	Step      string    `json:"step"`
	TaskIDs   []string  `json:"task_ids"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type JoinRequest struct {
	DeviceID  string `json:"device_id"`
	VirtualIP string `json:"virtual_ip"`
}
type Status struct {
	Configured        bool     `json:"configured"`
	Engine            string   `json:"engine"`
	Version           string   `json:"version"`
	PackageConfigured bool     `json:"package_configured"`
	Listeners         []string `json:"listeners"`
	ManualRoutes      bool     `json:"manual_routes"`
	Routes            []string `json:"routes"`
}

// Matches v2.6.4 official Web DEFAULT_NETWORK_CONFIG. Unspecified optional
// flags retain upstream defaults. Only WG removal and manual routes differ.
// Static IP is an explicit member choice, never silently substituted for DHCP.
func EngineConfig(n Network, m Member, secret string) map[string]any {
	p, _ := netip.ParsePrefix(n.CIDR)
	c := map[string]any{"instance_id": m.InstanceID, "network_name": n.ID, "network_secret": secret, "dhcp": m.VirtualIP == "", "virtual_ipv4": m.VirtualIP, "network_length": p.Bits(), "networking_method": 1, "peer_urls": n.PeerURLs, "listener_urls": []string{"tcp://0.0.0.0:11010", "udp://0.0.0.0:11010"}, "enable_manual_routes": true, "routes": n.Routes, "bind_device": true, "multi_thread": true}
	return c
}
