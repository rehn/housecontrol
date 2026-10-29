// Package schedule switches lights and outlets on and off at set times or
// relative to sunrise/sunset, optionally only on some weekdays.
package schedule

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	KindTime    = "time"    // a clock time, e.g. 07:00
	KindSunrise = "sunrise" // sunrise ± offset
	KindSunset  = "sunset"  // sunset ± offset

	ActionOn  = "on"
	ActionOff = "off"

	MaxSchedules  = 50
	MaxDevices    = 50
	MaxNameLen    = 40
	MaxOffsetMins = 240
)

// Trigger is when something happens on a given day.
type Trigger struct {
	Kind      string `json:"kind"`
	Time      string `json:"time,omitempty"`      // HH:MM, kind time
	Offset    int    `json:"offset,omitempty"`    // minutes, sun kinds (negative = before)
	NotBefore string `json:"notBefore,omitempty"` // HH:MM, sun kinds: not earlier than this
	NotAfter  string `json:"notAfter,omitempty"`  // HH:MM, sun kinds: not later than this
}

// Schedule turns DeviceIDs on (On) and/or off (Off).
type Schedule struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Enabled    bool     `json:"enabled"`
	DeviceIDs  []string `json:"deviceIds"`
	Days       []int    `json:"days,omitempty"` // 0 = Sunday … 6 = Saturday; empty = every day
	On         *Trigger `json:"on,omitempty"`
	Off        *Trigger `json:"off,omitempty"`
	Brightness int      `json:"brightness,omitempty"` // % when turning on; 0 = leave as is
}

// Place is where "sunrise" and "sunset" are computed, and the home's time zone.
type Place struct {
	Lat, Lon float64
	Loc      *time.Location
	Known    bool // false: no position saved, sun triggers never fire
}

// Event is one thing to do.
type Event struct {
	ScheduleID string
	Action     string // on / off
	At         time.Time
}

