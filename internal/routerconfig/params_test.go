package routerconfig

import (
	"strings"
	"testing"
)

func str(s string) *string { return &s }
func TestParams(t *testing.T) {
	for _, p := range []Params{
		{Backend: "nvram", Operation: "get", Key: str("pci/1/1/venid")},
		{Backend: "nvram", Operation: "set", Key: str("SN"), Value: str("")},
		{Backend: "nvram", Operation: "set", Key: str("name"), Value: str(" a'\";$(touch /tmp/no)\n中文 ")},
		{Backend: "nvram", Operation: "delete", Key: str("lan_ipaddr")},
		{Backend: "nvram", Operation: "commit"},
		{Backend: "uci", Operation: "get", Key: str("system.@system[-1].hostname")},
		{Backend: "uci", Operation: "set", Key: str("network.lan.ipaddr"), Value: str("192.168.1.1")},
		{Backend: "uci", Operation: "delete", Key: str("system.main.hostname")},
		{Backend: "uci", Operation: "commit", Package: str("system")},
	} {
		if err := p.Validate(); err != nil {
			t.Errorf("valid %+v: %v", p, err)
		}
	}
	for _, p := range []Params{
		{Backend: "other", Operation: "commit"},
		{Backend: "nvram", Operation: "set", Key: str("SN")},
		{Backend: "nvram", Operation: "get", Key: str("-x")},
		{Backend: "nvram", Operation: "get", Key: str("a=b")},
		{Backend: "nvram", Operation: "get", Key: str("x"), Value: str("")},
		{Backend: "nvram", Operation: "commit", Key: str("")},
		{Backend: "nvram", Operation: "commit", Package: str("")},
		{Backend: "nvram", Operation: "set", Key: str("x"), Value: str("a\x00b")},
		{Backend: "nvram", Operation: "set", Key: str("x"), Value: str("\xff")},
		{Backend: "nvram", Operation: "set", Key: str("x"), Value: str(strings.Repeat("x", 4097))},
		{Backend: "uci", Operation: "commit"},
		{Backend: "uci", Operation: "commit", Package: str("system.main")},
		{Backend: "uci", Operation: "get", Key: str("system.@system[].hostname")},
		{Backend: "uci", Operation: "delete", Key: str("network.lan")},
		{Backend: "uci", Operation: "get", Key: str("network.lan.ipaddr;reboot")},
		{Backend: "uci", Operation: "show", Key: str("network.lan.ipaddr")},
	} {
		if p.Validate() == nil {
			t.Errorf("invalid accepted %+v", p)
		}
	}
}
