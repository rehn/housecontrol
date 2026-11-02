package api

import (
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
	buttons := r["buttons"].([]any)
	if len(buttons) != 2 || buttons[0].(map[string]any)["id"] != "on" ||
		len(buttons[0].(map[string]any)["events"].([]any)) != 2 {
		t.Fatalf("buttons %v", buttons)
	}
	if b := v["bindings"].([]any); len(b) != 0 {
		t.Fatalf("bindings %v", b)
	}
}

func TestRemotesSave(t *testing.T) {
	e := newEnv(t)
	withRemotes(t, e)
	code, v := do(t, "PUT", e.srv.URL+"/api/settings/remotes", `{"bindings":[
	  {"remote":"0x10","button":"on","event":"press","action":"toggle","targets":["group:vardagsrum","device:0x01"]},
	  {"remote":"0x10","button":"on","event":"hold","action":"dim_up","targets":["group:vardagsrum"]}]}`)
	if code != 200 || len(v["bindings"].([]any)) != 2 {
		t.Fatalf("%d %v", code, v)
	}
	saved, _ := e.store.LoadRemotes()
	if len(saved) != 2 || saved[1].Action != remote.ActionDimUp {
		t.Fatalf("%+v", saved)
	}
	code, v = do(t, "PUT", e.srv.URL+"/api/settings/remotes", `{"bindings":[
	  {"remote":"0x10","button":"on","event":"press","action":"dim_up","targets":["device:0x01"]}]}`)
	if code != 400 {
		t.Fatalf("dimming on a short press: %d %v", code, v)
	}
	if saved, _ := e.store.LoadRemotes(); len(saved) != 2 {
		t.Fatal("a refused change must not be saved")
	}
}

func TestRemotesNeedLogin(t *testing.T) {
	e := newAuthEnv(t)
	for _, m := range []string{"GET", "PUT"} {
		if code, _ := do(t, m, e.srv.URL+"/api/settings/remotes", `{"bindings":[]}`); code != 401 {
			t.Errorf("%s: %d", m, code)
		}
	}
}

func TestBackupKeepsRemotes(t *testing.T) {
	from := newEnv(t)
	from.store.SaveRemotes([]remote.Binding{{Remote: "0x10", Button: "on", Event: "press", Action: "on", Targets: []string{"device:0x01"}}})
	code, file, _ := exportWith(t, from, `{"protect":false}`)
	if code != 200 {
		t.Fatal(code)
	}
	to := newEnv(t)
	if code, v := restoreBackup(t, to, "", file); code != 200 {
		t.Fatalf("%d %v", code, v)
	}
	if r, _ := to.store.LoadRemotes(); len(r) != 1 || r[0].Button != "on" {
		t.Fatalf("%+v", r)
	}
}
