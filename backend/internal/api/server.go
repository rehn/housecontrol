// Package api exposes the REST + WebSocket API used by the frontend, and
// optionally serves the built frontend as static files.
package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"housecontrol/internal/auth"
	"housecontrol/internal/calendar"
	"housecontrol/internal/config"
	"housecontrol/internal/dashboard"
	"housecontrol/internal/devices"
	"housecontrol/internal/mqttclient"
	"housecontrol/internal/schedule"
	"housecontrol/internal/settings"
	"housecontrol/internal/weather"
)

// MQTT is the broker connection (implemented by mqttclient.Manager).
type MQTT interface {
	Set(name string, payload map[string]any) error
	Status() devices.MQTTStatus
	Config() (config.MQTT, string)
	Apply(cfg config.MQTT, source string)
	Test(cfg config.MQTT) error
}

// SettingsStore persists settings changed from the UI.
type SettingsStore interface {
	SaveMQTT(config.MQTT) error
	LoadDashboard() (*dashboard.Dashboard, error)
	SaveDashboard(dashboard.Dashboard) error
	LoadUI() (*settings.UI, error)
	SaveUI(settings.UI) error
	LoadWeather() (*settings.Weather, error)
	SaveWeather(settings.Weather) error
	LoadCalendars() ([]settings.Calendar, error)
	SaveCalendars([]settings.Calendar) error
	LoadSchedules() ([]schedule.Schedule, error)
	SaveSchedules([]schedule.Schedule) error
	Snapshot() (settings.Snapshot, error)
	Restore(settings.Snapshot) error
}

// CalendarService downloads iCal calendars (calendar.Service).
type CalendarService interface {
	Upcoming(ctx context.Context, sources []calendar.Source, from time.Time, days int) calendar.Result
	Test(ctx context.Context, url string) (int, error)
}

// WeatherService fetches forecasts and searches places (weather.Service).
type WeatherService interface {
	Forecast(ctx context.Context, q weather.Query) (weather.Forecast, error)
	Places(ctx context.Context, q, lang string) ([]weather.Place, error)
	Area(ctx context.Context, lat, lon float64, lang string) (string, error)
	Locate(ctx context.Context) (weather.Place, error)
}

type Options struct {
	CORSOrigins []string
	StaticDir   string          // built frontend; empty = API only
	Weather     WeatherService  // nil = weather cards unavailable
	Calendar    CalendarService // nil = calendar cards unavailable
	Auth        *auth.Auth      // nil or no password = no login needed
	// TrustProxy: running behind a reverse proxy / Kubernetes Gateway, so the
	// client is the last address in X-Forwarded-For (else every request would
	// seem to come from the proxy). Never set it when clients connect directly.
	TrustProxy bool
	Version    string // shown in backups
}

type Server struct {
	reg     *devices.Registry
	mqtt    MQTT
	store   SettingsStore
	log     *slog.Logger
	opts    Options
	limiter *auth.Limiter
}

