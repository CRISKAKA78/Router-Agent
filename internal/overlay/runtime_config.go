package overlay

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// runtimeConfig uses the official, read-only ShowNodeInfo RPC. Unlike the Web
// NetworkConfig conversion, config.dump preserves manual routes with zero CIDRs.
func (w *WebClient) runtimeConfig(ctx context.Context, machine, instance string) (map[string]any, error) {
	b, e := hex.DecodeString(strings.ReplaceAll(instance, "-", ""))
	if e != nil || len(b) != 16 {
		return nil, ErrInvalid
	}
	parts := map[string]uint32{"part1": binary.BigEndian.Uint32(b[:4]), "part2": binary.BigEndian.Uint32(b[4:8]), "part3": binary.BigEndian.Uint32(b[8:12]), "part4": binary.BigEndian.Uint32(b[12:])}
	q := map[string]any{"service_name": "api.instance.PeerManageRpcService", "method_name": "ShowNodeInfo", "payload": map[string]any{"instance": map[string]any{"selector": map[string]any{"Id": parts}}}}
	var reply struct {
		Node *struct {
			Instance string `json:"inst_id"`
			Config   string `json:"config"`
		} `json:"node_info"`
	}
	if e := w.call(ctx, "POST", "/api/v1/machines/"+machine+"/proxy-rpc", q, &reply); e != nil {
		return nil, ErrUpstream
	}
	if reply.Node == nil || reply.Node.Instance != instance || reply.Node.Config == "" {
		return nil, ErrUncertain
	}
	var raw map[string]any
	if toml.Unmarshal([]byte(reply.Node.Config), &raw) != nil {
		return nil, ErrUncertain
	}
	return raw, nil
}
func valuesEqual(a, b any) bool {
	aa, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(aa) == string(bb)
}
func rawConfigMatches(raw, expected map[string]any) bool {
	flags, _ := raw["flags"].(map[string]any)
	// dump() deliberately omits values equal to gen_default_flags in 2.6.4.
	// Merge only pinned, supported defaults; a missing routes field is NOT true.
	effective := map[string]any{"mtu": 1380, "bind_device": true, "multi_thread": true, "dev_name": "", "proxy_forward_by_system": false, "private_mode": false, "lazy_p2p": false, "need_p2p": false, "p2p_only": false, "disable_p2p": false, "disable_sym_hole_punching": false, "disable_upnp": false}
	for key, value := range flags {
		effective[key] = value
	}
	flags = effective
	identity, _ := raw["network_identity"].(map[string]any)
	list := func(key, field string) []string {
		out := []string{}
		if rows, ok := raw[key].([]any); ok {
			for _, row := range rows {
				if m, ok := row.(map[string]any); ok {
					if s, ok := m[field].(string); ok {
						out = append(out, s)
					}
				}
			}
		}
		return out
	}
	for k, v := range expected {
		var a any
		switch k {
		case "network_name", "network_secret":
			a = identity[k]
		case "networking_method":
			continue // Encoded by the actual peer list, checked separately.
		case "virtual_ipv4", "network_length":
			if expected["dhcp"] == true {
				continue
			}
			ip, _ := raw["ipv4"].(string)
			want, _ := expected["virtual_ipv4"].(string)
			prefix, _ := json.Marshal(expected["network_length"])
			if ip != want+"/"+string(prefix) {
				return false
			}
			continue
		case "peer_urls":
			a = list("peer", "uri")
		case "listener_urls":
			a = raw["listeners"]
		case "proxy_cidrs":
			a = list("proxy_network", "cidr")
		case "enable_manual_routes":
			_, present := raw["routes"]
			a = present
		case "routes":
			if expected["enable_manual_routes"] != true {
				continue
			}
			a = raw["routes"]
		case "enable_private_mode":
			a = flags["private_mode"]
		case "instance_id", "hostname", "dhcp":
			a = raw[k]
		default:
			a = flags[k]
		}
		if a == nil { // TOML optional false flags are semantically the official false default.
			if b, ok := v.(bool); keyIsFalseRoot(k) && ok && !b {
				continue
			}
			return false
		}
		if reflect.ValueOf(v).Kind() == reflect.Slice && reflect.ValueOf(a).Kind() == reflect.Slice {
			// Ordering of peers/routes/listeners is not an identity or routing property.
			av := reflect.ValueOf(a)
			bv := reflect.ValueOf(v)
			if av.Len() != bv.Len() {
				return false
			}
			counts := map[string]int{}
			for i := 0; i < av.Len(); i++ {
				j, _ := json.Marshal(av.Index(i).Interface())
				counts[string(j)]++
			}
			for i := 0; i < bv.Len(); i++ {
				j, _ := json.Marshal(bv.Index(i).Interface())
				counts[string(j)]--
				if counts[string(j)] < 0 {
					return false
				}
			}
			continue
		}
		if !valuesEqual(a, v) {
			return false
		}
	}
	return true
}

func keyIsFalseRoot(key string) bool { return key == "dhcp" }
