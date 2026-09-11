package overlay

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Run only in the dedicated network/mount/PID namespace documented in
// OVERLAY_REDESIGN_VERIFICATION. No production address is used by this test.
func TestOfficialEasyTierRuntimeConfig(t *testing.T) {
	bin := os.Getenv("RMP_EASYTIER_TEST_BIN")
	if bin == "" {
		t.Skip("official EasyTier binaries and isolated Linux namespace required")
	}
	ifaces, _ := net.Interfaces()
	for _, i := range ifaces {
		if i.Name != "lo" && i.Name != "et-test" {
			t.Fatal("official test requires a isolated network namespace containing only lo and dummy et-test")
		}
	}
	dir := t.TempDir()
	seed, e := os.ReadFile(os.Getenv("RMP_EASYTIER_TEST_DB"))
	if e != nil {
		t.Fatal("isolated official account seed required", e)
	}
	if e = os.WriteFile(filepath.Join(dir, "web.db"), seed, 0600); e != nil {
		t.Fatal(e)
	}
	start := func(name string, args ...string) *exec.Cmd {
		cmd := exec.Command(filepath.Join(bin, name), args...)
		log, e := os.Create(filepath.Join(dir, name+".log"))
		if e != nil {
			t.Fatal(e)
		}
		cmd.Stdout = log
		cmd.Stderr = log
		if e = cmd.Start(); e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			_ = log.Close()
			if t.Failed() {
				data, _ := os.ReadFile(log.Name())
				t.Logf("isolated %s log: %s", name, data)
			}
		})
		return cmd
	}
	start("easytier-web", "--db", filepath.Join(dir, "web.db"), "--config-server-protocol", "tcp", "--config-server-port", "22020", "--api-server-addr", "127.0.0.1", "--api-server-port", "11211", "--console-log-level", "warn")
	cfg := testConfig()
	cfg.Username = "overlay-test"
	cfg.Password = "admin"
	cfg.ConfigServerURL = "tcp://127.0.0.1:22020/overlay-test"
	web, e := NewWebClient(cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer web.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancel()
	for {
		e = web.login(ctx)
		if e == nil {
			break
		}
		if ctx.Err() != nil {
			t.Fatal("local official Web login", e)
		}
		time.Sleep(200 * time.Millisecond)
	}
	if e := os.Mkdir(filepath.Join(dir, "configs"), 0700); e != nil {
		t.Fatal(e)
	}
	machine := MachineID("FE7140555489")
	instance := UUID()
	start("easytier-core", "--config-server", cfg.ConfigServerURL, "--machine-id", "FE7140555489", "--config-dir", filepath.Join(dir, "configs"), "--rpc-portal", "127.0.0.1:15990")
	for {
		if web.Ready(ctx, machine) == nil {
			break
		}
		if ctx.Err() != nil {
			t.Fatal("official raw device ID did not register expected UUID", machine)
		}
		time.Sleep(200 * time.Millisecond)
	}
	n := Network{ID: UUID(), Profile: 2, Spec: Spec{Name: "isolated", CIDR: "10.144.144.0/24", MTU: 1380, PeerURLs: []string{}, Routes: []string{}}}
	c := DefaultMemberConfig("isolated-router")
	c.VirtualIP = "10.144.144.1"
	m := Member{MachineID: machine, InstanceID: instance, DeviceID: "FE7140555489", VirtualIP: c.VirtualIP, Config: &c}
	desired := EngineConfig(n, m, "isolated-test-password")
	if e = web.Apply(ctx, machine, instance, desired); e != nil {
		t.Fatal("official apply", e)
	}
	s := &Service{controller: web}
	if _, e = s.waitRuntime(ctx, machine, instance, true); e != nil {
		t.Fatal("official runtime", e)
	}
	raw, e := web.runtimeConfig(ctx, machine, instance)
	if e != nil {
		t.Fatal("official raw RPC", e)
	}
	if !rawConfigMatches(raw, desired) {
		for key, value := range desired {
			one := map[string]any{key: value, "dhcp": desired["dhcp"], "virtual_ipv4": desired["virtual_ipv4"], "network_length": desired["network_length"], "enable_manual_routes": desired["enable_manual_routes"]}
			if !rawConfigMatches(raw, one) {
				t.Errorf("official raw mismatch at key %s", key)
			}
		}
		t.Logf("raw mtu=%v bind_device=%v flags=%v", raw["mtu"], raw["bind_device"], raw["flags"])
		t.Fatal("official raw configuration differs from desired")
	}
	if e = web.Confirm(ctx, machine, instance, desired); e != nil {
		t.Fatal("enabled empty routes confirmation", e)
	}
	if _, present := raw["routes"]; !present {
		t.Fatal("official empty manual routes not represented")
	}
	for _, manual := range []bool{false, true} {
		if e = web.Stop(ctx, machine, instance); e != nil {
			t.Fatal(e)
		}
		if _, e = s.waitRuntime(ctx, machine, instance, false); e != nil {
			t.Fatal(e)
		}
		c.ManualRoutes = manual
		c.LazyP2P = true
		c.NeedP2P = true
		c.P2POnly = manual
		c.DisableP2P = !manual
		c.ProxyCIDRs = []string{"192.168.50.0/24"}
		c.Routes = []string{}
		if manual {
			c.Routes = []string{"192.168.60.0/24"}
		}
		m.Config = &c
		desired = EngineConfig(n, m, "isolated-test-password")
		if e = web.Apply(ctx, machine, instance, desired); e != nil {
			t.Fatal(e)
		}
		if _, e = s.waitRuntime(ctx, machine, instance, true); e != nil {
			t.Fatal(e)
		}
		if e = web.Confirm(ctx, machine, instance, desired); e != nil {
			t.Fatal("official routing mode round-trip", manual, e)
		}
	}
	if e = web.Stop(ctx, machine, instance); e != nil {
		t.Fatal(e)
	}
	if _, e = s.waitRuntime(ctx, machine, instance, false); e != nil {
		t.Fatal(e)
	}
}