func New(reg *devices.Registry, mq MQTT, store SettingsStore, log *slog.Logger, opts Options) *Server {
	return &Server{reg: reg, mqtt: mq, store: store, log: log, opts: opts,
		limiter: auth.NewLimiter(5, 5*time.Minute)}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	ok := func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) }
	mux.HandleFunc("GET /healthz", ok)
	// Ready as soon as HTTP is up: the UI must stay reachable even when the
	// broker is down, so MQTT can be (re)configured from it.
	mux.HandleFunc("GET /readyz", ok)

	mux.HandleFunc("GET /api/auth", s.authStatus)
	mux.HandleFunc("POST /api/auth/login", s.login)
	mux.HandleFunc("POST /api/auth/logout", s.logout)
	mux.HandleFunc("GET /api/status", s.status)
	mux.HandleFunc("GET /api/devices", s.listDevices)
	mux.HandleFunc("GET /api/devices/{id}", s.getDevice)
	mux.HandleFunc("POST /api/devices/{id}/set", s.setDevice)
	mux.HandleFunc("POST /api/devices/set-many", s.setMany)
	mux.HandleFunc("GET /api/settings/ui", s.getUI)
	mux.HandleFunc("PUT /api/settings/ui", s.putUI)
	mux.HandleFunc("GET /api/settings/weather", s.getWeatherSettings)
	mux.HandleFunc("PUT /api/settings/weather", s.putWeatherSettings)
	mux.HandleFunc("GET /api/weather", s.getWeather)
	mux.HandleFunc("GET /api/weather/places", s.searchPlaces)
	mux.HandleFunc("GET /api/weather/area", s.lookupArea)
	mux.HandleFunc("GET /api/weather/locate", s.locate)
	mux.HandleFunc("GET /api/settings/calendars", s.getCalendars)
	mux.HandleFunc("PUT /api/settings/calendars", s.putCalendars)
	mux.HandleFunc("POST /api/settings/calendars/test", s.testCalendar)
	mux.HandleFunc("GET /api/calendar", s.getCalendar)
	mux.HandleFunc("POST /api/settings/backup", s.exportBackup)
	mux.HandleFunc("POST /api/settings/restore", s.restoreBackup)
	mux.HandleFunc("GET /api/settings/schedules", s.getSchedules)
	mux.HandleFunc("PUT /api/settings/schedules", s.putSchedules)
	mux.HandleFunc("GET /api/dashboard", s.getDashboard)
	mux.HandleFunc("PUT /api/dashboard", s.putDashboard)
	mux.HandleFunc("GET /api/settings/mqtt", s.getMQTTSettings)
	mux.HandleFunc("PUT /api/settings/mqtt", s.putMQTTSettings)
	mux.HandleFunc("POST /api/settings/mqtt/test", s.testMQTTSettings)
	mux.HandleFunc("GET /api/ws", s.ws)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeErr(w, http.StatusNotFound, "not found")
	})

	if s.opts.StaticDir != "" {
		mux.Handle("/", staticHandler(s.opts.StaticDir))
	}
	return s.cors(s.requireLogin(mux))
}

func (s *Server) status(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"mqtt": s.mqtt.Status()})
}

func (s *Server) listDevices(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.reg.List())
}

