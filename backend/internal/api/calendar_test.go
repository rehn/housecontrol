package api

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"housecontrol/internal/calendar"
)

type fakeCalendar struct {
	lastSources []calendar.Source
	lastDays    int
	lastFrom    time.Time
	testErr     error
	testedURL   string
}

func (f *fakeCalendar) Upcoming(_ context.Context, src []calendar.Source, from time.Time, days int) calendar.Result {
	f.lastSources, f.lastDays, f.lastFrom = src, days, from
	return calendar.Result{Days: []calendar.Day{{Date: "2026-10-28", Events: []calendar.DayEvent{
		{Title: "Träning", CalendarID: src[0].ID, Color: src[0].Color},
	}}}}
}

func (f *fakeCalendar) Test(_ context.Context, url string) (int, error) {
	f.testedURL = url
	return 12, f.testErr
}

const googleURL = "https://calendar.google.com/calendar/ical/abc%40group.calendar.google.com/private-secret123/basic.ics"

func TestCalendarNotConfigured(t *testing.T) {
	e := newEnv(t)
	code, v := do(t, "GET", e.srv.URL+"/api/calendar", "")
	if code != 404 || v["code"] != "calendar_not_configured" {
		t.Fatalf("%d %v", code, v)
	}
	if _, v := do(t, "GET", e.srv.URL+"/api/settings/calendars", ""); len(v["calendars"].([]any)) != 0 {
		t.Fatalf("%v", v)
	}
}

