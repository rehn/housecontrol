package api

import (
	"strings"
	"testing"
	"time"

	"housecontrol/internal/settings"
)

func TestSchedulesEmpty(t *testing.T) {
	e := newEnv(t)
	code, v := do(t, "GET", e.srv.URL+"/api/settings/schedules", "")
	if code != 200 || len(v["schedules"].([]any)) != 0 {
		t.Fatalf("%d %v", code, v)
	}
	if place := v["place"].(map[string]any); place["known"] != false {
		t.Fatalf("no weather place saved = sun times unknown: %v", place)
	}
}

func TestSchedulesSaveAndShowNextRun(t *testing.T) {
	e := newEnv(t)
	e.store.SaveWeather(settings.Weather{Provider: "smhi", Latitude: 59.64, Longitude: 17.08, Location: "Enköping"})

	code, v := do(t, "PUT", e.srv.URL+"/api/settings/schedules", `{"schedules":[
		{"id":"s1","name":" Ute ","enabled":true,"deviceIds":["0x01"],"days":[1,2,3,4,5],
		 "on":{"kind":"sunset","offset":-30,"notBefore":"16:00"},"off":{"kind":"time","time":"23:30"},"brightness":40},
		{"id":"s2","name":"Kaffe","enabled":false,"deviceIds":["0x02"],"on":{"kind":"time","time":"06:30"}}]}`)
	if code != 200 {
		t.Fatalf("%d %v", code, v)
	}
	list := v["schedules"].([]any)
	first := list[0].(map[string]any)
	if first["name"] != "Ute" {
		t.Fatalf("%v", first)
	}
	next := v["next"].(map[string]any)
	n1, ok := next["s1"].(map[string]any)
	if !ok || (n1["action"] != "on" && n1["action"] != "off") {
		t.Fatalf("next run for an enabled schedule: %v", next)
	}
	if at, err := time.Parse(time.RFC3339, n1["at"].(string)); err != nil || !at.After(time.Now()) {
		t.Fatalf("next at %v %v", n1["at"], err)
	}
	if _, has := next["s2"]; has {
		t.Fatal("a disabled schedule has no next run")
	}
	place := v["place"].(map[string]any)
	if place["known"] != true || place["name"] != "Enköping" || place["sunrise"] == "" || place["sunset"] == "" {
		t.Fatalf("%v", place)
	}

	saved, _ := e.store.LoadSchedules()
	if len(saved) != 2 || saved[0].On.Offset != -30 || saved[0].Brightness != 40 {
		t.Fatalf("%+v", saved)
	}
}

func TestSchedulesValidation(t *testing.T) {
	e := newEnv(t)
	many := make([]string, 51)
	for i := range many {
		many[i] = `{"id":"s` + string(rune('a'+i%26)) + strings.Repeat("x", i/26) + `","name":"x","deviceIds":["a"],"on":{"kind":"time","time":"07:00"}}`
	}
	for name, body := range map[string]string{
		"not json":      `nope`,
		"bad schedule":  `{"schedules":[{"id":"s","name":"x","deviceIds":["a"]}]}`,
		"duplicate ids": `{"schedules":[{"id":"s","name":"x","deviceIds":["a"],"on":{"kind":"time","time":"07:00"}},{"id":"s","name":"y","deviceIds":["a"],"on":{"kind":"time","time":"08:00"}}]}`,
		"too many":      `{"schedules":[` + strings.Join(many, ",") + `]}`,
	} {
		if code, _ := do(t, "PUT", e.srv.URL+"/api/settings/schedules", body); code != 400 {
			t.Errorf("%s: %d", name, code)
		}
	}
}

func TestSchedulesNeedLogin(t *testing.T) {
	e := newAuthEnv(t)
	for _, m := range []string{"GET", "PUT"} {
		if code, _ := do(t, m, e.srv.URL+"/api/settings/schedules", `{"schedules":[]}`); code != 401 {
			t.Errorf("%s: %d", m, code)
		}
	}
}