// Normalize validates s and returns a cleaned copy.
func (s Schedule) Normalize() (Schedule, error) {
	if strings.TrimSpace(s.ID) == "" {
		return s, errors.New("schedule without id")
	}
	s.Name = strings.TrimSpace(s.Name)
	if s.Name == "" || utf8.RuneCountInString(s.Name) > MaxNameLen {
		return s, fmt.Errorf("name must be 1-%d characters", MaxNameLen)
	}
	ids := make([]string, 0, len(s.DeviceIDs))
	for _, id := range s.DeviceIDs {
		if id != "" && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 || len(ids) > MaxDevices {
		return s, fmt.Errorf("%q: choose 1-%d lights or outlets", s.Name, MaxDevices)
	}
	s.DeviceIDs = ids
	var days []int
	for _, d := range s.Days {
		if d < 0 || d > 6 {
			return s, fmt.Errorf("%q: unknown weekday %d", s.Name, d)
		}
		if !slices.Contains(days, d) {
			days = append(days, d)
		}
	}
	sort.Ints(days)
	if len(days) == 7 {
		days = nil // every day
	}
	s.Days = days
	if s.On == nil && s.Off == nil {
		return s, fmt.Errorf("%q: set a time to turn on and/or off", s.Name)
	}
	for _, tr := range []*Trigger{s.On, s.Off} {
		if tr == nil {
			continue
		}
		if err := tr.normalize(); err != nil {
			return s, fmt.Errorf("%q: %w", s.Name, err)
		}
	}
	if s.Brightness < 0 || s.Brightness > 100 {
		return s, fmt.Errorf("%q: brightness must be 0-100 %%", s.Name)
	}
	return s, nil
}

func (t *Trigger) normalize() error {
	switch t.Kind {
	case KindTime:
		if _, ok := clock(t.Time); !ok {
			return errors.New("time must be HH:MM")
		}
		t.Offset, t.NotBefore, t.NotAfter = 0, "", ""
	case KindSunrise, KindSunset:
		t.Time = ""
		if t.Offset < -MaxOffsetMins || t.Offset > MaxOffsetMins {
			return fmt.Errorf("offset must be within ±%d minutes", MaxOffsetMins)
		}
		nb, okB := clock(t.NotBefore)
		na, okA := clock(t.NotAfter)
		if t.NotBefore != "" && !okB || t.NotAfter != "" && !okA {
			return errors.New("not before / not after must be HH:MM")
		}
		if okB && okA && nb > na {
			return errors.New("not before must be earlier than not after")
		}
	default:
		return fmt.Errorf("unknown trigger %q", t.Kind)
	}
	return nil
}

// clock parses "HH:MM" into minutes after midnight.
func clock(s string) (int, bool) {
	var h, m int
	if len(s) != 5 || s[2] != ':' {
		return 0, false
	}
	if _, err := fmt.Sscanf(s, "%02d:%02d", &h, &m); err != nil || h > 23 || m > 59 || h < 0 || m < 0 {
		return 0, false
	}
	return h*60 + m, true
}

// wall returns hh:mm on the given local day, following daylight saving.
func wall(day time.Time, minutes int) time.Time {
	y, mo, d := day.Date()
	return time.Date(y, mo, d, minutes/60, minutes%60, 0, 0, day.Location())
}

// on returns when the trigger fires on `day` (a local date), if it does.
func (t *Trigger) on(day time.Time, p Place) (time.Time, bool) {
	switch t.Kind {
	case KindTime:
		mins, ok := clock(t.Time)
		return wall(day, mins), ok
	case KindSunrise, KindSunset:
		if !p.Known {
			return time.Time{}, false
		}
		sun := SunOn(day, p.Lat, p.Lon)
		if sun.Polar != "" {
			return time.Time{}, false
		}
		at := sun.Rise
		if t.Kind == KindSunset {
			at = sun.Set
		}
		at = at.In(day.Location()).Add(time.Duration(t.Offset) * time.Minute).Truncate(time.Minute)
		if mins, ok := clock(t.NotBefore); ok && at.Before(wall(day, mins)) {
			at = wall(day, mins)
		}
		if mins, ok := clock(t.NotAfter); ok && at.After(wall(day, mins)) {
			at = wall(day, mins)
		}
		return at, true
	}
	return time.Time{}, false
}

func (s Schedule) runsOn(day time.Time) bool {
	return len(s.Days) == 0 || slices.Contains(s.Days, int(day.Weekday()))
}

// Next is the schedule's first event strictly after `after` (looking up to
// two weeks ahead). ok is false when nothing will happen.
func (s Schedule) Next(after time.Time, p Place) (Event, bool) {
	if !s.Enabled {
		return Event{}, false
	}
	loc := p.Loc
	if loc == nil {
		loc = time.Local
	}
	start := after.In(loc)
	for i := -1; i <= 14; i++ { // -1: an offset can push yesterday's event past midnight
		day := time.Date(start.Year(), start.Month(), start.Day()+i, 12, 0, 0, 0, loc)
		if !s.runsOn(day) {
			continue
		}
		var best Event
		found := false
		for _, c := range []struct {
			tr     *Trigger
			action string
		}{{s.On, ActionOn}, {s.Off, ActionOff}} {
			if c.tr == nil {
				continue
			}
			at, ok := c.tr.on(day, p)
			if !ok || !at.After(after) {
				continue
			}
			if !found || at.Before(best.At) {
				best, found = Event{ScheduleID: s.ID, Action: c.action, At: at}, true
			}
		}
		if found {
			return best, true
		}
	}
	return Event{}, false
}

// Due lists every event in (from, to], oldest first.
func Due(list []Schedule, from, to time.Time, p Place) []Event {
	var out []Event
	for _, s := range list {
		cursor := from
		for {
			ev, ok := s.Next(cursor, p)
			if !ok || ev.At.After(to) {
				break
			}
			out = append(out, ev)
			cursor = ev.At
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out
}
