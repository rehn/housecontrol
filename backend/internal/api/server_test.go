package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"housecontrol/internal/config"
	"housecontrol/internal/devices"
	"housecontrol/internal/mqttclient"
	"housecontrol/internal/settings"
)

type fakeMQTT struct {
	cfg       config.MQTT
	source    string
	testErr   error
	connected bool

	mu   sync.Mutex
	sent []string // "<name> <json payload>"
}

func (f *fakeMQTT) Set(name string, payload map[string]any) error {
	if !f.connected {
		return mqttclient.ErrNotConnected
	}
	b, _ := json.Marshal(payload)
	f.mu.Lock()
	f.sent = append(f.sent, name+" "+string(b))
	f.mu.Unlock()
	return nil
}
func (f *fakeMQTT) Status() devices.MQTTStatus    { return devices.MQTTStatus{State: "unconfigured"} }
func (f *fakeMQTT) Config() (config.MQTT, string) { return f.cfg, f.source }
func (f *fakeMQTT) Apply(c config.MQTT, s string) { f.cfg, f.source = c, s }
func (f *fakeMQTT) Test(config.MQTT) error        { return f.testErr }

func (f *fakeMQTT) sentCommands() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.sent...)
}

type testEnv struct {
	srv      *httptest.Server
	mq       *fakeMQTT
	store    *settings.Store
	reg      *devices.Registry
	weather  *fakeWeather
	calendar *fakeCalendar
}

func newEnv(t *testing.T) *testEnv { return newEnvWith(t, nil) }

// newEnvWith lets a test change the server options (e.g. switch login on).
func newEnvWith(t *testing.T, opt func(*Options)) *testEnv {
	dir := t.TempDir()
	static := filepath.Join(dir, "web")
	os.MkdirAll(filepath.Join(static, "settings"), 0o755)
	os.WriteFile(filepath.Join(static, "index.html"), []byte("home"), 0o644)
	os.WriteFile(filepath.Join(static, "settings", "index.html"), []byte("settings"), 0o644)
	os.WriteFile(filepath.Join(static, "404.html"), []byte("nope"), 0o644)

	e := &testEnv{
		mq:       &fakeMQTT{cfg: config.MQTT{BaseTopic: "zigbee2mqtt"}, source: "config"},
		store:    settings.NewStore(filepath.Join(dir, "data")),
		reg:      devices.NewRegistry(),
		weather:  &fakeWeather{},
		calendar: &fakeCalendar{},
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	o := Options{StaticDir: static, Weather: e.weather, Calendar: e.calendar}
	if opt != nil {
		opt(&o)
	}
	e.srv = httptest.NewServer(New(e.reg, e.mq, e.store, log, o).Handler())
	t.Cleanup(e.srv.Close)
	return e
}

// newTest is kept for the older tests.
func newTest(t *testing.T) (*httptest.Server, *fakeMQTT, *settings.Store, string) {
	e := newEnv(t)
	return e.srv, e.mq, e.store, ""
}

// doList GETs an endpoint that returns a JSON array.
func doList(t *testing.T, url string) (int, []map[string]any) {
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var v []map[string]any
	_ = json.NewDecoder(res.Body).Decode(&v)
	return res.StatusCode, v
}

func do(t *testing.T, method, url, body string) (int, map[string]any) {
	req, _ := http.NewRequest(method, url, strings.NewReader(body))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var m map[string]any
	_ = json.NewDecoder(res.Body).Decode(&m)
	return res.StatusCode, m
}

func TestMQTTSettingsFlow(t *testing.T) {
	srv, mq, store, _ := newTest(t)

	code, v := do(t, "PUT", srv.URL+"/api/settings/mqtt",
		`{"url":"192.168.1.201","username":"ha","password":"secret","baseTopic":""}`)
	if code != 200 || v["url"] != "tcp://192.168.1.201:1883" || v["hasPassword"] != true || v["source"] != "ui" {
		t.Fatalf("put: %d %v", code, v)
	}
	if _, leaked := v["password"]; leaked {
		t.Fatal("password must never be returned")
	}
	if mq.cfg.BaseTopic != "zigbee2mqtt" || mq.cfg.Password != "secret" {
		t.Fatalf("applied: %+v", mq.cfg)
	}
	saved, _ := store.LoadMQTT()
	if saved == nil || saved.URL != "tcp://192.168.1.201:1883" {
		t.Fatalf("saved: %+v", saved)
	}

	// password omitted = keep
	do(t, "PUT", srv.URL+"/api/settings/mqtt", `{"url":"tcp://192.168.1.201:1883","username":"ha"}`)
	if mq.cfg.Password != "secret" {
		t.Fatal("password should be kept when omitted")
	}
	// explicit empty = remove
	do(t, "PUT", srv.URL+"/api/settings/mqtt", `{"url":"tcp://192.168.1.201:1883","password":""}`)
	if mq.cfg.Password != "" {
		t.Fatal("password should be cleared")
	}

	if code, _ := do(t, "PUT", srv.URL+"/api/settings/mqtt", `{"url":"http://x"}`); code != 400 {
		t.Fatalf("bad scheme: %d", code)
	}

	mq.testErr = errors.New("connection refused")
	code, v = do(t, "POST", srv.URL+"/api/settings/mqtt/test", `{"url":"192.168.1.201"}`)
	if code != 200 || v["ok"] != false || v["error"] != "connection refused" {
		t.Fatalf("test: %d %v", code, v)
	}
}

func TestAppWorksWithoutMQTT(t *testing.T) {
	srv, _, _, _ := newTest(t)
	for _, p := range []string{"/healthz", "/readyz", "/api/status", "/api/devices"} {
		if code, _ := do(t, "GET", srv.URL+p, ""); code != 200 {
			t.Errorf("%s: %d", p, code)
		}
	}
}

func TestStatic(t *testing.T) {
	srv, _, _, _ := newTest(t)
	get := func(p string) (int, string) {
		res, err := http.Get(srv.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(b)
	}
	if c, b := get("/"); c != 200 || b != "home" {
		t.Errorf("/: %d %q", c, b)
	}
	if c, b := get("/settings/"); c != 200 || b != "settings" {
		t.Errorf("/settings/: %d %q", c, b)
	}
	if c, b := get("/settings"); c != 200 || b != "settings" { // client follows redirect
		t.Errorf("/settings: %d %q", c, b)
	}
	if c, b := get("/nope"); c != 404 || b != "nope" {
		t.Errorf("/nope: %d %q", c, b)
	}
	if c, _ := get("/api/unknown"); c != 404 {
		t.Errorf("/api/unknown: %d", c)
	}
}