func (s *Server) getDevice(w http.ResponseWriter, r *http.Request) {
	d, ok := s.reg.Get(r.PathValue("id"))
	if !ok {
		writeErrCode(w, http.StatusNotFound, codeDeviceNotFound, "device not found")
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// allowed command keys; keeps the API from becoming a raw MQTT pipe.
var allowedKeys = []string{"state", "brightness", "color_temp", "color", "transition"}

// validateCommand checks a device command and upper-cases "state".
func validateCommand(body map[string]any) error {
	if len(body) == 0 {
		return errors.New("empty command")
	}
	for k, v := range body {
		if !slices.Contains(allowedKeys, k) {
			return errors.New("unsupported property: " + k)
		}
		if k == "state" {
			sv, _ := v.(string)
			sv = strings.ToUpper(sv)
			if sv != "ON" && sv != "OFF" && sv != "TOGGLE" {
				return errors.New("state must be ON, OFF or TOGGLE")
			}
			body[k] = sv
		}
	}
	return nil
}

func (s *Server) setDevice(w http.ResponseWriter, r *http.Request) {
	d, ok := s.reg.Get(r.PathValue("id"))
	if !ok {
		writeErrCode(w, http.StatusNotFound, codeDeviceNotFound, "device not found")
		return
	}
	var body map[string]any
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := validateCommand(body); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.mqtt.Set(d.Name, body); err != nil {
		writeMQTTErr(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// setMany sends the same command to several devices (group cards).
// Body: {"ids": ["0x..", ...], "payload": {"state": "ON"}}
func (s *Server) setMany(w http.ResponseWriter, r *http.Request) {
	var in struct {
		IDs     []string       `json:"ids"`
		Payload map[string]any `json:"payload"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if len(in.IDs) == 0 || len(in.IDs) > dashboard.MaxGroupDevices {
		writeErr(w, http.StatusBadRequest, "ids must list 1-50 devices")
		return
	}
	if err := validateCommand(in.Payload); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	names := make([]string, 0, len(in.IDs))
	for _, id := range in.IDs {
		d, ok := s.reg.Get(id)
		if !ok {
			writeErr(w, http.StatusBadRequest, "unknown device: "+id)
			return
		}
		names = append(names, d.Name)
	}
	failed := map[string]string{}
	notConnected := false
	for i, name := range names {
		if err := s.mqtt.Set(name, in.Payload); err != nil {
			failed[in.IDs[i]] = err.Error()
			notConnected = notConnected || errors.Is(err, mqttclient.ErrNotConnected)
		}
	}
	switch {
	case len(failed) == 0:
		w.WriteHeader(http.StatusAccepted)
	case len(failed) == len(names):
		res := map[string]any{"error": failed[in.IDs[0]], "failed": failed}
		if notConnected {
			res["code"] = codeMQTTNotConnected
		}
		writeJSON(w, http.StatusServiceUnavailable, res)
	default:
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "some devices failed", "failed": failed})
	}
}

// --- UI settings (language) ------------------------------------------------------

// Language tags like "en", "sv", "pt-BR"; also keeps file paths out.
var languageRe = regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z0-9]{2,8})?$`)

func (s *Server) currentUI() settings.UI {
	u := settings.UI{Language: settings.DefaultLanguage}
	if saved, err := s.store.LoadUI(); err == nil && saved != nil {
		u = *saved
		if u.Language == "" {
			u.Language = settings.DefaultLanguage
		}
	}
	return u
}

func (s *Server) getUI(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.currentUI())
}

// putUI updates only the fields present in the body, e.g. {"title":"Stugan"}
// keeps the language and {"language":"sv"} keeps the title.
func (s *Server) putUI(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Language *string `json:"language"`
		Title    *string `json:"title"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048)).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	u := s.currentUI()
	if in.Language != nil {
		if !languageRe.MatchString(*in.Language) {
			writeErr(w, http.StatusBadRequest, "invalid language")
			return
		}
		u.Language = *in.Language
	}
	if in.Title != nil {
		title := strings.TrimSpace(*in.Title)
		if utf8.RuneCountInString(title) > settings.MaxTitleLen {
			writeErr(w, http.StatusBadRequest, "title is too long")
			return
		}
		u.Title = title
	}
	if err := s.store.SaveUI(u); err != nil {
		writeErr(w, http.StatusInternalServerError, "could not save settings: "+err.Error())
		return
	}
	s.reg.Broadcast(devices.Event{Type: "ui", UI: &u})
	writeJSON(w, http.StatusOK, u)
}

// --- weather -------------------------------------------------------------------

type weatherSettingsView struct {
	settings.Weather
	Configured bool     `json:"configured"`
	Providers  []string `json:"providers"`
}

func (s *Server) currentWeather() (settings.Weather, bool) {
	if w, err := s.store.LoadWeather(); err == nil && w != nil && w.Provider != "" {
		return *w, true
	}
	return settings.Weather{Provider: weather.ProviderIDs[0]}, false
}

func (s *Server) getWeatherSettings(w http.ResponseWriter, _ *http.Request) {
	cur, ok := s.currentWeather()
	writeJSON(w, http.StatusOK, weatherSettingsView{Weather: cur, Configured: ok, Providers: weather.ProviderIDs})
}

func (s *Server) putWeatherSettings(w http.ResponseWriter, r *http.Request) {
	var in settings.Weather
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048)).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	in.Location = strings.TrimSpace(in.Location)
	switch {
	case !slices.Contains(weather.ProviderIDs, in.Provider):
		writeErr(w, http.StatusBadRequest, "unknown weather provider")
		return
	case in.Latitude < -90 || in.Latitude > 90 || in.Longitude < -180 || in.Longitude > 180:
		writeErr(w, http.StatusBadRequest, "latitude/longitude out of range")
		return
	case utf8.RuneCountInString(in.Location) > 100:
		writeErr(w, http.StatusBadRequest, "location name is too long")
		return
	}
	// Look the area name up now, once; the weather card just shows it.
	in.Area = ""
	if s.opts.Weather != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		area, err := s.opts.Weather.Area(ctx, in.Latitude, in.Longitude, s.currentUI().Language)
		cancel()
		if err != nil {
			s.log.Warn("area lookup failed", "err", err)
		}
		in.Area = area
	}
	if err := s.store.SaveWeather(in); err != nil {
		writeErr(w, http.StatusInternalServerError, "could not save settings: "+err.Error())
		return
	}
	s.reg.Broadcast(devices.Event{Type: "weather"}) // weather cards reload
	writeJSON(w, http.StatusOK, weatherSettingsView{Weather: in, Configured: true, Providers: weather.ProviderIDs})
}

func (s *Server) getWeather(w http.ResponseWriter, r *http.Request) {
	cur, ok := s.currentWeather()
	if !ok || s.opts.Weather == nil {
		writeErrCode(w, http.StatusNotFound, codeWeatherNotConfigured, "weather is not configured")
		return
	}
	f, err := s.opts.Weather.Forecast(r.Context(), weather.Query{
		Provider: cur.Provider, Latitude: cur.Latitude, Longitude: cur.Longitude, Location: cur.Location,
	})
	if err != nil {
		s.log.Warn("weather", "provider", cur.Provider, "err", err)
		writeErrCode(w, http.StatusBadGateway, codeWeatherUnavailable, err.Error())
		return
	}
	f.Area = cur.Area
	writeJSON(w, http.StatusOK, struct {
		weather.Forecast
		Sun sunView `json:"sun"`
	}{f, todaysSun(cur.Latitude, cur.Longitude)})
}

// sunView is today's sunrise/sunset for the weather card (RFC 3339, home time
// zone), or polar "day"/"night" when the sun doesn't rise or set today.
type sunView struct {
	Sunrise string `json:"sunrise,omitempty"`
	Sunset  string `json:"sunset,omitempty"`
	Polar   string `json:"polar,omitempty"`
}

func todaysSun(lat, lon float64) sunView {
	sun := schedule.SunOn(time.Now(), lat, lon)
	if sun.Polar != "" {
		return sunView{Polar: sun.Polar}
	}
	return sunView{Sunrise: sun.Rise.Local().Format(time.RFC3339), Sunset: sun.Set.Local().Format(time.RFC3339)}
}

// lookupArea names a position, used by Settings → "Use my position".
func (s *Server) lookupArea(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	lat, err1 := strconv.ParseFloat(q.Get("lat"), 64)
	lon, err2 := strconv.ParseFloat(q.Get("lon"), 64)
	if err1 != nil || err2 != nil || lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		writeErr(w, http.StatusBadRequest, "lat/lon missing or out of range")
		return
	}
	lang := q.Get("lang")
	if !languageRe.MatchString(lang) {
		lang = s.currentUI().Language
	}
	if s.opts.Weather == nil {
		writeErr(w, http.StatusServiceUnavailable, "area lookup is not available")
		return
	}
	area, err := s.opts.Weather.Area(r.Context(), lat, lon, lang)
	if err != nil {
		writeErrCode(w, http.StatusBadGateway, codeWeatherUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"area": area})
}

// locate gives the home's rough position from its internet connection; used
// when the browser can't share a precise position (plain http on the LAN).
func (s *Server) locate(w http.ResponseWriter, r *http.Request) {
	if s.opts.Weather == nil {
		writeErr(w, http.StatusServiceUnavailable, "positioning is not available")
		return
	}
	p, err := s.opts.Weather.Locate(r.Context())
	if err != nil {
		s.log.Warn("locate failed", "err", err)
		writeErrCode(w, http.StatusBadGateway, codeLocateFailed, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) searchPlaces(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if utf8.RuneCountInString(q) < 2 || utf8.RuneCountInString(q) > 100 {
		writeErr(w, http.StatusBadRequest, "search for at least 2 characters")
		return
	}
	lang := r.URL.Query().Get("lang")
	if !languageRe.MatchString(lang) {
		lang = "en"
	}
	if s.opts.Weather == nil {
		writeErr(w, http.StatusServiceUnavailable, "place search is not available")
		return
	}
	places, err := s.opts.Weather.Places(r.Context(), q, lang)
	if err != nil {
		writeErrCode(w, http.StatusBadGateway, codeWeatherUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, places)
}

// --- calendars -------------------------------------------------------------------

const (
	maxCalendars    = 10
	maxCalendarName = 40
	calendarDays    = 7 // today + 6
)

var colorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

type calendarView struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Color   string `json:"color"`
	URLHint string `json:"urlHint"` // e.g. "calendar.google.com/…/basic.ics"
}

// urlHint shows where a calendar comes from without its secret parts.
func urlHint(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "…"
	}
	last := path.Base(u.Path)
	if last == "." || last == "/" {
		return u.Host
	}
	if len(last) > 24 {
		last = "…" + last[len(last)-12:]
	}
	return u.Host + "/…/" + last
}

func calendarViews(cals []settings.Calendar) []calendarView {
	out := make([]calendarView, 0, len(cals))
	for _, c := range cals {
		out = append(out, calendarView{ID: c.ID, Name: c.Name, Color: c.Color, URLHint: urlHint(c.URL)})
	}
	return out
}

func validCalendarURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "webcal":
		return true
	}
	return false
}

func (s *Server) getCalendars(w http.ResponseWriter, _ *http.Request) {
	cals, err := s.store.LoadCalendars()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not read settings: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"calendars": calendarViews(cals)})
}

// putCalendars replaces the list. An existing calendar (by id) keeps its
// link when "url" is left out, so the secret link never has to be re-sent.
func (s *Server) putCalendars(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Calendars []struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Color string `json:"color"`
			URL   string `json:"url"`
		} `json:"calendars"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if len(in.Calendars) > maxCalendars {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("at most %d calendars", maxCalendars))
		return
	}
	existing, err := s.store.LoadCalendars()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not read settings: "+err.Error())
		return
	}
	byID := map[string]settings.Calendar{}
	for _, c := range existing {
		byID[c.ID] = c
	}
	out := make([]settings.Calendar, 0, len(in.Calendars))
	for _, c := range in.Calendars {
		cal := settings.Calendar{ID: c.ID, Name: strings.TrimSpace(c.Name), Color: c.Color, URL: strings.TrimSpace(c.URL)}
		if cal.ID != "" {
			old, ok := byID[cal.ID]
			if !ok {
				writeErr(w, http.StatusBadRequest, "unknown calendar id "+cal.ID)
				return
			}
			if cal.URL == "" {
				cal.URL = old.URL
			}
		} else {
			cal.ID = newCalendarID()
		}
		switch {
		case cal.Name == "" || utf8.RuneCountInString(cal.Name) > maxCalendarName:
			writeErr(w, http.StatusBadRequest, fmt.Sprintf("calendar name must be 1-%d characters", maxCalendarName))
			return
		case !colorRe.MatchString(cal.Color):
			writeErr(w, http.StatusBadRequest, "color must look like #f5c242")
			return
		case !validCalendarURL(cal.URL):
			writeErr(w, http.StatusBadRequest, "calendar link must be an http(s) or webcal address")
			return
		}
		out = append(out, cal)
	}
	if err := s.store.SaveCalendars(out); err != nil {
		writeErr(w, http.StatusInternalServerError, "could not save settings: "+err.Error())
		return
	}
	s.reg.Broadcast(devices.Event{Type: "calendar"}) // calendar cards reload
	writeJSON(w, http.StatusOK, map[string]any{"calendars": calendarViews(out)})
}

func (s *Server) testCalendar(w http.ResponseWriter, r *http.Request) {
	var in struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&in); err != nil || !validCalendarURL(in.URL) {
		writeErr(w, http.StatusBadRequest, "calendar link must be an http(s) or webcal address")
		return
	}
	if s.opts.Calendar == nil {
		writeErr(w, http.StatusServiceUnavailable, "calendars are not available")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	n, err := s.opts.Calendar.Test(ctx, strings.TrimSpace(in.URL))
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "events": n})
}

// getCalendar serves a week of events; ?from=YYYY-MM-DD picks the first day (default today).
func (s *Server) getCalendar(w http.ResponseWriter, r *http.Request) {
	var from time.Time
	if q := r.URL.Query().Get("from"); q != "" {
		d, err := time.Parse("2006-01-02", q)
		if err != nil || d.Year() < 1970 || d.Year() > 2200 {
			writeErr(w, http.StatusBadRequest, "from must be a date like 2026-10-28")
			return
		}
		from = d
	}
	cals, err := s.store.LoadCalendars()
	if err != nil || len(cals) == 0 || s.opts.Calendar == nil {
		writeErrCode(w, http.StatusNotFound, codeCalendarNotConfigured, "no calendars are set up")
		return
	}
	src := make([]calendar.Source, 0, len(cals))
	for _, c := range cals {
		src = append(src, calendar.Source{ID: c.ID, URL: c.URL, Color: c.Color})
	}
	writeJSON(w, http.StatusOK, s.opts.Calendar.Upcoming(r.Context(), src, from, calendarDays))
}

func newCalendarID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return "cal-" + hex.EncodeToString(b)
}

