package weather

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// --- SMHI (Sweden/Nordics), snow1g API ---------------------------------------------
// https://opendata.smhi.se/metfcst/snow1gv1 (replaced pmp3g on 2026-03-31)

type SMHI struct {
	BaseURL string // default https://opendata-download-metfcst.smhi.se
	Client  *http.Client
}

const smhiMissing = 9999

func (p *SMHI) Fetch(ctx context.Context, lat, lon float64, now time.Time, loc *time.Location) (Forecast, error) {
	base := p.BaseURL
	if base == "" {
		base = "https://opendata-download-metfcst.smhi.se"
	}
	u := fmt.Sprintf("%s/api/category/snow1g/version/1/geotype/point/lon/%.4f/lat/%.4f/data.json", base, lon, lat)
	var body struct {
		TimeSeries []struct {
			Time time.Time          `json:"time"`
			Data map[string]float64 `json:"data"`
		} `json:"timeSeries"`
	}
	if err := getJSON(ctx, p.Client, u, &body); err != nil {
		return Forecast{}, fmt.Errorf("SMHI: %w", err)
	}
	points := make([]point, 0, len(body.TimeSeries))
	for _, ts := range body.TimeSeries {
		temp, ok := ts.Data["air_temperature"]
		if !ok || temp == smhiMissing {
			temp = math.NaN()
		}
		points = append(points, point{t: ts.Time, temp: temp, cond: smhiCondition(int(ts.Data["symbol_code"]))})
	}
	if len(points) == 0 {
		return Forecast{}, fmt.Errorf("SMHI: empty forecast")
	}
	return fromHourly(points, now, loc), nil
}

// smhiCondition maps SMHI's symbol codes (1-27, same as the old Wsymb2).
func smhiCondition(code int) Condition {
	switch code {
	case 1:
		return Clear
	case 2:
		return MostlyClear
	case 3, 4:
		return PartlyCloudy
	case 5, 6:
		return Cloudy
	case 7:
		return Fog
	case 8, 9:
		return Showers
	case 10, 20:
		return HeavyRain
	case 11, 21:
		return Thunder
	case 12, 13, 14, 22, 23, 24:
		return Sleet
	case 15, 16, 17:
		return SnowShowers
	case 18:
		return LightRain
	case 19:
		return Rain
	case 25:
		return LightSnow
	case 26, 27:
		return Snow
	}
	return Unknown
}

// --- MET Norway (Yr), Locationforecast 2.0 -------------------------------------------
// https://api.met.no/weatherapi/locationforecast/2.0/documentation

type METNorway struct {
	BaseURL string // default https://api.met.no
	Client  *http.Client
}

func (p *METNorway) Fetch(ctx context.Context, lat, lon float64, now time.Time, loc *time.Location) (Forecast, error) {
	base := p.BaseURL
	if base == "" {
		base = "https://api.met.no"
	}
	u := fmt.Sprintf("%s/weatherapi/locationforecast/2.0/compact?lat=%.4f&lon=%.4f", base, lat, lon)
	type summary struct {
		Summary struct {
			SymbolCode string `json:"symbol_code"`
		} `json:"summary"`
	}
	var body struct {
		Properties struct {
			Timeseries []struct {
				Time time.Time `json:"time"`
				Data struct {
					Instant struct {
						Details struct {
							AirTemperature *float64 `json:"air_temperature"`
						} `json:"details"`
					} `json:"instant"`
					Next1 *summary `json:"next_1_hours"`
					Next6 *summary `json:"next_6_hours"`
				} `json:"data"`
			} `json:"timeseries"`
		} `json:"properties"`
	}
	if err := getJSON(ctx, p.Client, u, &body); err != nil {
		return Forecast{}, fmt.Errorf("MET Norway: %w", err)
	}
	points := make([]point, 0, len(body.Properties.Timeseries))
	for _, ts := range body.Properties.Timeseries {
		temp := math.NaN()
		if v := ts.Data.Instant.Details.AirTemperature; v != nil {
			temp = *v
		}
		code := ""
		if ts.Data.Next1 != nil {
			code = ts.Data.Next1.Summary.SymbolCode
		} else if ts.Data.Next6 != nil {
			code = ts.Data.Next6.Summary.SymbolCode
		}
		points = append(points, point{t: ts.Time, temp: temp, cond: metCondition(code)})
	}
	if len(points) == 0 {
		return Forecast{}, fmt.Errorf("MET Norway: empty forecast")
	}
	return fromHourly(points, now, loc), nil
}

