package mqttclient

// Integration tests against a real MQTT broker (mochi-mqtt) running inside
// the test process, so no Pi or Mosquitto is needed.

import (
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	broker "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"

	"housecontrol/internal/config"
	"housecontrol/internal/devices"
)

const bridgeDevices = `[
 {"ieee_address":"0x00","friendly_name":"Coordinator","type":"Coordinator"},
 {"ieee_address":"0x01","friendly_name":"living/ceiling","type":"Router",
  "definition":{"model":"LED1545G12","vendor":"IKEA","description":"bulb","exposes":[
   {"type":"light","features":[
     {"type":"binary","name":"state","property":"state"},
     {"type":"numeric","name":"brightness","property":"brightness","value_max":254}]}]}}
]`

// --- test broker ------------------------------------------------------------

type testBroker struct {
	t         *testing.T
	addr      string
	srv       *broker.Server
	closeOnce sync.Once
}

// startBroker starts an in-process broker on addr ("" = random free port).
// With user != "" only that username/password is accepted.
func startBroker(t *testing.T, addr, user, pass string) *testBroker {
	t.Helper()
	if addr == "" {
		addr = freeAddr(t)
	}
	srv := broker.New(&broker.Options{
		InlineClient: true,
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)), // keep test output readable
	})
	var err error
	if user == "" {
		err = srv.AddHook(new(auth.AllowHook), nil)
	} else {
		err = srv.AddHook(new(auth.Hook), &auth.Options{
			Ledger: &auth.Ledger{
				Auth: auth.AuthRules{{Username: auth.RString(user), Password: auth.RString(pass), Allow: true}},
				ACL:  auth.ACLRules{{Filters: auth.Filters{"#": auth.ReadWrite}}},
			},
		})
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.AddListener(listeners.NewTCP(listeners.Config{ID: "tcp", Address: addr})); err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve() }()
	waitFor(t, 5*time.Second, "broker listening", func() bool {
		c, err := net.Dial("tcp", addr)
		if err == nil {
			c.Close()
		}
		return err == nil
	})
	b := &testBroker{t: t, addr: addr, srv: srv}
	t.Cleanup(b.close)
	return b
}

func (b *testBroker) url() string { return "tcp://" + b.addr }

// close is safe to call more than once (tests stop the broker themselves,
// and t.Cleanup closes it again; mochi panics on a double Close).
func (b *testBroker) close() { b.closeOnce.Do(func() { _ = b.srv.Close() }) }

func (b *testBroker) publish(topic, payload string, retain bool) {
	b.t.Helper()
	if err := b.srv.Publish(topic, []byte(payload), retain, 0); err != nil {
		b.t.Fatal(err)
	}
}

// capture records messages published by clients on filter.
func (b *testBroker) capture(filter string) *captured {
	c := &captured{}
	err := b.srv.Subscribe(filter, 1, func(_ *broker.Client, _ packets.Subscription, pk packets.Packet) {
		c.mu.Lock()
		c.msgs = append(c.msgs, pk.TopicName+" "+string(pk.Payload))
		c.mu.Unlock()
	})
	if err != nil {
		b.t.Fatal(err)
	}
	return c
}

type captured struct {
	mu   sync.Mutex
	msgs []string
}

func (c *captured) has(substr string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, m := range c.msgs {
		if strings.Contains(m, substr) {
			return true
		}
	}
	return false
}

// --- helpers -------------------------------------------------------------------

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()
	return addr
}

func waitFor(t *testing.T, timeout time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for: %s", what)
}