// --- dashboard ------------------------------------------------------------------

// loadDashboard reads the saved layout, upgraded to tabs (nil = none saved).
func (s *Server) loadDashboard() (*dashboard.Dashboard, error) {
	d, err := s.store.LoadDashboard()
	if err != nil || d == nil {
		return d, err
	}
	up := d.Upgrade()
	return &up, nil
}

func (s *Server) getDashboard(w http.ResponseWriter, _ *http.Request) {
	d, err := s.loadDashboard()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not read dashboard: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"dashboard": d})
}

func (s *Server) putDashboard(w http.ResponseWriter, r *http.Request) {
	var in dashboard.Dashboard
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	d, err := in.Normalize()
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.SaveDashboard(d); err != nil {
		s.log.Error("save dashboard", "err", err)
		writeErr(w, http.StatusInternalServerError, "could not save dashboard: "+err.Error())
		return
	}
	s.reg.Broadcast(devices.Event{Type: "dashboard", Dashboard: &d})
	writeJSON(w, http.StatusOK, map[string]any{"dashboard": d})
}

// --- MQTT settings -------------------------------------------------------------

// mqttSettingsView never includes the password itself.
type mqttSettingsView struct {
	URL         string             `json:"url"`
	Username    string             `json:"username"`
	HasPassword bool               `json:"hasPassword"`
	ClientID    string             `json:"clientId"`
	BaseTopic   string             `json:"baseTopic"`
	Source      string             `json:"source"` // "ui" | "config"
	Status      devices.MQTTStatus `json:"status"`
}

