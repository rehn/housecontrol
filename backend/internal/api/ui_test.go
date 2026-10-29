package api

import (
	"strings"
	"testing"
	"time"

	"housecontrol/internal/devices"
)

func TestUISettingsDefaultToEnglish(t *testing.T) {
	e := newEnv(t)
	code, v := do(t, "GET", e.srv.URL+"/api/settings/ui", "")
	if code != 200 || v["language"] != "en" {
		t.Fatalf("got %d %v", code, v)
	}
}

func TestUISettingsSaveAndBroadcast(t *testing.T) {
	e := newEnv(t)
	events, cancel := e.reg.Subscribe()
	defer cancel()

	code, v := do(t, "PUT", e.srv.URL+"/api/settings/ui", `{"language":"sv"}`)
	if code != 200 || v["language"] != "sv" {
		t.Fatalf("put: %d %v", code, v)
	}
	if _, v = do(t, "GET", e.srv.URL+"/api/settings/ui", ""); v["language"] != "sv" {
		t.Fatalf("not persisted: %v", v)
	}
	select {
	case ev := <-events:
		if ev.Type != "ui" || ev.UI == nil || ev.UI.Language != "sv" {
			t.Fatalf("event: %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("other clients were not notified")
	}
}

func TestUISettingsRejectInvalidLanguage(t *testing.T) {
	e := newEnv(t)
	for _, body := range []string{`{"language":""}`, `{"language":"svenska!"}`, `{"language":"../../x"}`, `nope`} {
		if code, _ := do(t, "PUT", e.srv.URL+"/api/settings/ui", body); code != 400 {
			t.Errorf("%s: %d", body, code)
		}
	}
}

func TestErrorsCarryCodesForTranslation(t *testing.T) {
	e := groupEnv(t)
	e.mq.connected = false

	code, v := do(t, "POST", e.srv.URL+"/api/devices/0x01/set", `{"state":"ON"}`)
	if code != 503 || v["code"] != "mqtt_not_connected" {
		t.Fatalf("set while disconnected: %d %v", code, v)
	}
	code, v = do(t, "POST", e.srv.URL+"/api/devices/set-many", `{"ids":["0x01"],"payload":{"state":"ON"}}`)
	if code != 503 || v["code"] != "mqtt_not_connected" {
		t.Fatalf("set-many while disconnected: %d %v", code, v)
	}
	code, v = do(t, "POST", e.srv.URL+"/api/devices/0x99/set", `{"state":"ON"}`)
	if code != 404 || v["code"] != "device_not_found" {
		t.Fatalf("unknown device: %d %v", code, v)
	}
}

var _ = devices.Event{}

func TestUITitle(t *testing.T) {
	e := newEnv(t)

	// title is empty until set (the frontend shows "housecontrol" then)
	if _, v := do(t, "GET", e.srv.URL+"/api/settings/ui", ""); v["title"] != "" {
		t.Fatalf("default title: %v", v)
	}

	code, v := do(t, "PUT", e.srv.URL+"/api/settings/ui", `{"title":"  Hemma på Ekvägen  "}`)
	if code != 200 || v["title"] != "Hemma på Ekvägen" || v["language"] != "en" {
		t.Fatalf("set title: %d %v", code, v)
	}

	// changing only the language keeps the title, and vice versa
	do(t, "PUT", e.srv.URL+"/api/settings/ui", `{"language":"sv"}`)
	if _, v = do(t, "GET", e.srv.URL+"/api/settings/ui", ""); v["title"] != "Hemma på Ekvägen" || v["language"] != "sv" {
		t.Fatalf("after language change: %v", v)
	}
	do(t, "PUT", e.srv.URL+"/api/settings/ui", `{"title":"Stugan"}`)
	if _, v = do(t, "GET", e.srv.URL+"/api/settings/ui", ""); v["title"] != "Stugan" || v["language"] != "sv" {
		t.Fatalf("after title change: %v", v)
	}

	// empty resets to the default
	do(t, "PUT", e.srv.URL+"/api/settings/ui", `{"title":"   "}`)
	if _, v = do(t, "GET", e.srv.URL+"/api/settings/ui", ""); v["title"] != "" {
		t.Fatalf("reset title: %v", v)
	}
}

func TestUITitleTooLong(t *testing.T) {
	e := newEnv(t)
	long := `{"title":"` + strings.Repeat("å", 61) + `"}`
	if code, _ := do(t, "PUT", e.srv.URL+"/api/settings/ui", long); code != 400 {
		t.Fatalf("61 characters should be rejected, got %d", code)
	}
	ok := `{"title":"` + strings.Repeat("å", 60) + `"}`
	if code, _ := do(t, "PUT", e.srv.URL+"/api/settings/ui", ok); code != 200 {
		t.Fatalf("60 characters should be accepted, got %d", code)
	}
}

func TestUITitleIsBroadcast(t *testing.T) {
	e := newEnv(t)
	events, cancel := e.reg.Subscribe()
	defer cancel()
	do(t, "PUT", e.srv.URL+"/api/settings/ui", `{"title":"Stugan"}`)
	select {
	case ev := <-events:
		if ev.Type != "ui" || ev.UI == nil || ev.UI.Title != "Stugan" || ev.UI.Language != "en" {
			t.Fatalf("event: %+v", ev.UI)
		}
	case <-time.After(time.Second):
		t.Fatal("not broadcast")
	}
}