func TestCalendarsSaveHideURLAndFetch(t *testing.T) {
	e := newEnv(t)
	events, cancel := e.reg.Subscribe()
	defer cancel()

	code, v := do(t, "PUT", e.srv.URL+"/api/settings/calendars", `{"calendars":[
		{"name":" Familjen ","color":"#f5c242","url":"`+googleURL+`"},
		{"name":"Innebandy","color":"#8a63d2","url":"webcal://example.com/team.ics"}]}`)
	if code != 200 {
		t.Fatalf("%d %v", code, v)
	}
	cals := v["calendars"].([]any)
	first := cals[0].(map[string]any)
	if first["name"] != "Familjen" || first["id"] == "" || first["color"] != "#f5c242" {
		t.Fatalf("%v", first)
	}
	if _, leaked := first["url"]; leaked || strings.Contains(first["urlHint"].(string), "secret123") {
		t.Fatalf("the secret calendar address must not be sent back: %v", first)
	}
	if !strings.HasPrefix(first["urlHint"].(string), "calendar.google.com") {
		t.Fatalf("hint: %v", first["urlHint"])
	}
	select {
	case ev := <-events:
		if ev.Type != "calendar" {
			t.Fatalf("%+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("calendar cards were not told to reload")
	}

	code, v = do(t, "GET", e.srv.URL+"/api/calendar", "")
	if code != 200 || len(v["days"].([]any)) != 1 {
		t.Fatalf("%d %v", code, v)
	}
	src := e.calendar.lastSources
	if len(src) != 2 || src[0].URL != googleURL || src[1].Color != "#8a63d2" || e.calendar.lastDays != 7 {
		t.Fatalf("sources: %+v days %d", src, e.calendar.lastDays)
	}
}

func TestCalendarsKeepURLWhenOmitted(t *testing.T) {
	e := newEnv(t)
	_, v := do(t, "PUT", e.srv.URL+"/api/settings/calendars",
		`{"calendars":[{"name":"Familjen","color":"#f5c242","url":"`+googleURL+`"}]}`)
	id := v["calendars"].([]any)[0].(map[string]any)["id"].(string)

	// rename + recolour without sending the url again
	code, v := do(t, "PUT", e.srv.URL+"/api/settings/calendars",
		`{"calendars":[{"id":"`+id+`","name":"Hemma","color":"#4fa8e8"}]}`)
	if code != 200 {
		t.Fatalf("%d %v", code, v)
	}
	saved, _ := e.store.LoadCalendars()
	if len(saved) != 1 || saved[0].URL != googleURL || saved[0].Name != "Hemma" || saved[0].Color != "#4fa8e8" {
		t.Fatalf("%+v", saved)
	}

	// removing all calendars
	do(t, "PUT", e.srv.URL+"/api/settings/calendars", `{"calendars":[]}`)
	if saved, _ := e.store.LoadCalendars(); len(saved) != 0 {
		t.Fatalf("%+v", saved)
	}
}

func TestCalendarsValidation(t *testing.T) {
	e := newEnv(t)
	long := strings.Repeat("x", 41)
	for name, body := range map[string]string{
		"no url for new": `{"calendars":[{"name":"A","color":"#f5c242"}]}`,
		"bad scheme":     `{"calendars":[{"name":"A","color":"#f5c242","url":"ftp://x/y.ics"}]}`,
		"no host":        `{"calendars":[{"name":"A","color":"#f5c242","url":"https:///y.ics"}]}`,
		"empty name":     `{"calendars":[{"name":"  ","color":"#f5c242","url":"https://x/y.ics"}]}`,
		"long name":      `{"calendars":[{"name":"` + long + `","color":"#f5c242","url":"https://x/y.ics"}]}`,
		"bad color":      `{"calendars":[{"name":"A","color":"red","url":"https://x/y.ics"}]}`,
		"unknown id":     `{"calendars":[{"id":"nope","name":"A","color":"#f5c242"}]}`,
		"not json":       `nope`,
	} {
		if code, _ := do(t, "PUT", e.srv.URL+"/api/settings/calendars", body); code != 400 {
			t.Errorf("%s: %d", name, code)
		}
	}
	var many []string
	for i := 0; i < 11; i++ {
		many = append(many, `{"name":"A","color":"#f5c242","url":"https://x/y.ics"}`)
	}
	if code, _ := do(t, "PUT", e.srv.URL+"/api/settings/calendars", `{"calendars":[`+strings.Join(many, ",")+`]}`); code != 400 {
		t.Errorf("more than 10 calendars: %d", code)
	}
}

func TestCalendarTestLink(t *testing.T) {
	e := newEnv(t)
	code, v := do(t, "POST", e.srv.URL+"/api/settings/calendars/test", `{"url":"`+googleURL+`"}`)
	if code != 200 || v["ok"] != true || v["events"] != float64(12) || e.calendar.testedURL != googleURL {
		t.Fatalf("%d %v", code, v)
	}
	e.calendar.testErr = errors.New("not an iCal calendar (check the link)")
	code, v = do(t, "POST", e.srv.URL+"/api/settings/calendars/test", `{"url":"https://x/y.ics"}`)
	if code != 200 || v["ok"] != false || v["error"] != "not an iCal calendar (check the link)" {
		t.Fatalf("%d %v", code, v)
	}
	if code, _ := do(t, "POST", e.srv.URL+"/api/settings/calendars/test", `{"url":"file:///etc/passwd"}`); code != 400 {
		t.Fatalf("only http(s)/webcal links may be fetched: %d", code)
	}
}

func TestCalendarPeriod(t *testing.T) {
	e := newEnv(t)
	do(t, "PUT", e.srv.URL+"/api/settings/calendars", `{"calendars":[{"name":"A","color":"#f5c242","url":"https://x/y.ics"}]}`)

	if code, v := do(t, "GET", e.srv.URL+"/api/calendar", ""); code != 200 || !e.calendar.lastFrom.IsZero() {
		t.Fatalf("no from = today: %d %v %v", code, v, e.calendar.lastFrom)
	}
	if code, v := do(t, "GET", e.srv.URL+"/api/calendar?from=2026-11-04", ""); code != 200 ||
		e.calendar.lastFrom.Format("2006-01-02") != "2026-11-04" || e.calendar.lastDays != 7 {
		t.Fatalf("%d %v %v", code, v, e.calendar.lastFrom)
	}
	for _, bad := range []string{"nope", "2026-13-01", "1969-12-31", "2201-01-01"} {
		if code, _ := do(t, "GET", e.srv.URL+"/api/calendar?from="+bad, ""); code != 400 {
			t.Errorf("from=%s: %d", bad, code)
		}
	}
}