// mqttSettingsInput: Password nil = keep the current one, "" = no password.
type mqttSettingsInput struct {
	URL       string  `json:"url"`
	Username  string  `json:"username"`
	Password  *string `json:"password"`
	ClientID  string  `json:"clientId"`
	BaseTopic string  `json:"baseTopic"`
}

func (s *Server) view() mqttSettingsView {
	cfg, src := s.mqtt.Config()
	return mqttSettingsView{
		URL: cfg.URL, Username: cfg.Username, HasPassword: cfg.Password != "",
		ClientID: cfg.ClientID, BaseTopic: cfg.BaseTopic, Source: src, Status: s.mqtt.Status(),
	}
}

func (s *Server) getMQTTSettings(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.view())
}

func (s *Server) readMQTTInput(w http.ResponseWriter, r *http.Request) (config.MQTT, bool) {
	var in mqttSettingsInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return config.MQTT{}, false
	}
	cur, _ := s.mqtt.Config()
	cfg := config.MQTT{URL: in.URL, Username: in.Username, ClientID: in.ClientID, BaseTopic: in.BaseTopic}
	if in.Password != nil {
		cfg.Password = *in.Password
	} else {
		cfg.Password = cur.Password
	}
	cfg, err := cfg.Normalize()
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return config.MQTT{}, false
	}
	return cfg, true
}

