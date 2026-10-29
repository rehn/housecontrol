package api

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"testing"
	"time"

	"housecontrol/internal/weather"
)

type fakeWeather struct {
	err       error
	lastQuery weather.Query
	places    []weather.Place
	lastPlace [2]string
	areaErr   error
	areaCalls int
	areaLang  string
	locateErr error
}

func (f *fakeWeather) Forecast(_ context.Context, q weather.Query) (weather.Forecast, error) {
	f.lastQuery = q
	if f.err != nil {
		return weather.Forecast{}, f.err
	}
	return weather.Forecast{
		Provider: q.Provider,
		Location: q.Location,
		Current:  weather.Current{Temperature: 7.4, Condition: weather.PartlyCloudy},
		Daily:    []weather.Day{{Date: "2026-10-28", Min: 5.7, Max: 7.4, Condition: weather.PartlyCloudy}},
	}, nil
}

func (f *fakeWeather) Places(_ context.Context, q, lang string) ([]weather.Place, error) {
	f.lastPlace = [2]string{q, lang}
	return f.places, nil
}

func TestWeatherSettingsDefaults(t *testing.T) {
	e := newEnv(t)
	code, v := do(t, "GET", e.srv.URL+"/api/settings/weather", "")
	if code != 200 || v["provider"] != "smhi" || v["configured"] != false {
		t.Fatalf("%d %v", code, v)
	}
	providers, _ := v["providers"].([]any)
	if len(providers) != 3 {
		t.Fatalf("providers: %v", v["providers"])
	}
}

func TestWeatherNotConfigured(t *testing.T) {
	e := newEnv(t)
	code, v := do(t, "GET", e.srv.URL+"/api/weather", "")
	if code != 404 || v["code"] != "weather_not_configured" {
		t.Fatalf("%d %v", code, v)
	}
}