func newManager(t *testing.T) (*Manager, *devices.Registry) {
	reg := devices.NewRegistry()
	m := NewManager(reg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(m.Close)
	return m, reg
}

func mqttCfg(url string) config.MQTT {
	return config.MQTT{URL: url, BaseTopic: "zigbee2mqtt"}
}

func waitState(t *testing.T, m *Manager, state string) devices.MQTTStatus {
	t.Helper()
	waitFor(t, 15*time.Second, "mqtt state "+state, func() bool { return m.Status().State == state })
	return m.Status()
}

// --- tests -----------------------------------------------------------------------

func TestUnconfiguredDoesNotCrash(t *testing.T) {
	m, reg := newManager(t)
	m.Apply(config.MQTT{BaseTopic: "zigbee2mqtt"}, "config")

	if got := m.Status().State; got != StateUnconfigured {
		t.Fatalf("state = %q, want unconfigured", got)
	}
	if err := m.Set("lamp", map[string]any{"state": "ON"}); err != ErrNotConnected {
		t.Fatalf("Set error = %v, want ErrNotConnected", err)
	}
	if len(reg.List()) != 0 {
		t.Fatal("expected no devices")
	}
}

func TestUnreachableBrokerReportsErrorAndKeepsRunning(t *testing.T) {
	m, _ := newManager(t)
	m.Apply(mqttCfg("tcp://"+freeAddr(t)), "config") // nothing listening

	st := waitState(t, m, StateError)
	if st.Error == "" {
		t.Fatal("expected an error message for the UI")
	}
	if err := m.Set("lamp", map[string]any{"state": "ON"}); err != ErrNotConnected {
		t.Fatalf("Set error = %v, want ErrNotConnected", err)
	}
}

func TestConnectsLateWhenBrokerComesUp(t *testing.T) {
	addr := freeAddr(t)
	m, _ := newManager(t)
	m.Apply(mqttCfg("tcp://"+addr), "config")
	waitState(t, m, StateError)

	startBroker(t, addr, "", "") // broker appears after the app started
	waitState(t, m, StateConnected)
}

func TestDiscoversDevicesAndState(t *testing.T) {
	b := startBroker(t, "", "", "")
	b.publish("zigbee2mqtt/bridge/devices", bridgeDevices, true)
	b.publish("zigbee2mqtt/living/ceiling", `{"state":"ON","brightness":120}`, true)

	m, reg := newManager(t)
	m.Apply(mqttCfg(b.url()), "config")
	waitState(t, m, StateConnected)

	waitFor(t, 5*time.Second, "device + state", func() bool {
		d, ok := reg.Get("0x01")
		return ok && d.State["state"] == "ON"
	})
	d, _ := reg.Get("0x01")
	if d.Name != "living/ceiling" || d.Kind != devices.KindLight || !d.Capabilities.Brightness {
		t.Fatalf("unexpected device: %+v", d)
	}

	// live state update
	b.publish("zigbee2mqtt/living/ceiling", `{"state":"OFF"}`, false)
	waitFor(t, 5*time.Second, "state OFF", func() bool {
		d, _ := reg.Get("0x01")
		return d.State["state"] == "OFF"
	})
}

func TestSetPublishesCommand(t *testing.T) {
	b := startBroker(t, "", "", "")
	cmds := b.capture("zigbee2mqtt/+/+/set")

	m, _ := newManager(t)
	m.Apply(mqttCfg(b.url()), "config")
	waitState(t, m, StateConnected)

	if err := m.Set("living/ceiling", map[string]any{"state": "ON"}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 5*time.Second, "command on broker", func() bool {
		return cmds.has(`zigbee2mqtt/living/ceiling/set {"state":"ON"}`)
	})
}

func TestSwitchingBrokerReplacesDevices(t *testing.T) {
	a := startBroker(t, "", "", "")
	a.publish("zigbee2mqtt/bridge/devices", bridgeDevices, true)
	b := startBroker(t, "", "", "")
	b.publish("zigbee2mqtt/bridge/devices", `[{"ieee_address":"0x99","friendly_name":"plug","type":"Router"}]`, true)

	m, reg := newManager(t)
	m.Apply(mqttCfg(a.url()), "config")
	waitFor(t, 15*time.Second, "devices from A", func() bool { _, ok := reg.Get("0x01"); return ok })

	m.Apply(mqttCfg(b.url()), "ui")
	waitFor(t, 15*time.Second, "devices from B only", func() bool {
		_, fromA := reg.Get("0x01")
		_, fromB := reg.Get("0x99")
		return !fromA && fromB
	})
	if _, src := m.Config(); src != "ui" {
		t.Fatalf("source = %q", src)
	}
	if st := m.Status(); st.State != StateConnected || st.Broker != b.url() {
		t.Fatalf("status = %+v", st)
	}
}

func TestReconnectsAfterBrokerRestart(t *testing.T) {
	b := startBroker(t, "", "", "")
	m, _ := newManager(t)
	m.Apply(mqttCfg(b.url()), "config")
	waitState(t, m, StateConnected)

	b.close()
	waitFor(t, 15*time.Second, "noticed broker gone", func() bool { return m.Status().State != StateConnected })

	startBroker(t, b.addr, "", "")
	waitState(t, m, StateConnected)
}

func TestCredentials(t *testing.T) {
	b := startBroker(t, "", "ha", "secret")
	m, _ := newManager(t)

	bad := mqttCfg(b.url())
	bad.Username, bad.Password = "ha", "wrong"
	if err := m.Test(bad); err == nil {
		t.Fatal("Test with wrong password should fail")
	}

	good := bad
	good.Password = "secret"
	if err := m.Test(good); err != nil {
		t.Fatalf("Test with right password: %v", err)
	}

	m.Apply(bad, "ui")
	st := waitState(t, m, StateError)
	if st.Error == "" {
		t.Fatal("expected auth error message")
	}
	m.Apply(good, "ui")
	waitState(t, m, StateConnected)
}

func TestTestDoesNotDisturbLiveConnection(t *testing.T) {
	b := startBroker(t, "", "", "")
	cfg := mqttCfg(b.url())
	cfg.ClientID = "housecontrol"
	m, _ := newManager(t)
	m.Apply(cfg, "config")
	waitState(t, m, StateConnected)

	if err := m.Test(cfg); err != nil { // same client id configured
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if m.Status().State != StateConnected {
		t.Fatalf("live connection disturbed: %+v", m.Status())
	}
}