func (s *Server) putMQTTSettings(w http.ResponseWriter, r *http.Request) {
	cfg, ok := s.readMQTTInput(w, r)
	if !ok {
		return
	}
	if err := s.store.SaveMQTT(cfg); err != nil {
		s.log.Error("save settings", "err", err)
		writeErr(w, http.StatusInternalServerError, "could not save settings: "+err.Error())
		return
	}
	s.log.Info("mqtt settings changed from UI", "url", cfg.URL, "baseTopic", cfg.BaseTopic)
	s.mqtt.Apply(cfg, "ui")
	writeJSON(w, http.StatusOK, s.view())
}

func (s *Server) testMQTTSettings(w http.ResponseWriter, r *http.Request) {
	cfg, ok := s.readMQTTInput(w, r)
	if !ok {
		return
	}
	if err := s.mqtt.Test(cfg); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error(), "url": cfg.URL})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "url": cfg.URL})
}

// --- WebSocket ------------------------------------------------------------------

// ws streams a snapshot (status + devices) followed by live events.
func (s *Server) ws(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: hostsOf(s.opts.CORSOrigins)})
	if err != nil {
		return
	}
	defer conn.CloseNow()

	events, cancel := s.reg.Subscribe()
	defer cancel()

	// We don't expect client messages, but reading handles pings/close.
	ctx := conn.CloseRead(r.Context())

	st := s.mqtt.Status()
	if write(ctx, conn, devices.Event{Type: "status", MQTT: &st}) != nil {
		return
	}
	ui := s.currentUI()
	if write(ctx, conn, devices.Event{Type: "ui", UI: &ui}) != nil {
		return
	}
	if write(ctx, conn, devices.Event{Type: "devices", Devices: s.reg.List()}) != nil {
		return
	}
	if d, err := s.loadDashboard(); err == nil {
		if write(ctx, conn, devices.Event{Type: "dashboard", Dashboard: d}) != nil {
			return
		}
	}
	ping := time.NewTicker(30 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-events:
			if !ok || write(ctx, conn, e) != nil {
				return
			}
		case <-ping.C:
			pctx, c := context.WithTimeout(ctx, 10*time.Second)
			err := conn.Ping(pctx)
			c()
			if err != nil {
				return
			}
		}
	}
}

