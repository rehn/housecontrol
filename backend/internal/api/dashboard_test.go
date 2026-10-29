package api

import (
	"slices"
	"strings"
	"testing"
	"time"

	"housecontrol/internal/devices"
)

const validDashboard = `{"version":1,"columns":[
	{"id":"c1","cards":[{"id":"clock","type":"clock"}]},
	{"id":"c2","cards":[{"id":"g1","type":"group","title":"Köket","deviceIds":["0x01","0x02"]}]}
]}`

func TestDashboardIsNullUntilSaved(t *testing.T) {
	e := newEnv(t)
	code, v := do(t, "GET", e.srv.URL+"/api/dashboard", "")
	if code != 200 {
		t.Fatalf("code %d", code)
	}
	if d, ok := v["dashboard"]; !ok || d != nil {
		t.Fatalf("want {\"dashboard\":null}, got %v", v)
	}
}

func TestDashboardSaveLoadAndBroadcast(t *testing.T) {
	e := newEnv(t)
	events, cancel := e.reg.Subscribe()
	defer cancel()

	code, v := do(t, "PUT", e.srv.URL+"/api/dashboard", validDashboard)
	if code != 200 {
		t.Fatalf("put: %d %v", code, v)
	}

	_, v = do(t, "GET", e.srv.URL+"/api/dashboard", "")
	d, _ := v["dashboard"].(map[string]any)
	cols, _ := d["columns"].([]any)
	if len(cols) != 2 {
		t.Fatalf("reloaded dashboard: %v", v)
	}

	saved, err := e.store.LoadDashboard()
	if err != nil || saved == nil || saved.Columns[1].Cards[0].Title != "Köket" {
		t.Fatalf("persisted: %+v %v", saved, err)
	}

	select {
	case ev := <-events:
		if ev.Type != "dashboard" || ev.Dashboard == nil {
			t.Fatalf("event: %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("other clients were not notified")
	}
}

func TestDashboardRejectsInvalid(t *testing.T) {
	e := newEnv(t)
	for _, body := range []string{
		`not json`,
		`{"version":1,"columns":[]}`,
		`{"version":1,"columns":[{"id":"c","cards":[{"id":"a","type":"camera"}]}]}`,
	} {
		code, v := do(t, "PUT", e.srv.URL+"/api/dashboard", body)
		if code != 400 || v["error"] == nil {
			t.Errorf("%s: %d %v", body, code, v)
		}
	}
	if d, _ := e.store.LoadDashboard(); d != nil {
		t.Fatal("invalid dashboard must not be saved")
	}
}

func groupEnv(t *testing.T) *testEnv {
	e := newEnv(t)
	e.reg.SetDevices([]devices.Device{
		{ID: "0x01", Name: "kitchen/1", Kind: devices.KindLight},
		{ID: "0x02", Name: "kitchen/2", Kind: devices.KindLight},
	})
	e.mq.connected = true
	return e
}

func TestSetManySendsToEveryDevice(t *testing.T) {
	e := groupEnv(t)
	code, v := do(t, "POST", e.srv.URL+"/api/devices/set-many",
		`{"ids":["0x01","0x02"],"payload":{"state":"on"}}`)
	if code != 202 {
		t.Fatalf("code %d %v", code, v)
	}
	got := e.mq.sentCommands()
	slices.Sort(got)
	want := []string{`kitchen/1 {"state":"ON"}`, `kitchen/2 {"state":"ON"}`}
	if !slices.Equal(got, want) {
		t.Fatalf("sent %v, want %v", got, want)
	}
}

func TestSetManyValidation(t *testing.T) {
	e := groupEnv(t)
	cases := map[string]string{
		"unknown device":  `{"ids":["0x01","0x99"],"payload":{"state":"ON"}}`,
		"no ids":          `{"ids":[],"payload":{"state":"ON"}}`,
		"bad payload":     `{"ids":["0x01"],"payload":{"hack":1}}`,
		"bad state":       `{"ids":["0x01"],"payload":{"state":"MAYBE"}}`,
		"empty payload":   `{"ids":["0x01"],"payload":{}}`,
		"not json at all": `nope`,
	}
	for name, body := range cases {
		code, v := do(t, "POST", e.srv.URL+"/api/devices/set-many", body)
		if code != 400 {
			t.Errorf("%s: code %d %v", name, code, v)
		}
	}
	if n := len(e.mq.sentCommands()); n != 0 {
		t.Fatalf("nothing should be sent for invalid requests, sent %d", n)
	}
}

func TestSetManyWhenDisconnected(t *testing.T) {
	e := groupEnv(t)
	e.mq.connected = false
	code, v := do(t, "POST", e.srv.URL+"/api/devices/set-many", `{"ids":["0x01"],"payload":{"state":"ON"}}`)
	if code != 503 || !strings.Contains(v["error"].(string), "not connected") {
		t.Fatalf("code %d %v", code, v)
	}
}
