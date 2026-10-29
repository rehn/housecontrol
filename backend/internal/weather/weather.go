// Package weather fetches forecasts from free providers (SMHI, MET Norway,
// Open-Meteo) and normalises them into one small format for the UI.
package weather

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"sync"
	"time"
)

// Condition is a provider-independent weather type; the frontend translates
// it (locales/*.json "weather.conditions.<condition>") and picks an icon.
type Condition string

const (
	Clear        Condition = "clear"
	MostlyClear  Condition = "mostly_clear"
	PartlyCloudy Condition = "partly_cloudy"
	Cloudy       Condition = "cloudy"
	Fog          Condition = "fog"
	LightRain    Condition = "light_rain"
	Rain         Condition = "rain"
	HeavyRain    Condition = "heavy_rain"
	Showers      Condition = "showers"
	Thunder      Condition = "thunder"
	Sleet        Condition = "sleet"
	LightSnow    Condition = "light_snow"
	Snow         Condition = "snow"
	SnowShowers  Condition = "snow_showers"
	Unknown      Condition = "unknown"
)

type Forecast struct {
	Provider string `json:"provider"`
	Location string `json:"location,omitempty"`
	// Area is the stored area name, e.g. "Enköping, Uppsala län".
	Area    string    `json:"area,omitempty"`
	Updated time.Time `json:"updated"`
	// Stale is set when the provider failed and this is the last good forecast.
	Stale   bool    `json:"stale,omitempty"`
	Current Current `json:"current"`
	Daily   []Day   `json:"daily"`
}

type Current struct {
	Temperature float64   `json:"temperature"`
	Condition   Condition `json:"condition"`
}

type Day struct {
	Date      string    `json:"date"` // YYYY-MM-DD, local
	Min       float64   `json:"min"`
	Max       float64   `json:"max"`
	Condition Condition `json:"condition"`
}

// Days is how many days (including today) a forecast covers.
const Days = 5

// Provider fetches a forecast for a coordinate. `now` and `loc` decide what
// "today" is and how hourly data is grouped into days.
type Provider interface {
	Fetch(ctx context.Context, lat, lon float64, now time.Time, loc *time.Location) (Forecast, error)
}

// ProviderIDs lists the selectable providers; the first is the default.
var ProviderIDs = []string{"smhi", "metno", "openmeteo"}

// DefaultProviders builds the real providers with a shared HTTP client.
func DefaultProviders() map[string]Provider {
	c := &http.Client{Timeout: 15 * time.Second}
	return map[string]Provider{
		"smhi":      &SMHI{Client: c},
		"metno":     &METNorway{Client: c},
		"openmeteo": &OpenMeteo{Client: c},
	}
}

const userAgent = "housecontrol/0.1 (self-hosted home automation)"

func getJSON(ctx context.Context, c *http.Client, url string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent) // required by MET Norway
	req.Header.Set("Accept", "application/json")
	if c == nil {
		c = http.DefaultClient
	}
	res, err := c.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
		return fmt.Errorf("weather service answered %s", res.Status)
	}
	return json.NewDecoder(io.LimitReader(res.Body, 8<<20)).Decode(v)
}

// --- hourly series -> current + daily -------------------------------------------------

type point struct {
	t    time.Time
	temp float64 // NaN = missing
	cond Condition
}