// metCondition maps MET symbol codes like "lightrainshowers_day".
func metCondition(code string) Condition {
	c := code
	if i := strings.IndexByte(c, '_'); i >= 0 {
		c = c[:i] // drop _day/_night/_polartwilight
	}
	switch {
	case c == "":
		return Unknown
	case strings.Contains(c, "thunder"):
		return Thunder
	case strings.Contains(c, "sleet"):
		return Sleet
	case strings.Contains(c, "snowshowers"):
		return SnowShowers
	case c == "lightsnow":
		return LightSnow
	case strings.Contains(c, "snow"):
		return Snow
	case strings.Contains(c, "rainshowers"):
		return Showers
	case c == "lightrain":
		return LightRain
	case c == "heavyrain":
		return HeavyRain
	case c == "rain":
		return Rain
	case c == "clearsky":
		return Clear
	case c == "fair":
		return MostlyClear
	case c == "partlycloudy":
		return PartlyCloudy
	case c == "cloudy":
		return Cloudy
	case c == "fog":
		return Fog
	}
	return Unknown
}

// --- Open-Meteo (worldwide) ------------------------------------------------------------
// https://open-meteo.com/en/docs

type OpenMeteo struct {
	BaseURL string // default https://api.open-meteo.com
	Client  *http.Client
}

func (p *OpenMeteo) Fetch(ctx context.Context, lat, lon float64, now time.Time, loc *time.Location) (Forecast, error) {
	base := p.BaseURL
	if base == "" {
		base = "https://api.open-meteo.com"
	}
	q := url.Values{}
	q.Set("latitude", fmt.Sprintf("%.4f", lat))
	q.Set("longitude", fmt.Sprintf("%.4f", lon))
	q.Set("current", "temperature_2m,weather_code")
	q.Set("daily", "weather_code,temperature_2m_max,temperature_2m_min")
	q.Set("timezone", loc.String())
	q.Set("forecast_days", fmt.Sprint(Days+1))
	var body struct {
		Current struct {
			Temperature float64 `json:"temperature_2m"`
			WeatherCode int     `json:"weather_code"`
		} `json:"current"`
		Daily struct {
			Time        []string  `json:"time"`
			WeatherCode []int     `json:"weather_code"`
			Max         []float64 `json:"temperature_2m_max"`
			Min         []float64 `json:"temperature_2m_min"`
		} `json:"daily"`
	}
	if err := getJSON(ctx, p.Client, base+"/v1/forecast?"+q.Encode(), &body); err != nil {
		return Forecast{}, fmt.Errorf("Open-Meteo: %w", err)
	}
	f := Forecast{Current: Current{Temperature: body.Current.Temperature, Condition: wmoCondition(body.Current.WeatherCode)}}
	today := now.In(loc).Format("2006-01-02")
	d := body.Daily
	for i, date := range d.Time {
		if date < today || len(f.Daily) == Days || i >= len(d.Max) || i >= len(d.Min) || i >= len(d.WeatherCode) {
			continue
		}
		f.Daily = append(f.Daily, Day{Date: date, Min: d.Min[i], Max: d.Max[i], Condition: wmoCondition(d.WeatherCode[i])})
	}
	return f, nil
}

// wmoCondition maps WMO weather interpretation codes used by Open-Meteo.
func wmoCondition(code int) Condition {
	switch code {
	case 0:
		return Clear
	case 1:
		return MostlyClear
	case 2:
		return PartlyCloudy
	case 3:
		return Cloudy
	case 45, 48:
		return Fog
	case 51, 53, 55, 56, 57, 61:
		return LightRain
	case 63:
		return Rain
	case 65, 82:
		return HeavyRain
	case 66, 67:
		return Sleet
	case 71:
		return LightSnow
	case 73, 75, 77:
		return Snow
	case 80, 81:
		return Showers
	case 85, 86:
		return SnowShowers
	case 95, 96, 99:
		return Thunder
	}
	return Unknown
}

// --- place search (Open-Meteo geocoding, free) -----------------------------------------

type Place struct {
	Name      string  `json:"name"`
	Region    string  `json:"region,omitempty"`
	Country   string  `json:"country,omitempty"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type Geocoder interface {
	Search(ctx context.Context, q, lang string) ([]Place, error)
}

type OpenMeteoGeocoder struct {
	BaseURL string // default https://geocoding-api.open-meteo.com
	Client  *http.Client
}

func (g *OpenMeteoGeocoder) Search(ctx context.Context, q, lang string) ([]Place, error) {
	base := g.BaseURL
	if base == "" {
		base = "https://geocoding-api.open-meteo.com"
	}
	v := url.Values{}
	v.Set("name", q)
	v.Set("count", "8")
	v.Set("language", lang)
	v.Set("format", "json")
	var body struct {
		Results []struct {
			Name      string  `json:"name"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
			Country   string  `json:"country"`
			Admin1    string  `json:"admin1"`
		} `json:"results"`
	}
	if err := getJSON(ctx, g.Client, base+"/v1/search?"+v.Encode(), &body); err != nil {
		return nil, fmt.Errorf("place search: %w", err)
	}
	places := make([]Place, 0, len(body.Results))
	for _, r := range body.Results {
		places = append(places, Place{Name: r.Name, Region: r.Admin1, Country: r.Country, Latitude: r.Latitude, Longitude: r.Longitude})
	}
	return places, nil
}
