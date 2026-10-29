// Package mqttclient manages the broker connection and bridges zigbee2mqtt
// topics into the device registry.
//
// The Manager never takes the app down: with no broker configured it simply
// reports "unconfigured", and failed connections are retried in the
// background with the error exposed through Status() for the UI. The broker
// can be swapped at runtime with Apply().
package mqttclient

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"housecontrol/internal/config"
	"housecontrol/internal/devices"
)

const (
	StateUnconfigured = "unconfigured"
	StateConnecting   = "connecting"
	StateConnected    = "connected"
	StateError        = "error"

	connectTimeout = 10 * time.Second
)

var ErrNotConnected = errors.New("not connected to the MQTT broker")

type Manager struct {
	reg *devices.Registry
	log *slog.Logger

	mu     sync.Mutex
	gen    uint64 // bumped on every Apply; stale connections are ignored
	cur    *conn
	cfg    config.MQTT
	source string // where cfg came from: "ui" or "config"
	status devices.MQTTStatus
}

type conn struct {
	m      *Manager
	gen    uint64
	cfg    config.MQTT
	client mqtt.Client
	ctx    context.Context
	cancel context.CancelFunc
}

func NewManager(reg *devices.Registry, log *slog.Logger) *Manager {
	return &Manager{
		reg:    reg,
		log:    log,
		status: devices.MQTTStatus{State: StateUnconfigured, Since: time.Now()},
	}
}

// Apply switches to new settings: the old connection (if any) is closed,
// devices are forgotten and a new connection is started in the background.
// An empty URL leaves MQTT unconfigured.
func (m *Manager) Apply(cfg config.MQTT, source string) {
	m.mu.Lock()
	old := m.cur
	m.gen++
	m.cfg = cfg
	m.source = source
	m.cur = nil
	if cfg.URL != "" {
		ctx, cancel := context.WithCancel(context.Background())
		c := &conn{m: m, gen: m.gen, cfg: cfg, ctx: ctx, cancel: cancel}
		c.client = mqtt.NewClient(c.options())
		m.cur = c
		go c.run()
	}
	gen := m.gen
	m.mu.Unlock()

	if old != nil {
		old.cancel()
		go old.client.Disconnect(250)
	}
	m.reg.Reset()
	if cfg.URL == "" {
		m.setStatus(gen, StateUnconfigured, "", nil)
		m.log.Warn("mqtt not configured; set it in the UI (Settings) or via MQTT_URL")
	}
}

// Close disconnects from the broker.
func (m *Manager) Close() {
	m.mu.Lock()
	c := m.cur
	m.cur = nil
	m.gen++
	m.mu.Unlock()
	if c != nil {
		c.cancel()
		c.client.Disconnect(250)
	}
}

// Config returns the active settings and where they came from.
func (m *Manager) Config() (config.MQTT, string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cfg, m.source
}

func (m *Manager) Status() devices.MQTTStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

func (m *Manager) Connected() bool { return m.Status().State == StateConnected }

// Set publishes a command to <base>/<name>/set.
func (m *Manager) Set(name string, payload map[string]any) error {
	c := m.current()
	if c == nil || !c.client.IsConnectionOpen() {
		return ErrNotConnected
	}
	return c.publish(c.cfg.BaseTopic+"/"+name+"/set", payload)
}

// Test tries a one-off connection with cfg without touching the live one.
func (m *Manager) Test(cfg config.MQTT) error {
	if cfg.URL == "" {
		return errors.New("broker URL is empty")
	}
	opts := baseOptions(cfg).
		SetClientID("housecontrol-test-" + randomSuffix()). // never kick the live client
		SetAutoReconnect(false).
		SetConnectRetry(false)
	cl := mqtt.NewClient(opts)
	t := cl.Connect()
	if !t.WaitTimeout(connectTimeout) {
		return fmt.Errorf("timed out connecting to %s", cfg.URL)
	}
	if err := t.Error(); err != nil {
		return err
	}
	cl.Disconnect(100)
	return nil
}

func (m *Manager) current() *conn {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cur
}

// setStatus records and broadcasts a status change, unless it comes from a
// connection that has since been replaced.
func (m *Manager) setStatus(gen uint64, state, broker string, err error) {
	m.mu.Lock()
	if gen != m.gen {
		m.mu.Unlock()
		return
	}
	st := devices.MQTTStatus{State: state, Broker: broker, Since: time.Now()}
	if err != nil {
		st.Error = err.Error()
	}
	if st.State == m.status.State && st.Error == m.status.Error && st.Broker == m.status.Broker {
		m.mu.Unlock()
		return
	}
	m.status = st
	m.mu.Unlock()
	m.reg.Broadcast(devices.Event{Type: "status", MQTT: &st})
}

// --- one connection ----------------------------------------------------------

func baseOptions(cfg config.MQTT) *mqtt.ClientOptions {
	return mqtt.NewClientOptions().
		AddBroker(cfg.URL).
		SetUsername(cfg.Username).
		SetPassword(cfg.Password).
		SetConnectTimeout(connectTimeout).
		SetOrderMatters(false)
}