// fromHourly builds a forecast from a time series: current = point closest
// to now; per local day min/max temperature and the condition closest to noon.
func fromHourly(points []point, now time.Time, loc *time.Location) Forecast {
	var f Forecast
	if len(points) == 0 {
		return f
	}
	sort.SliceStable(points, func(i, j int) bool { return points[i].t.Before(points[j].t) })

	best := -1
	for i, p := range points {
		if math.IsNaN(p.temp) {
			continue
		}
		if best < 0 || absDur(p.t.Sub(now)) < absDur(points[best].t.Sub(now)) {
			best = i
		}
	}
	if best >= 0 {
		f.Current = Current{Temperature: points[best].temp, Condition: points[best].cond}
	}

	type acc struct {
		day       Day
		hasTemp   bool
		noonDist  time.Duration
		hasSymbol bool
	}
	today := now.In(loc).Format("2006-01-02")
	var order []string
	days := map[string]*acc{}
	for _, p := range points {
		lt := p.t.In(loc)
		key := lt.Format("2006-01-02")
		if key < today {
			continue
		}
		a := days[key]
		if a == nil {
			if len(order) == Days {
				continue
			}
			a = &acc{day: Day{Date: key, Condition: Unknown}}
			days[key] = a
			order = append(order, key)
		}
		if !math.IsNaN(p.temp) {
			if !a.hasTemp || p.temp < a.day.Min {
				a.day.Min = p.temp
			}
			if !a.hasTemp || p.temp > a.day.Max {
				a.day.Max = p.temp
			}
			a.hasTemp = true
		}
		if p.cond != Unknown {
			noon := time.Date(lt.Year(), lt.Month(), lt.Day(), 12, 0, 0, 0, loc)
			d := absDur(lt.Sub(noon))
			if !a.hasSymbol || d < a.noonDist {
				a.day.Condition, a.noonDist, a.hasSymbol = p.cond, d, true
			}
		}
	}
	for _, k := range order {
		if days[k].hasTemp {
			f.Daily = append(f.Daily, days[k].day)
		}
	}
	return f
}

func absDur(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// --- service: cache + stale fallback ------------------------------------------------

// Query is what the UI asks for (from the saved weather settings).
type Query struct {
	Provider  string
	Latitude  float64
	Longitude float64
	Location  string // display name
}

type cached struct {
	f       Forecast
	fetched time.Time
}

type Service struct {
	providers map[string]Provider
	geo       Geocoder
	reverse   Reverse
	locator   Locator
	loc       *time.Location
	ttl       time.Duration
	maxStale  time.Duration
	now       func() time.Time

	mu    sync.Mutex
	cache map[string]cached
}

func NewService(providers map[string]Provider, geo Geocoder, loc *time.Location) *Service {
	if loc == nil {
		loc = time.Local
	}
	return &Service{
		providers: providers,
		geo:       geo,
		loc:       loc,
		ttl:       15 * time.Minute,
		maxStale:  6 * time.Hour,
		now:       time.Now,
		cache:     map[string]cached{},
	}
}

var ErrUnknownProvider = errors.New("unknown weather provider")

// Forecast returns a cached forecast if fresh, otherwise fetches a new one.
// If fetching fails, a recent cached forecast is returned with Stale set.
func (s *Service) Forecast(ctx context.Context, q Query) (Forecast, error) {
	p, ok := s.providers[q.Provider]
	if !ok {
		return Forecast{}, ErrUnknownProvider
	}
	key := fmt.Sprintf("%s|%.4f|%.4f", q.Provider, q.Latitude, q.Longitude)
	now := s.now()

	s.mu.Lock()
	c, hit := s.cache[key]
	s.mu.Unlock()
	if hit && now.Sub(c.fetched) < s.ttl {
		return c.withLocation(q.Location), nil
	}

	f, err := p.Fetch(ctx, q.Latitude, q.Longitude, now, s.loc)
	if err != nil {
		if hit && now.Sub(c.fetched) < s.maxStale {
			stale := c.withLocation(q.Location)
			stale.Stale = true
			return stale, nil
		}
		return Forecast{}, err
	}
	f.Provider = q.Provider
	f.Updated = now
	s.mu.Lock()
	s.cache[key] = cached{f: f, fetched: now}
	s.mu.Unlock()
	f.Location = q.Location
	return f, nil
}

func (c cached) withLocation(name string) Forecast {
	f := c.f
	f.Location = name
	return f
}

// Places searches for locations by name.
func (s *Service) Places(ctx context.Context, q, lang string) ([]Place, error) {
	if s.geo == nil {
		return nil, errors.New("place search is not available")
	}
	return s.geo.Search(ctx, q, lang)
}
