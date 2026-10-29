package settings

import (
	"os"
	"testing"

	"housecontrol/internal/config"
	"housecontrol/internal/dashboard"
	"housecontrol/internal/schedule"
)

func TestRoundTrip(t *testing.T) {
	s := NewStore(t.TempDir() + "/nested")
	if m, err := s.LoadMQTT(); err != nil || m != nil {
		t.Fatalf("empty store: %v %v", m, err)
	}
	want := config.MQTT{URL: "tcp://192.168.1.201:1883", Username: "u", Password: "p", BaseTopic: "zigbee2mqtt"}
	if err := s.SaveMQTT(want); err != nil {
		t.Fatal(err)
	}
	got, err := s.LoadMQTT()
	if err != nil || got == nil || *got != want {
		t.Fatalf("got %+v %v", got, err)
	}
	fi, _ := os.Stat(s.Path())
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("perm %v", fi.Mode().Perm())
	}
}

func TestDashboardRoundTripKeepsMQTT(t *testing.T) {
	s := NewStore(t.TempDir())
	if d, err := s.LoadDashboard(); err != nil || d != nil {
		t.Fatalf("empty store: %v %v", d, err)
	}
	if err := s.SaveMQTT(config.MQTT{URL: "tcp://pi:1883"}); err != nil {
		t.Fatal(err)
	}
	want := dashboard.Dashboard{Version: 1, Columns: []dashboard.Column{
		{ID: "c1", Cards: []dashboard.Card{{ID: "a", Type: "clock"}}},
	}}
	if err := s.SaveDashboard(want); err != nil {
		t.Fatal(err)
	}
	got, err := s.LoadDashboard()
	if err != nil || got == nil || got.Columns[0].Cards[0].Type != "clock" {
		t.Fatalf("got %+v %v", got, err)
	}
	if m, _ := s.LoadMQTT(); m == nil || m.URL != "tcp://pi:1883" {
		t.Fatalf("saving the dashboard lost the MQTT settings: %+v", m)
	}
}

func TestUIRoundTripKeepsOtherSettings(t *testing.T) {
	s := NewStore(t.TempDir())
	if u, err := s.LoadUI(); err != nil || u != nil {
		t.Fatalf("empty store: %v %v", u, err)
	}
	if err := s.SaveMQTT(config.MQTT{URL: "tcp://pi:1883"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveUI(UI{Language: "sv"}); err != nil {
		t.Fatal(err)
	}
	u, err := s.LoadUI()
	if err != nil || u == nil || u.Language != "sv" {
		t.Fatalf("got %+v %v", u, err)
	}
	if m, _ := s.LoadMQTT(); m == nil || m.URL != "tcp://pi:1883" {
		t.Fatalf("saving UI settings lost the MQTT settings: %+v", m)
	}
}

func TestWeatherRoundTripKeepsOtherSettings(t *testing.T) {
	s := NewStore(t.TempDir())
	if w, err := s.LoadWeather(); err != nil || w != nil {
		t.Fatalf("empty: %v %v", w, err)
	}
	s.SaveUI(UI{Language: "sv"})
	if err := s.SaveWeather(Weather{Provider: "smhi", Latitude: 59.6, Longitude: 17.1, Location: "Enköping"}); err != nil {
		t.Fatal(err)
	}
	w, _ := s.LoadWeather()
	if w == nil || w.Location != "Enköping" || w.Latitude != 59.6 {
		t.Fatalf("%+v", w)
	}
	if u, _ := s.LoadUI(); u == nil || u.Language != "sv" {
		t.Fatal("saving weather lost the UI settings")
	}
}

func TestCalendarsRoundTrip(t *testing.T) {
	s := NewStore(t.TempDir())
	if c, err := s.LoadCalendars(); err != nil || len(c) != 0 {
		t.Fatalf("%v %v", c, err)
	}
	s.SaveUI(UI{Language: "sv"})
	want := []Calendar{{ID: "c1", Name: "Familjen", URL: "https://x/y.ics", Color: "#f5c242"}}
	if err := s.SaveCalendars(want); err != nil {
		t.Fatal(err)
	}
	got, _ := s.LoadCalendars()
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("%+v", got)
	}
	if u, _ := s.LoadUI(); u == nil || u.Language != "sv" {
		t.Fatal("saving calendars lost other settings")
	}
}

func TestSchedulesRoundTrip(t *testing.T) {
	s := NewStore(t.TempDir())
	if l, err := s.LoadSchedules(); err != nil || len(l) != 0 {
		t.Fatalf("%v %v", l, err)
	}
	s.SaveUI(UI{Language: "sv"})
	want := []schedule.Schedule{{ID: "s1", Name: "Ute", Enabled: true, DeviceIDs: []string{"0x01"},
		On: &schedule.Trigger{Kind: schedule.KindSunset, Offset: -30}, Brightness: 40}}
	if err := s.SaveSchedules(want); err != nil {
		t.Fatal(err)
	}
	got, _ := s.LoadSchedules()
	if len(got) != 1 || got[0].Name != "Ute" || got[0].On.Offset != -30 || got[0].Brightness != 40 {
		t.Fatalf("%+v", got)
	}
	if u, _ := s.LoadUI(); u == nil || u.Language != "sv" {
		t.Fatal("saving schedules lost other settings")
	}
}
