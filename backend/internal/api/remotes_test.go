package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"housecontrol/internal/devices"
	"housecontrol/internal/remote"
)

const remotesJSON = `[
 {"ieee_address":"0x01","friendly_name":"Taklampa","type":"Router","supported":true,
  "definition":{"model":"LED","vendor":"IKEA","exposes":[{"type":"light","features":[{"type":"binary","name":"state","property":"state"}]}]}},
 {"ieee_address":"0x10","friendly_name":"Fjärr soffan","type":"EndDevice","supported":true,
  "definition":{"model":"E2201","vendor":"IKEA","description":"RODRET wireless dimmer","exposes":[
   {"type":"enum","name":"action","property":"action","values":["on","off","brightness_move_down","brightness_move_up","brightness_stop"]}]}}
]`

func withRemotes(t *testing.T, e *testEnv) {
	t.Helper()
	ds, err := devices.ParseBridgeDevices([]byte(remotesJSON))
	if err != nil {
		t.Fatal(err)
	}
	e.reg.SetDevices(ds)
}

func TestRemotesList(t *testing.T) {
	e := newEnv(t)
	withRemotes(t, e)
	code, v := do(t, "GET", e.srv.URL+"/api/settings/remotes", "")
	if code != 200 {
		t.Fatalf("%d %v", code, v)
	}
	rs := v["remotes"].([]any)
	if len(rs) != 1 {
		t.Fatalf("only remotes: %v", rs)
	}
	r := rs[0].(map[string]any)
	if r["id"] != "0x10" || r["name"] != "Fjärr soffan" || r["model"] != "E2201" {
		t.Fatalf("%v", r)
	}
	if b := r["buttons"].([]any); len(b) != 2 || b[0].(map[string]any)["id"] != "on" {
		t.Fatalf("buttons %v", b)
	}
	// up/on switches on and dims up while held, down/off the other way
	if d := r["defaults"].([]any); len(d) != 4 || d[1].(map[string]any)["action"] != "dim_up" {
		t.Fatalf("defaults %v", d)
	}
	if c := v["configs"].([]any); len(c) != 0 {
		t.Fatalf("configs %v", c)
	}
}

func TestRemotesSave(t *testing.T) {
	e := newEnv(t)
	withRemotes(t, e)
	code, v := do(t, "PUT", e.srv.URL+"/api/settings/remotes", `{"configs":[{"remote":"0x10","target":"group:vardagsrum","actions":[
	  {"button":"on","event":"press","action":"toggle"},
	  {"button":"on","event":"hold","action":"dim_up"}]}]}`)
	if code != 200 || len(v["configs"].([]any)) != 1 {
		t.Fatalf("%d %v", code, v)
	}
	saved, _ := e.store.LoadRemotes()
	if len(saved) != 1 || saved[0].Target != "group:vardagsrum" || saved[0].Actions[1].Action != remote.ActionDimUp {
		t.Fatalf("%+v", saved)
	}
	code, v = do(t, "PUT", e.srv.URL+"/api/settings/remotes", `{"configs":[{"remote":"0x10","target":"device:0x01","actions":[
	  {"button":"on","event":"press","action":"dim_up"}]}]}`)
	if code != 400 {
		t.Fatalf("dimming on a short press: %d %v", code, v)
	}
	if saved, _ := e.store.LoadRemotes(); saved[0].Target != "group:vardagsrum" {
		t.Fatal("a refused change must not be saved")
	}
}

func TestRemotesFirstFormatIsMovedOver(t *testing.T) {
	e := newEnv(t)
	os.MkdirAll(filepath.Dir(e.store.Path()), 0o700)
	os.WriteFile(e.store.Path(), []byte(`{"remotes":[
	  {"remote":"0x10","button":"on","event":"press","action":"on","targets":["group:g","device:0x01"]},
	  {"remote":"0x10","button":"off","event":"press","action":"off","targets":["group:g"]}]}`), 0o600)
	_, v := do(t, "GET", e.srv.URL+"/api/settings/remotes", "")
	c := v["configs"].([]any)
	if len(c) != 1 || c[0].(map[string]any)["target"] != "group:g" || len(c[0].(map[string]any)["actions"].([]any)) != 2 {
		t.Fatalf("%v", c)
	}
	// saving writes the new format only
	e.store.SaveRemotes([]remote.Config{{Remote: "0x10", Target: "device:0x01", Actions: []remote.Action{}}})
	b, _ := os.ReadFile(e.store.Path())
	if s := string(b); !strings.Contains(s, `"remoteControls"`) || strings.Contains(s, `"remotes"`) {
		t.Fatalf("%s", s)
	}
}

func TestRemotesNeedLogin(t *testing.T) {
	e := newAuthEnv(t)
	for _, m := range []string{"GET", "PUT"} {
		if code, _ := do(t, m, e.srv.URL+"/api/settings/remotes", `{"configs":[]}`); code != 401 {
			t.Errorf("%s: %d", m, code)
		}
	}
}

func TestBackupKeepsRemotes(t *testing.T) {
	from := newEnv(t)
	from.store.SaveRemotes([]remote.Config{{Remote: "0x10", Target: "device:0x01",
		Actions: []remote.Action{{Button: "on", Event: "press", Action: "on"}}}})
	code, file, _ := exportWith(t, from, `{"protect":false}`)
	if code != 200 {
		t.Fatal(code)
	}
	to := newEnv(t)
	if code, v := restoreBackup(t, to, "", file); code != 200 {
		t.Fatalf("%d %v", code, v)
	}
	if r, _ := to.store.LoadRemotes(); len(r) != 1 || r[0].Target != "device:0x01" {
		t.Fatalf("%+v", r)
	}
}