func write(ctx context.Context, c *websocket.Conn, v any) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return wsjson.Write(ctx, c, v)
}

// --- static frontend -----------------------------------------------------------

// staticHandler serves the exported Next.js app (next build with
// output: "export"), falling back to its 404 page.
func staticHandler(dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	notFound := filepath.Join(dir, "404.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := filepath.Join(dir, filepath.FromSlash(filepath.Clean("/"+r.URL.Path)))
		if _, err := os.Stat(p); err != nil {
			if _, err := os.Stat(p + ".html"); err == nil {
				http.ServeFile(w, r, p+".html")
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			if b, err := os.ReadFile(notFound); err == nil {
				w.Write(b)
			}
			return
		}
		fs.ServeHTTP(w, r)
	})
}

// --- helpers ---------------------------------------------------------------------

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if o := r.Header.Get("Origin"); o != "" && slices.Contains(s.opts.CORSOrigins, o) {
			w.Header().Set("Access-Control-Allow-Origin", o)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Credentials", "true") // the login cookie
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// hostsOf turns "http://localhost:3000" into "localhost:3000" for the
// websocket origin check.
func hostsOf(origins []string) []string {
	out := make([]string, 0, len(origins))
	for _, o := range origins {
		if i := strings.Index(o, "://"); i >= 0 {
			o = o[i+3:]
		}
		out = append(out, strings.TrimSuffix(o, "/"))
	}
	return out
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// Error codes the frontend translates (frontend/locales/*.json "errors.<code>").
const (
	codeMQTTNotConnected = "mqtt_not_connected"
	codeDeviceNotFound   = "device_not_found"

	codeWeatherNotConfigured = "weather_not_configured"
	codeWeatherUnavailable   = "weather_unavailable"
	codeLocateFailed         = "locate_failed"

	codeCalendarNotConfigured = "calendar_not_configured"
	codeUnauthorized          = "unauthorized"
	codeBadLogin              = "bad_login"
	codeTooManyAttempts       = "too_many_attempts"
	codeBackupWeakPassword    = "backup_weak_password"
	codeBackupWrongPassword   = "backup_wrong_password"
	codeBackupNotBackup       = "backup_not_backup"
	codeBackupInvalid         = "backup_invalid"
	codeBackupNeedsPassword   = "backup_needs_password"
)

func writeErrCode(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]string{"error": msg, "code": code})
}

func writeMQTTErr(w http.ResponseWriter, err error) {
	if errors.Is(err, mqttclient.ErrNotConnected) {
		writeErrCode(w, http.StatusServiceUnavailable, codeMQTTNotConnected, err.Error())
		return
	}
	writeErr(w, http.StatusServiceUnavailable, err.Error())
}
