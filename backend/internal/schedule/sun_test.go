package schedule

import (
	"testing"
	"time"
)

var sthlm, _ = time.LoadLocation("Europe/Stockholm")

func near(t *testing.T, what string, got time.Time, hh, mm int) {
	t.Helper()
	got = got.In(sthlm)
	want := time.Date(got.Year(), got.Month(), got.Day(), hh, mm, 0, 0, sthlm)
	if d := got.Sub(want); d < -3*time.Minute || d > 3*time.Minute {
		t.Errorf("%s = %s, want about %02d:%02d", what, got.Format("15:04"), hh, mm)
	}
}

func TestSunStockholm(t *testing.T) {
	const lat, lon = 59.3293, 18.0686
	for _, c := range []struct {
		date                     time.Time
		riseH, riseM, setH, setM int
	}{
		{time.Date(2026, 6, 21, 0, 0, 0, 0, sthlm), 3, 31, 22, 8},   // midsummer (CEST)
		{time.Date(2026, 12, 21, 0, 0, 0, 0, sthlm), 8, 44, 14, 48}, // midwinter (CET)
		{time.Date(2026, 3, 20, 0, 0, 0, 0, sthlm), 5, 53, 18, 0},   // equinox: noon ~11:55 (east of 15°E)
	} {
		s := SunOn(c.date, lat, lon)
		if s.Polar != "" {
			t.Fatalf("%s: polar %q", c.date.Format("2006-01-02"), s.Polar)
		}
		near(t, c.date.Format("2006-01-02")+" sunrise", s.Rise, c.riseH, c.riseM)
		near(t, c.date.Format("2006-01-02")+" sunset", s.Set, c.setH, c.setM)
	}
}

func TestSunPolar(t *testing.T) {
	const lat, lon = 67.8558, 20.2253 // Kiruna
	if s := SunOn(time.Date(2026, 6, 21, 0, 0, 0, 0, sthlm), lat, lon); s.Polar != PolarDay {
		t.Errorf("midnight sun expected, got %+v", s)
	}
	if s := SunOn(time.Date(2026, 12, 21, 0, 0, 0, 0, sthlm), lat, lon); s.Polar != PolarNight {
		t.Errorf("polar night expected, got %+v", s)
	}
}