func TestWeatherSaveAndFetch(t *testing.T) {
	e := newEnv(t)
	events, cancel := e.reg.Subscribe()
	defer cancel()

	code, v := do(t, "PUT", e.srv.URL+"/api/settings/weather",
		`{"provider":"metno","latitude":59.6361,"longitude":17.0777,"location":" Enköping "}`)
	if code != 200 || v["provider"] != "metno" || v["location"] != "Enköping" || v["configured"] != true {
		t.Fatalf("save: %d %v", code, v)
	}
	select {
	case ev := <-events:
		if ev.Type != "weather" {
			t.Fatalf("event %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("clients were not told to reload the weather")
	}

	code, v = do(t, "GET", e.srv.URL+"/api/weather", "")
	if code != 200 || v["location"] != "Enköping" {
		t.Fatalf("weather: %d %v", code, v)
	}
	if q := e.weather.lastQuery; q.Provider != "metno" || q.Latitude != 59.6361 || q.Longitude != 17.0777 {
		t.Fatalf("query: %+v", q)
	}
	saved, _ := e.store.LoadWeather()
	if saved == nil || saved.Provider != "metno" {
		t.Fatalf("not persisted: %+v", saved)
	}
}

func TestWeatherSettingsValidation(t *testing.T) {
	e := newEnv(t)
	for _, body := range []string{
		`{"provider":"yahoo","latitude":59,"longitude":17}`,
		`{"provider":"smhi","latitude":91,"longitude":17}`,
		`{"provider":"smhi","latitude":59,"longitude":-181}`,
		`nope`,
	} {
		if code, _ := do(t, "PUT", e.srv.URL+"/api/settings/weather", body); code != 400 {
			t.Errorf("%s: %d", body, code)
		}
	}
}

func TestWeatherProviderDown(t *testing.T) {
	e := newEnv(t)
	do(t, "PUT", e.srv.URL+"/api/settings/weather", `{"provider":"smhi","latitude":59.6,"longitude":17.1,"location":"X"}`)
	e.weather.err = errors.New("SMHI: weather service answered 503")
	code, v := do(t, "GET", e.srv.URL+"/api/weather", "")
	if code != 502 || v["code"] != "weather_unavailable" {
		t.Fatalf("%d %v", code, v)
	}
}

func TestPlaceSearch(t *testing.T) {
	e := newEnv(t)
	e.weather.places = []weather.Place{{Name: "Enköping", Region: "Uppsala", Country: "Sverige", Latitude: 59.6, Longitude: 17.1}}

	req := e.srv.URL + "/api/weather/places?q=" + url.QueryEscape("Enköping") + "&lang=sv"
	code, v := doList(t, req)
	if code != 200 || len(v) != 1 || v[0]["name"] != "Enköping" {
		t.Fatalf("%d %v", code, v)
	}
	if e.weather.lastPlace != [2]string{"Enköping", "sv"} {
		t.Fatalf("%v", e.weather.lastPlace)
	}
	if code, _ := do(t, "GET", e.srv.URL+"/api/weather/places?q=a", ""); code != 400 {
		t.Fatalf("too short query should be rejected, got %d", code)
	}
}

func (f *fakeWeather) Area(_ context.Context, lat, lon float64, lang string) (string, error) {
	f.areaCalls++
	f.areaLang = lang
	if f.areaErr != nil {
		return "", f.areaErr
	}
	if lang == "sv" {
		return "Enköping, Uppsala län", nil
	}
	return "Enköping, Uppsala County", nil
}

func (f *fakeWeather) Locate(_ context.Context) (weather.Place, error) {
	if f.locateErr != nil {
		return weather.Place{}, f.locateErr
	}
	return weather.Place{Name: "Enköping", Region: "Uppsala", Country: "Sweden", Latitude: 59.63, Longitude: 17.07}, nil
}

// The area is looked up once, when the place is chosen and saved in
// Settings (in the app language); the weather card only shows it.
func TestWeatherAreaIsLookedUpWhenSavingSettings(t *testing.T) {
	e := newEnv(t)
	do(t, "PUT", e.srv.URL+"/api/settings/ui", `{"language":"sv"}`)

	code, v := do(t, "PUT", e.srv.URL+"/api/settings/weather",
		`{"provider":"smhi","latitude":59.6361,"longitude":17.0777,"location":"Enköping"}`)
	if code != 200 || v["area"] != "Enköping, Uppsala län" || e.weather.areaLang != "sv" {
		t.Fatalf("save should look up the area in the app language: %d %v", code, v)
	}
	if saved, _ := e.store.LoadWeather(); saved == nil || saved.Area != "Enköping, Uppsala län" {
		t.Fatalf("area not stored: %+v", saved)
	}

	for i := 0; i < 3; i++ {
		code, v = do(t, "GET", e.srv.URL+"/api/weather", "")
		if code != 200 || v["area"] != "Enköping, Uppsala län" {
			t.Fatalf("forecast should carry the stored area: %d %v", code, v)
		}
	}
	if e.weather.areaCalls != 1 {
		t.Fatalf("the weather card must not trigger lookups, got %d", e.weather.areaCalls)
	}
}

func TestWeatherWorksWhenAreaLookupFails(t *testing.T) {
	e := newEnv(t)
	e.weather.areaErr = errors.New("nominatim down")
	code, v := do(t, "PUT", e.srv.URL+"/api/settings/weather",
		`{"provider":"smhi","latitude":59.6,"longitude":17.1,"location":"Enköping"}`)
	if code != 200 || v["area"] != "" {
		t.Fatalf("%d %v", code, v)
	}
	if code, v = do(t, "GET", e.srv.URL+"/api/weather", ""); code != 200 || v["location"] != "Enköping" {
		t.Fatalf("%d %v", code, v)
	}
}

func TestAreaEndpoint(t *testing.T) {
	e := newEnv(t)
	code, v := do(t, "GET", e.srv.URL+"/api/weather/area?lat=59.6361&lon=17.0777&lang=sv", "")
	if code != 200 || v["area"] != "Enköping, Uppsala län" {
		t.Fatalf("%d %v", code, v)
	}
	if code, _ := do(t, "GET", e.srv.URL+"/api/weather/area?lat=99&lon=17", ""); code != 400 {
		t.Fatalf("bad latitude: %d", code)
	}
}

func TestLocateEndpoint(t *testing.T) {
	e := newEnv(t)
	code, v := do(t, "GET", e.srv.URL+"/api/weather/locate", "")
	if code != 200 || v["name"] != "Enköping" || v["latitude"] != 59.63 {
		t.Fatalf("%d %v", code, v)
	}
	e.weather.locateErr = errors.New("all IP services down")
	if code, v = do(t, "GET", e.srv.URL+"/api/weather/locate", ""); code != 502 || v["code"] != "locate_failed" {
		t.Fatalf("%d %v", code, v)
	}
}

func TestWeatherIncludesSunriseAndSunset(t *testing.T) {
	e := newEnv(t)
	do(t, "PUT", e.srv.URL+"/api/settings/weather", `{"provider":"smhi","latitude":59.6361,"longitude":17.0777,"location":"Enköping"}`)
	_, v := do(t, "GET", e.srv.URL+"/api/weather", "")
	sun, ok := v["sun"].(map[string]any)
	if !ok {
		t.Fatalf("no sun: %v", v)
	}
	rise, err1 := time.Parse(time.RFC3339, fmt.Sprint(sun["sunrise"]))
	set, err2 := time.Parse(time.RFC3339, fmt.Sprint(sun["sunset"]))
	if err1 != nil || err2 != nil || !set.After(rise) {
		t.Fatalf("%v", sun)
	}
	if y, m, d := rise.In(time.Local).Date(); time.Date(y, m, d, 0, 0, 0, 0, time.Local) != startOfToday() {
		t.Fatalf("sunrise is not today: %v", sun)
	}
}

func TestWeatherSunUnderMidnightSun(t *testing.T) {
	e := newEnv(t)
	// near the North Pole there is either polar day or night, any day of the year except around the equinoxes
	do(t, "PUT", e.srv.URL+"/api/settings/weather", `{"provider":"metno","latitude":89.9,"longitude":0,"location":"Pole"}`)
	_, v := do(t, "GET", e.srv.URL+"/api/weather", "")
	sun := v["sun"].(map[string]any)
	if p := sun["polar"]; p != "day" && p != "night" && sun["sunrise"] == nil {
		t.Fatalf("%v", sun)
	}
}

func startOfToday() time.Time {
	y, m, d := time.Now().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}
