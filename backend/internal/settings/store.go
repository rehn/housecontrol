// Package settings persists settings changed from the web UI as JSON in the
// data directory, so they survive restarts.
package settings

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"housecontrol/internal/config"
	"housecontrol/internal/dashboard"
	"housecontrol/internal/schedule"
)

type Store struct {
	path string
	mu   sync.Mutex
}

type file struct {
	MQTT      *config.MQTT         `json:"mqtt,omitempty"`
	Dashboard *dashboard.Dashboard `json:"dashboard,omitempty"`
	UI        *UI                  `json:"ui,omitempty"`
	Weather   *Weather             `json:"weather,omitempty"`
	Calendars []Calendar           `json:"calendars,omitempty"`
	Schedules []schedule.Schedule  `json:"schedules,omitempty"`
}

// Calendar is one iCal link shown on calendar cards. The URL often contains
// a secret token (Google's "secret address") and is never sent to browsers.
type Calendar struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	URL   string `json:"url"`
	Color string `json:"color"` // #rrggbb
}

// Weather is the forecast provider and location for weather cards.
type Weather struct {
	Provider  string  `json:"provider"` // "smhi" | "metno" | "openmeteo"
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Location  string  `json:"location"` // display name, e.g. "Enköping"
	// Area is looked up from the coordinates when the place is saved,
	// e.g. "Enköping, Uppsala län" (in the app language at that time).
	Area string `json:"area"`
}

// SchedulePlace is where schedules compute sunrise/sunset: the place saved
// for the weather cards, in time zone loc. Known is false (sun triggers never
// fire) until a place is saved. Also returns the place's display name.
func SchedulePlace(w *Weather, loc *time.Location) (schedule.Place, string) {
	p := schedule.Place{Loc: loc}
	if w == nil || (w.Latitude == 0 && w.Longitude == 0) {
		return p, ""
	}
	p.Lat, p.Lon, p.Known = w.Latitude, w.Longitude, true
	name := w.Location
	if name == "" {
		name = w.Area
	}
	return p, name
}

// UI holds app-wide display settings.
type UI struct {
	Language string `json:"language"` // e.g. "en", "sv"; matches frontend/locales/<language>.json
	Title    string `json:"title"`    // shown in the header; "" = default ("housecontrol")
}

// MaxTitleLen is the longest app title accepted (in characters).
const MaxTitleLen = 60

// DefaultLanguage is used until a language is chosen in Settings.
const DefaultLanguage = "en"

func NewStore(dataDir string) *Store {
	return &Store{path: filepath.Join(dataDir, "settings.json")}
}

func (s *Store) Path() string { return s.path }

// LoadMQTT returns the saved MQTT settings, or nil if none were saved.
func (s *Store) LoadMQTT() (*config.MQTT, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.read()
	if err != nil {
		return nil, err
	}
	return f.MQTT, nil
}

func (s *Store) SaveMQTT(m config.MQTT) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.read()
	if err != nil {
		f = file{} // corrupt file: overwrite
	}
	f.MQTT = &m
	return s.write(f)
}

// LoadDashboard returns the saved dashboard, or nil if none was saved.
func (s *Store) LoadDashboard() (*dashboard.Dashboard, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.read()
	if err != nil {
		return nil, err
	}
	return f.Dashboard, nil
}

func (s *Store) SaveDashboard(d dashboard.Dashboard) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.read()
	if err != nil {
		return err // don't overwrite (and lose) MQTT settings we couldn't read
	}
	f.Dashboard = &d
	return s.write(f)
}

// LoadUI returns the saved UI settings, or nil if none were saved.
func (s *Store) LoadUI() (*UI, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.read()
	if err != nil {
		return nil, err
	}
	return f.UI, nil
}

func (s *Store) SaveUI(u UI) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.read()
	if err != nil {
		return err
	}
	f.UI = &u
	return s.write(f)
}

// LoadWeather returns the saved weather settings, or nil if none were saved.
func (s *Store) LoadWeather() (*Weather, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.read()
	if err != nil {
		return nil, err
	}
	return f.Weather, nil
}

func (s *Store) SaveWeather(w Weather) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.read()
	if err != nil {
		return err
	}
	f.Weather = &w
	return s.write(f)
}

func (s *Store) LoadCalendars() ([]Calendar, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.read()
	if err != nil {
		return nil, err
	}
	return f.Calendars, nil
}

func (s *Store) SaveCalendars(c []Calendar) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.read()
	if err != nil {
		return err
	}
	f.Calendars = c
	return s.write(f)
}

func (s *Store) LoadSchedules() ([]schedule.Schedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.read()
	if err != nil {
		return nil, err
	}
	return f.Schedules, nil
}

func (s *Store) SaveSchedules(l []schedule.Schedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.read()
	if err != nil {
		return err
	}
	f.Schedules = l
	return s.write(f)
}

// Snapshot is everything saved from the UI (what a backup contains).
type Snapshot = file

// Snapshot returns all saved settings.
func (s *Store) Snapshot() (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read()
}

// Restore replaces all saved settings with snap (validate it first).
func (s *Store) Restore(snap Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.write(snap)
}

func (s *Store) read() (file, error) {
	var f file
	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return f, nil
	}
	if err != nil {
		return f, err
	}
	return f, json.Unmarshal(b, &f)
}

// write is atomic (temp file + rename) and owner-readable only, since the
// file contains the broker password.
func (s *Store) write(f file) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".settings-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.path)
}
