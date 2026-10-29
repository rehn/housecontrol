package weather

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var stockholm = mustLoc("Europe/Stockholm")

func mustLoc(name string) *time.Location {
	l, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return l
}

// now: Wednesday 28 Oct 2026 21:30 local (20:30 UTC)
var now = time.Date(2026, 10, 28, 21, 30, 0, 0, stockholm)

func serve(t *testing.T, body string, check func(*http.Request)) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if check != nil {
			check(r)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

// --- SMHI (snow1g) -------------------------------------------------------------

const smhiJSON = `{"createdTime":"2026-10-28T20:00:00Z","referenceTime":"2026-10-28T19:00:00Z",
"geometry":{"type":"Point","coordinates":[17.08,59.64]},
"timeSeries":[
 {"time":"2026-10-28T20:00:00Z","data":{"air_temperature":7.4,"symbol_code":3}},
 {"time":"2026-10-28T21:00:00Z","data":{"air_temperature":6.9,"symbol_code":6}},
 {"time":"2026-10-28T22:00:00Z","data":{"air_temperature":5.7,"symbol_code":6}},
 {"time":"2026-10-29T05:00:00Z","data":{"air_temperature":3.7,"symbol_code":5}},
 {"time":"2026-10-29T11:00:00Z","data":{"air_temperature":10.3,"symbol_code":19}},
 {"time":"2026-10-29T17:00:00Z","data":{"air_temperature":8.0,"symbol_code":18}},
 {"time":"2026-10-30T11:00:00Z","data":{"air_temperature":9.9,"symbol_code":1}},
 {"time":"2026-10-30T22:00:00Z","data":{"air_temperature":3.5,"symbol_code":1}},
 {"time":"2026-10-31T11:00:00Z","data":{"air_temperature":8.5,"symbol_code":26}},
 {"time":"2026-10-31T12:00:00Z","data":{"air_temperature":9999,"symbol_code":9999}},
 {"time":"2026-10-31T22:00:00Z","data":{"air_temperature":2.3,"symbol_code":2}},
 {"time":"2026-11-01T11:00:00Z","data":{"air_temperature":7.7,"symbol_code":11}},
 {"time":"2026-11-01T22:00:00Z","data":{"air_temperature":4.8,"symbol_code":11}},
 {"time":"2026-11-02T11:00:00Z","data":{"air_temperature":6.0,"symbol_code":7}}
]}`

func TestSMHI(t *testing.T) {
	srv, _ := serve(t, smhiJSON, func(r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/api/category/snow1g/version/1/geotype/point/lon/17.0800/lat/59.6400/data.json") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})
	p := &SMHI{BaseURL: srv.URL, Client: srv.Client()}
	f, err := p.Fetch(context.Background(), 59.64, 17.08, now, stockholm)
	if err != nil {
		t.Fatal(err)
	}
	// current = the entry closest to now (20:30 UTC -> 20:00 or 21:00, first wins)
	if f.Current.Temperature != 7.4 || f.Current.Condition != PartlyCloudy {
		t.Errorf("current: %+v", f.Current)
	}
	if len(f.Daily) != 5 {
		t.Fatalf("want 5 days, got %d: %+v", len(f.Daily), f.Daily)
	}
	want := []Day{
		{Date: "2026-10-28", Min: 5.7, Max: 7.4, Condition: PartlyCloudy}, // evening only: first entry
		{Date: "2026-10-29", Min: 3.7, Max: 10.3, Condition: Rain},        // around noon: 19
		{Date: "2026-10-30", Min: 3.5, Max: 9.9, Condition: Clear},
		{Date: "2026-10-31", Min: 2.3, Max: 8.5, Condition: Snow}, // 9999 ignored
		{Date: "2026-11-01", Min: 4.8, Max: 7.7, Condition: Thunder},
	}
	for i, d := range want {
		if f.Daily[i] != d {
			t.Errorf("day %d: got %+v, want %+v", i, f.Daily[i], d)
		}
	}
}

func TestSMHISymbols(t *testing.T) {
	cases := map[int]Condition{
		1: Clear, 2: MostlyClear, 3: PartlyCloudy, 4: PartlyCloudy, 5: Cloudy, 6: Cloudy, 7: Fog,
		8: Showers, 9: Showers, 10: HeavyRain, 11: Thunder, 12: Sleet, 14: Sleet, 15: SnowShowers,
		17: SnowShowers, 18: LightRain, 19: Rain, 20: HeavyRain, 21: Thunder, 22: Sleet, 24: Sleet,
		25: LightSnow, 26: Snow, 27: Snow, 0: Unknown, 9999: Unknown,
	}
	for code, want := range cases {
		if got := smhiCondition(code); got != want {
			t.Errorf("%d: got %q want %q", code, got, want)
		}
	}
}

// --- MET Norway -------------------------------------------------------------------

const metJSON = `{"type":"Feature","properties":{"timeseries":[
 {"time":"2026-10-28T20:00:00Z","data":{"instant":{"details":{"air_temperature":7.0}},"next_1_hours":{"summary":{"symbol_code":"partlycloudy_night"}}}},
 {"time":"2026-10-29T11:00:00Z","data":{"instant":{"details":{"air_temperature":11.0}},"next_1_hours":{"summary":{"symbol_code":"lightrainshowers_day"}}}},
 {"time":"2026-10-29T22:00:00Z","data":{"instant":{"details":{"air_temperature":4.0}},"next_6_hours":{"summary":{"symbol_code":"cloudy"}}}}
]}}`

func TestMETNorway(t *testing.T) {
	srv, _ := serve(t, metJSON, func(r *http.Request) {
		if r.URL.Query().Get("lat") != "59.6400" || r.URL.Query().Get("lon") != "17.0800" {
			t.Errorf("query: %s", r.URL.RawQuery)
		}
		if ua := r.Header.Get("User-Agent"); !strings.Contains(ua, "housecontrol") {
			t.Errorf("MET Norway requires an identifying User-Agent, got %q", ua)
		}
	})
	p := &METNorway{BaseURL: srv.URL, Client: srv.Client()}
	f, err := p.Fetch(context.Background(), 59.64, 17.08, now, stockholm)
	if err != nil {
		t.Fatal(err)
	}
	if f.Current.Temperature != 7.0 || f.Current.Condition != PartlyCloudy {
		t.Errorf("current: %+v", f.Current)
	}
	if len(f.Daily) != 2 || f.Daily[1] != (Day{Date: "2026-10-29", Min: 4, Max: 11, Condition: Showers}) {
		t.Errorf("daily: %+v", f.Daily)
	}
}

func TestMETSymbols(t *testing.T) {
	cases := map[string]Condition{
		"clearsky_day": Clear, "fair_night": MostlyClear, "partlycloudy_polartwilight": PartlyCloudy,
		"cloudy": Cloudy, "fog": Fog, "lightrain": LightRain, "rain": Rain, "heavyrain": HeavyRain,
		"rainshowers_day": Showers, "heavyrainshowers_day": Showers, "rainandthunder": Thunder,
		"lightsnowshowersandthunder_day": Thunder, "sleet": Sleet, "lightsleetshowers_day": Sleet,
		"lightsnow": LightSnow, "snow": Snow, "heavysnow": Snow, "snowshowers_day": SnowShowers, "": Unknown,
	}
	for code, want := range cases {
		if got := metCondition(code); got != want {
			t.Errorf("%q: got %q want %q", code, got, want)
		}
	}
}

// --- Open-Meteo ---------------------------------------------------------------------

const openMeteoJSON = `{"latitude":59.64,"longitude":17.08,"timezone":"Europe/Stockholm",
"current":{"time":"2026-10-28T21:30","temperature_2m":7.4,"weather_code":2},
"daily":{"time":["2026-10-28","2026-10-29","2026-10-30","2026-10-31","2026-11-01","2026-11-02"],
 "weather_code":[2,61,0,73,95,45],
 "temperature_2m_max":[7.4,10.3,9.9,8.5,7.7,6.0],
 "temperature_2m_min":[5.7,3.7,3.5,2.3,4.8,1.0]}}`

func TestOpenMeteo(t *testing.T) {
	srv, _ := serve(t, openMeteoJSON, func(r *http.Request) {
		q := r.URL.Query()
		if q.Get("latitude") != "59.6400" || q.Get("timezone") == "" || !strings.Contains(q.Get("daily"), "weather_code") {
			t.Errorf("query: %s", r.URL.RawQuery)
		}
	})
	p := &OpenMeteo{BaseURL: srv.URL, Client: srv.Client()}
	f, err := p.Fetch(context.Background(), 59.64, 17.08, now, stockholm)
	if err != nil {
		t.Fatal(err)
	}
	if f.Current.Temperature != 7.4 || f.Current.Condition != PartlyCloudy {
		t.Errorf("current: %+v", f.Current)
	}
	if len(f.Daily) != 5 || f.Daily[1] != (Day{Date: "2026-10-29", Min: 3.7, Max: 10.3, Condition: LightRain}) ||
		f.Daily[4].Condition != Thunder {
		t.Errorf("daily: %+v", f.Daily)
	}
}

func TestWMOCodes(t *testing.T) {
	cases := map[int]Condition{0: Clear, 1: MostlyClear, 2: PartlyCloudy, 3: Cloudy, 45: Fog, 48: Fog,
		51: LightRain, 61: LightRain, 63: Rain, 65: HeavyRain, 66: Sleet, 71: LightSnow, 73: Snow, 75: Snow,
		80: Showers, 82: HeavyRain, 85: SnowShowers, 95: Thunder, 99: Thunder, 42: Unknown}
	for code, want := range cases {
		if got := wmoCondition(code); got != want {
			t.Errorf("%d: got %q want %q", code, got, want)
		}
	}
}

// --- errors ------------------------------------------------------------------------

func TestProviderHTTPErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()
	for _, p := range []Provider{
		&SMHI{BaseURL: srv.URL, Client: srv.Client()},
		&METNorway{BaseURL: srv.URL, Client: srv.Client()},
		&OpenMeteo{BaseURL: srv.URL, Client: srv.Client()},
	} {
		if _, err := p.Fetch(context.Background(), 59.6, 17.0, now, stockholm); err == nil {
			t.Errorf("%T: expected error on HTTP 404", p)
		}
	}
}

// --- service: caching + stale fallback ------------------------------------------------

type fakeProvider struct {
	calls atomic.Int32
	err   error
}

func (f *fakeProvider) Fetch(_ context.Context, lat, lon float64, _ time.Time, _ *time.Location) (Forecast, error) {
	f.calls.Add(1)
	if f.err != nil {
		return Forecast{}, f.err
	}
	return Forecast{Current: Current{Temperature: lat}}, nil
}

func TestServiceCaches(t *testing.T) {
	fp := &fakeProvider{}
	clock := now
	s := NewService(map[string]Provider{"smhi": fp}, nil, stockholm)
	s.now = func() time.Time { return clock }

	q := Query{Provider: "smhi", Latitude: 59.6, Longitude: 17.0, Location: "Enköping"}
	f, err := s.Forecast(context.Background(), q)
	if err != nil || f.Location != "Enköping" || f.Provider != "smhi" {
		t.Fatalf("%+v %v", f, err)
	}
	s.Forecast(context.Background(), q)
	if fp.calls.Load() != 1 {
		t.Fatalf("second call within 15 min should be cached, calls=%d", fp.calls.Load())
	}

	clock = clock.Add(16 * time.Minute)
	s.Forecast(context.Background(), q)
	if fp.calls.Load() != 2 {
		t.Fatalf("should refetch after the cache expired, calls=%d", fp.calls.Load())
	}

	q2 := q
	q2.Latitude = 60.0
	s.Forecast(context.Background(), q2)
	if fp.calls.Load() != 3 {
		t.Fatal("another location must not use the cached forecast")
	}
}

func TestServiceServesStaleOnError(t *testing.T) {
	fp := &fakeProvider{}
	clock := now
	s := NewService(map[string]Provider{"smhi": fp}, nil, stockholm)
	s.now = func() time.Time { return clock }
	q := Query{Provider: "smhi", Latitude: 59.6, Longitude: 17.0}

	if _, err := s.Forecast(context.Background(), q); err != nil {
		t.Fatal(err)
	}
	fp.err = context.DeadlineExceeded
	clock = clock.Add(30 * time.Minute)
	f, err := s.Forecast(context.Background(), q)
	if err != nil || !f.Stale || f.Current.Temperature != 59.6 {
		t.Fatalf("expected the last forecast marked stale, got %+v %v", f, err)
	}

	s2 := NewService(map[string]Provider{"smhi": fp}, nil, stockholm)
	if _, err := s2.Forecast(context.Background(), q); err == nil {
		t.Fatal("no cached data: the error must be returned")
	}
}

func TestServiceUnknownProvider(t *testing.T) {
	s := NewService(map[string]Provider{}, nil, stockholm)
	if _, err := s.Forecast(context.Background(), Query{Provider: "nope"}); err == nil {
		t.Fatal("expected error")
	}
}

// --- place search ------------------------------------------------------------------

func TestPlaces(t *testing.T) {
	srv, _ := serve(t, `{"results":[
		{"name":"Enköping","latitude":59.63607,"longitude":17.07768,"country":"Sverige","admin1":"Uppsala"},
		{"name":"Enköpings-Näs","latitude":59.6,"longitude":17.2,"country":"Sverige"}]}`,
		func(r *http.Request) {
			if r.URL.Query().Get("name") != "Enköping" || r.URL.Query().Get("language") != "sv" {
				t.Errorf("query: %s", r.URL.RawQuery)
			}
		})
	g := &OpenMeteoGeocoder{BaseURL: srv.URL, Client: srv.Client()}
	places, err := g.Search(context.Background(), "Enköping", "sv")
	if err != nil || len(places) != 2 {
		t.Fatalf("%+v %v", places, err)
	}
	if places[0] != (Place{Name: "Enköping", Region: "Uppsala", Country: "Sverige", Latitude: 59.63607, Longitude: 17.07768}) {
		t.Errorf("%+v", places[0])
	}
}

func TestPlacesNoResults(t *testing.T) {
	srv, _ := serve(t, `{"generationtime_ms":0.1}`, nil)
	g := &OpenMeteoGeocoder{BaseURL: srv.URL, Client: srv.Client()}
	places, err := g.Search(context.Background(), "zzzz", "en")
	if err != nil || places == nil || len(places) != 0 {
		t.Fatalf("want empty list, got %+v %v", places, err)
	}
}

// --- area name (Nominatim reverse geocoding) -----------------------------------------

func TestNominatimArea(t *testing.T) {
	srv, _ := serve(t, `{"place_id":1,"address":{"town":"Enköping","municipality":"Enköpings kommun",
		"county":"Uppsala län","country":"Sverige","country_code":"se"}}`, func(r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/reverse" || q.Get("lat") != "59.6361" || q.Get("lon") != "17.0777" ||
			q.Get("format") != "jsonv2" || q.Get("accept-language") != "sv" {
			t.Errorf("request: %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		if !strings.Contains(r.Header.Get("User-Agent"), "housecontrol") {
			t.Error("Nominatim requires an identifying User-Agent")
		}
	})
	n := &NominatimReverse{BaseURL: srv.URL, Client: srv.Client()}
	area, err := n.Area(context.Background(), 59.63607, 17.07768, "sv")
	if err != nil || area != "Enköping, Uppsala län" {
		t.Fatalf("%q %v", area, err)
	}
}

func TestAreaFromAddress(t *testing.T) {
	cases := []struct {
		addr map[string]string
		want string
	}{
		{map[string]string{"city": "Uppsala", "county": "Uppsala län"}, "Uppsala, Uppsala län"},
		{map[string]string{"village": "Bålsta", "state": "Uppsala län"}, "Bålsta, Uppsala län"},
		{map[string]string{"suburb": "Södermalm", "city": "Stockholm", "county": "Stockholms län"}, "Stockholm, Stockholms län"},
		{map[string]string{"municipality": "Enköpings kommun", "county": "Uppsala län"}, "Enköpings kommun, Uppsala län"},
		{map[string]string{"county": "Uppsala län", "country": "Sverige"}, "Uppsala län, Sverige"},
		{map[string]string{"city": "Oslo", "state": "Oslo"}, "Oslo"}, // no duplicate
		{map[string]string{"country": "Sverige"}, "Sverige"},
		{map[string]string{}, ""},
	}
	for _, c := range cases {
		if got := areaFromAddress(c.addr); got != c.want {
			t.Errorf("%v: got %q want %q", c.addr, got, c.want)
		}
	}
}

// --- approximate position from the internet connection -----------------------------------

func TestIPLocatorUsesFirstWorkingService(t *testing.T) {
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"success":false,"message":"rate limited"}`))
	}))
	defer down.Close()
	ipapi, _ := serve(t, `{"ip":"1.2.3.4","city":"Enköping","region":"Uppsala","country_name":"Sweden",
		"latitude":59.6356,"longitude":17.0764}`, nil)

	l := &IPLocator{Client: http.DefaultClient, IPWhoURL: down.URL, IPAPIURL: ipapi.URL}
	p, err := l.Locate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if p != (Place{Name: "Enköping", Region: "Uppsala", Country: "Sweden", Latitude: 59.6356, Longitude: 17.0764}) {
		t.Fatalf("%+v", p)
	}
}

func TestIPLocatorIPWho(t *testing.T) {
	ipwho, _ := serve(t, `{"ip":"1.2.3.4","success":true,"city":"Enköping","region":"Uppsala County",
		"country":"Sweden","latitude":59.63,"longitude":17.07}`, nil)
	l := &IPLocator{Client: http.DefaultClient, IPWhoURL: ipwho.URL, IPAPIURL: "http://127.0.0.1:1"}
	p, err := l.Locate(context.Background())
	if err != nil || p.Name != "Enköping" || p.Latitude != 59.63 {
		t.Fatalf("%+v %v", p, err)
	}
}

func TestIPLocatorAllDown(t *testing.T) {
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusTooManyRequests)
	}))
	defer bad.Close()
	l := &IPLocator{Client: http.DefaultClient, IPWhoURL: bad.URL, IPAPIURL: bad.URL}
	if _, err := l.Locate(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}