func (c *conn) options() *mqtt.ClientOptions {
	id := c.cfg.ClientID
	if id == "" {
		id = "housecontrol-" + randomSuffix()
	}
	return baseOptions(c.cfg).
		SetClientID(id).
		// The initial connect is retried by run() so errors reach the UI;
		// after that paho reconnects by itself.
		SetConnectRetry(false).
		SetAutoReconnect(true).
		SetMaxReconnectInterval(30 * time.Second).
		SetOnConnectHandler(c.onConnect).
		SetConnectionLostHandler(func(_ mqtt.Client, err error) {
			c.m.log.Warn("mqtt connection lost", "err", err)
			c.m.setStatus(c.gen, StateError, c.cfg.URL, fmt.Errorf("connection lost: %w", err))
		}).
		SetReconnectingHandler(func(mqtt.Client, *mqtt.ClientOptions) {
			c.m.setStatus(c.gen, StateConnecting, c.cfg.URL, nil)
		})
}

// run keeps trying the initial connection with backoff until it succeeds or
// the connection is replaced.
func (c *conn) run() {
	backoff := 2 * time.Second
	for {
		if c.ctx.Err() != nil {
			return
		}
		c.m.setStatus(c.gen, StateConnecting, c.cfg.URL, nil)
		c.m.log.Info("connecting to mqtt", "url", c.cfg.URL, "baseTopic", c.cfg.BaseTopic)

		t := c.client.Connect()
		var err error
		if !t.WaitTimeout(connectTimeout + 2*time.Second) {
			err = fmt.Errorf("timed out connecting to %s", c.cfg.URL)
		} else {
			err = t.Error()
		}
		if err == nil {
			if c.ctx.Err() != nil { // replaced while connecting
				c.client.Disconnect(100)
			}
			return
		}

		c.m.log.Warn("mqtt connect failed", "url", c.cfg.URL, "err", err, "retryIn", backoff)
		c.m.setStatus(c.gen, StateError, c.cfg.URL, err)
		select {
		case <-c.ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, 30*time.Second)
	}
}

func (c *conn) onConnect(cl mqtt.Client) {
	if c.ctx.Err() != nil {
		return
	}
	topic := c.cfg.BaseTopic + "/#"
	t := cl.Subscribe(topic, 0, c.onMessage)
	if t.WaitTimeout(connectTimeout) && t.Error() != nil {
		c.m.log.Error("subscribe failed", "topic", topic, "err", t.Error())
		c.m.setStatus(c.gen, StateError, c.cfg.URL, fmt.Errorf("subscribe %s: %w", topic, t.Error()))
		return
	}
	c.m.log.Info("mqtt connected", "url", c.cfg.URL)
	c.m.setStatus(c.gen, StateConnected, c.cfg.URL, nil)
}

func (c *conn) publish(topic string, payload map[string]any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	t := c.client.Publish(topic, 0, false, b)
	if !t.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("publish to %s timed out", topic)
	}
	return t.Error()
}

func (c *conn) onMessage(_ mqtt.Client, msg mqtt.Message) {
	if c.ctx.Err() != nil {
		return // message from a replaced connection
	}
	reg := c.m.reg
	rest := strings.TrimPrefix(msg.Topic(), c.cfg.BaseTopic+"/")
	payload := msg.Payload()

	switch {
	case rest == "bridge/devices":
		list, err := devices.ParseBridgeDevices(payload)
		if err != nil {
			c.m.log.Error("parse bridge/devices", "err", err)
			return
		}
		reg.SetDevices(list)
		c.m.log.Info("device list updated", "count", len(list))
		go c.requestMissingState()

	case strings.HasPrefix(rest, "bridge/"):
		// other bridge topics (state, info, logging, ...) ignored for now

	case strings.HasSuffix(rest, "/availability"):
		reg.SetAvailability(strings.TrimSuffix(rest, "/availability"), parseAvailability(payload))

	case strings.HasSuffix(rest, "/set"), strings.HasSuffix(rest, "/get"),
		strings.Contains(rest, "/set/"):
		// commands echoing back

	default:
		var st map[string]any
		if err := json.Unmarshal(payload, &st); err != nil {
			return // not a JSON state message
		}
		reg.MergeState(rest, st)
	}
}

// requestMissingState asks lights/switches without a known state to report
// it, so the UI is correct even if zigbee2mqtt doesn't retain state.
func (c *conn) requestMissingState() {
	for _, d := range c.m.reg.List() {
		if d.Kind == devices.KindOther || len(d.State) > 0 || !d.Capabilities.OnOff {
			continue
		}
		p := map[string]any{d.Capabilities.StateProperty: ""}
		if d.Capabilities.Brightness {
			p["brightness"] = ""
		}
		if err := c.publish(c.cfg.BaseTopic+"/"+d.Name+"/get", p); err != nil {
			c.m.log.Debug("state request failed", "device", d.Name, "err", err)
		}
	}
}

func parseAvailability(b []byte) string {
	var v struct {
		State string `json:"state"`
	}
	if json.Unmarshal(b, &v) == nil && v.State != "" {
		return v.State
	}
	return strings.TrimSpace(string(b)) // legacy plain "online"/"offline"
}

func randomSuffix() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
