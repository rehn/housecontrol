package schedule

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

var home = Place{Lat: 59.3293, Lon: 18.0686, Loc: sthlm, Known: true}

func at(y int, m time.Month, d, hh, mm int) time.Time { return time.Date(y, m, d, hh, mm, 0, 0, sthlm) }

func parseS(t *testing.T, s string) Schedule {
	t.Helper()
	var sc Schedule
	if err := json.Unmarshal([]byte(s), &sc); err != nil {
		t.Fatal(err)
	}
	return sc
}

func TestNormalize(t *testing.T) {
	s, err := parseS(t, `{"id":"s1","name":" Ute ","enabled":true,"deviceIds":["a","b","a"],"days":[5,1,1],
		"on":{"kind":"sunset","offset":-30,"notBefore":"16:00"},"off":{"kind":"time","time":"23:30"},"brightness":40}`).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "Ute" || len(s.DeviceIDs) != 2 || len(s.Days) != 2 || s.Days[0] != 1 || s.Days[1] != 5 {
		t.Fatalf("%+v", s)
	}
}

func TestNormalizeRejects(t *testing.T) {
	for name, js := range map[string]string{
		"no name":          `{"id":"s","name":" ","deviceIds":["a"],"on":{"kind":"time","time":"07:00"}}`,
		"long name":        `{"id":"s","name":"` + strings.Repeat("x", 41) + `","deviceIds":["a"],"on":{"kind":"time","time":"07:00"}}`,
		"no id":            `{"name":"x","deviceIds":["a"],"on":{"kind":"time","time":"07:00"}}`,
		"no devices":       `{"id":"s","name":"x","deviceIds":[],"on":{"kind":"time","time":"07:00"}}`,
		"nothing to do":    `{"id":"s","name":"x","deviceIds":["a"]}`,
		"bad time":         `{"id":"s","name":"x","deviceIds":["a"],"on":{"kind":"time","time":"25:00"}}`,
		"time missing":     `{"id":"s","name":"x","deviceIds":["a"],"on":{"kind":"time"}}`,
		"unknown kind":     `{"id":"s","name":"x","deviceIds":["a"],"on":{"kind":"moonrise"}}`,
		"offset too big":   `{"id":"s","name":"x","deviceIds":["a"],"on":{"kind":"sunset","offset":300}}`,
		"bad not before":   `{"id":"s","name":"x","deviceIds":["a"],"on":{"kind":"sunset","notBefore":"7"}}`,
		"before > after":   `{"id":"s","name":"x","deviceIds":["a"],"on":{"kind":"sunset","notBefore":"20:00","notAfter":"18:00"}}`,
		"bad day":          `{"id":"s","name":"x","deviceIds":["a"],"days":[7],"on":{"kind":"time","time":"07:00"}}`,
		"brightness > 100": `{"id":"s","name":"x","deviceIds":["a"],"brightness":101,"on":{"kind":"time","time":"07:00"}}`,
	} {
		if _, err := parseS(t, js).Normalize(); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestNextFixedTime(t *testing.T) {
	s := Schedule{ID: "s", Enabled: true, On: &Trigger{Kind: KindTime, Time: "07:00"}, Off: &Trigger{Kind: KindTime, Time: "08:30"}}
	ev, ok := s.Next(at(2026, 10, 29, 6, 0), home)
	if !ok || ev.Action != ActionOn || !ev.At.Equal(at(2026, 10, 29, 7, 0)) {
		t.Fatalf("%+v", ev)
	}
	ev, _ = s.Next(at(2026, 10, 29, 7, 0), home) // strictly after
	if ev.Action != ActionOff || !ev.At.Equal(at(2026, 10, 29, 8, 30)) {
		t.Fatalf("%+v", ev)
	}
	ev, _ = s.Next(at(2026, 10, 29, 9, 0), home)
	if !ev.At.Equal(at(2026, 10, 30, 7, 0)) {
		t.Fatalf("tomorrow: %+v", ev)
	}
}

func TestNextOnlyOnChosenDays(t *testing.T) {
	// weekdays only; Friday evening -> Monday morning
	s := Schedule{ID: "s", Enabled: true, Days: []int{1, 2, 3, 4, 5}, On: &Trigger{Kind: KindTime, Time: "06:30"}}
	ev, ok := s.Next(at(2026, 10, 30, 20, 0), home) // Fri 30 Oct
	if !ok || !ev.At.Equal(at(2026, 11, 2, 6, 30)) {
		t.Fatalf("%+v", ev)
	}
}

func TestNextAcrossDaylightSaving(t *testing.T) {
	// clocks go back on 25 Oct 2026: 07:00 is still 07:00 on the wall
	s := Schedule{ID: "s", Enabled: true, On: &Trigger{Kind: KindTime, Time: "07:00"}}
	ev, _ := s.Next(at(2026, 10, 24, 8, 0), home)
	if got := ev.At.In(sthlm).Format("2006-01-02 15:04"); got != "2026-10-25 07:00" {
		t.Fatal(got)
	}
}

func TestNextSunWithOffsetAndLimits(t *testing.T) {
	on := &Trigger{Kind: KindSunset, Offset: -30}
	s := Schedule{ID: "s", Enabled: true, On: on}
	ev, _ := s.Next(at(2026, 12, 21, 12, 0), home) // sunset ~14:48
	near(t, "sunset-30", ev.At, 14, 18)

	on.NotBefore = "16:00" // not before 16:00 in winter
	ev, _ = s.Next(at(2026, 12, 21, 12, 0), home)
	if !ev.At.Equal(at(2026, 12, 21, 16, 0)) {
		t.Fatalf("notBefore: %s", ev.At)
	}

	s.On = &Trigger{Kind: KindSunset, NotAfter: "21:00"} // summer sunset ~22:08
	ev, _ = s.Next(at(2026, 6, 21, 12, 0), home)
	if !ev.At.Equal(at(2026, 6, 21, 21, 0)) {
		t.Fatalf("notAfter: %s", ev.At)
	}

	s.On = &Trigger{Kind: KindSunrise, Offset: 15}
	ev, _ = s.Next(at(2026, 12, 21, 0, 30), home) // sunrise ~08:44
	near(t, "sunrise+15", ev.At, 8, 59)
}

func TestSunNeedsAPlace(t *testing.T) {
	s := Schedule{ID: "s", Enabled: true, On: &Trigger{Kind: KindSunset}, Off: &Trigger{Kind: KindTime, Time: "23:00"}}
	ev, ok := s.Next(at(2026, 10, 29, 12, 0), Place{Loc: sthlm})
	if !ok || ev.Action != ActionOff {
		t.Fatalf("without a place only the fixed time remains: %+v %v", ev, ok)
	}
}

func TestPolarDaySkipsSunTriggers(t *testing.T) {
	kiruna := Place{Lat: 67.8558, Lon: 20.2253, Loc: sthlm, Known: true}
	s := Schedule{ID: "s", Enabled: true, On: &Trigger{Kind: KindSunset}}
	ev, ok := s.Next(at(2026, 6, 21, 12, 0), kiruna)
	if ok && ev.At.Before(at(2026, 7, 5, 0, 0)) {
		t.Fatalf("no sunset under the midnight sun: %s", ev.At)
	}
}

func TestDisabledNeverRuns(t *testing.T) {
	s := Schedule{ID: "s", Enabled: false, On: &Trigger{Kind: KindTime, Time: "07:00"}}
	if _, ok := s.Next(at(2026, 10, 29, 6, 0), home); ok {
		t.Fatal("disabled")
	}
}

func TestDue(t *testing.T) {
	list := []Schedule{
		{ID: "a", Enabled: true, On: &Trigger{Kind: KindTime, Time: "07:00"}, Off: &Trigger{Kind: KindTime, Time: "07:10"}},
		{ID: "b", Enabled: true, On: &Trigger{Kind: KindTime, Time: "07:05"}},
		{ID: "c", Enabled: false, On: &Trigger{Kind: KindTime, Time: "07:05"}},
	}
	got := Due(list, at(2026, 10, 29, 6, 59), at(2026, 10, 29, 7, 10), home)
	var ids []string
	for _, e := range got {
		ids = append(ids, e.ScheduleID+":"+e.Action)
	}
	if strings.Join(ids, " ") != "a:on b:on a:off" {
		t.Fatalf("%v", ids)
	}
	if got := Due(list, at(2026, 10, 29, 7, 10), at(2026, 10, 29, 7, 11), home); len(got) != 0 {
		t.Fatalf("nothing new: %v", got)
	}
}
