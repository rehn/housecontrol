package weather

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// --- area name from coordinates (OpenStreetMap Nominatim) ------------------------------
// https://nominatim.org/release-docs/develop/api/Reverse
// Free; requires an identifying User-Agent and at most ~1 request/second,
// which is fine since the result is stored in the settings.

type NominatimReverse struct {
	BaseURL string // default https://nominatim.openstreetmap.org
	Client  *http.Client
}

// Area returns e.g. "Enköping, Uppsala län" in the given language.
func (n *NominatimReverse) Area(ctx context.Context, lat, lon float64, lang string) (string, error) {
	base := n.BaseURL
	if base == "" {
		base = "https://nominatim.openstreetmap.org"
	}
	q := url.Values{}
	q.Set("lat", fmt.Sprintf("%.4f", lat))
	q.Set("lon", fmt.Sprintf("%.4f", lon))
	q.Set("format", "jsonv2")
	q.Set("zoom", "10") // city level
	q.Set("accept-language", lang)
	var body struct {
		Address map[string]string `json:"address"`
		Error   string            `json:"error"`
	}
	if err := getJSON(ctx, n.Client, base+"/reverse?"+q.Encode(), &body); err != nil {
		return "", fmt.Errorf("area lookup: %w", err)
	}
	if body.Error != "" {
		return "", fmt.Errorf("area lookup: %s", body.Error)
	}
	area := areaFromAddress(body.Address)
	if area == "" {
		return "", errors.New("area lookup: no name for this position")
	}
	return area, nil
}

// areaFromAddress builds "<locality>, <county/state>", falling back to
// "<county/state>, <country>" outside built-up areas.
func areaFromAddress(a map[string]string) string {
	first := func(keys ...string) string {
		for _, k := range keys {
			if v := strings.TrimSpace(a[k]); v != "" {
				return v
			}
		}
		return ""
	}
	join := func(x, y string) string {
		switch {
		case x == "":
			return y
		case y == "" || y == x:
			return x
		}
		return x + ", " + y
	}
	locality := first("city", "town", "village", "municipality")
	region := first("county", "state")
	if locality != "" {
		return join(locality, region)
	}
	return join(region, first("country"))
}

// --- approximate position from the internet connection ----------------------------------
// The backend runs at home, so its public IP gives the home's rough location.

type IPLocator struct {
	Client   *http.Client
	IPWhoURL string // default https://ipwho.is/  (free, no key)
	IPAPIURL string // default https://ipapi.co/json/ (free, no key, fallback)
}

// Locate tries the services in order and returns the first answer.
func (l *IPLocator) Locate(ctx context.Context) (Place, error) {
	var errs []error
	for _, try := range []func(context.Context) (Place, error){l.ipwho, l.ipapi} {
		p, err := try(ctx)
		if err == nil {
			return p, nil
		}
		errs = append(errs, err)
	}
	return Place{}, fmt.Errorf("could not find the position: %w", errors.Join(errs...))
}

func (l *IPLocator) ipwho(ctx context.Context) (Place, error) {
	u := l.IPWhoURL
	if u == "" {
		u = "https://ipwho.is/"
	}
	var b struct {
		Success   bool    `json:"success"`
		Message   string  `json:"message"`
		City      string  `json:"city"`
		Region    string  `json:"region"`
		Country   string  `json:"country"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}
	if err := getJSON(ctx, l.Client, u, &b); err != nil {
		return Place{}, fmt.Errorf("ipwho.is: %w", err)
	}
	if !b.Success {
		return Place{}, fmt.Errorf("ipwho.is: %s", b.Message)
	}
	return place(b.City, b.Region, b.Country, b.Latitude, b.Longitude, "ipwho.is")
}

func (l *IPLocator) ipapi(ctx context.Context) (Place, error) {
	u := l.IPAPIURL
	if u == "" {
		u = "https://ipapi.co/json/"
	}
	var b struct {
		Error     bool    `json:"error"`
		Reason    string  `json:"reason"`
		City      string  `json:"city"`
		Region    string  `json:"region"`
		Country   string  `json:"country_name"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}
	if err := getJSON(ctx, l.Client, u, &b); err != nil {
		return Place{}, fmt.Errorf("ipapi.co: %w", err)
	}
	if b.Error {
		return Place{}, fmt.Errorf("ipapi.co: %s", b.Reason)
	}
	return place(b.City, b.Region, b.Country, b.Latitude, b.Longitude, "ipapi.co")
}

func place(city, region, country string, lat, lon float64, src string) (Place, error) {
	if lat == 0 && lon == 0 {
		return Place{}, fmt.Errorf("%s: no coordinates", src)
	}
	return Place{Name: city, Region: region, Country: country, Latitude: lat, Longitude: lon}, nil
}

// --- service wiring ---------------------------------------------------------------------

type Reverse interface {
	Area(ctx context.Context, lat, lon float64, lang string) (string, error)
}

type Locator interface {
	Locate(ctx context.Context) (Place, error)
}

// WithGeo adds area lookup and IP positioning to the service.
func (s *Service) WithGeo(r Reverse, l Locator) *Service {
	s.reverse, s.locator = r, l
	return s
}

func (s *Service) Area(ctx context.Context, lat, lon float64, lang string) (string, error) {
	if s.reverse == nil {
		return "", errors.New("area lookup is not available")
	}
	return s.reverse.Area(ctx, lat, lon, lang)
}

func (s *Service) Locate(ctx context.Context) (Place, error) {
	if s.locator == nil {
		return Place{}, errors.New("positioning is not available")
	}
	return s.locator.Locate(ctx)
}
