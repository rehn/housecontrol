// Package calendar reads iCal (.ics) calendars, e.g. Google Calendar's
// "secret address in iCal format", iCloud or Outlook links, expands
// recurring events and groups them per day for the calendar card.
package calendar

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// Event is one VEVENT from a calendar file.
type Event struct {
	UID      string
	Summary  string
	Location string
	Start    time.Time
	End      time.Time
	AllDay   bool
	RRule    string
	ExDates  []time.Time
	// RecurrenceID is set on an event that replaces one occurrence of a
	// recurring event (same UID); zero otherwise.
	RecurrenceID time.Time
	Cancelled    bool
}

type prop struct {
	name   string
	params map[string]string
	value  string
}

// Parse reads all events of an iCal stream. Times without a time zone
// ("floating") and unknown time zones use loc.
func Parse(r io.Reader, loc *time.Location) ([]Event, error) {
	lines, err := unfold(r)
	if err != nil {
		return nil, err
	}
	var (
		events  []Event
		cur     *Event
		inCal   bool
		depth   int // nested components inside a VEVENT (VALARM)
		hasDur  bool
		dur     time.Duration
		hasEnd  bool
		calZone = loc
	)
	for _, line := range lines {
		p, ok := parseLine(line)
		if !ok {
			continue
		}
		switch {
		case p.name == "BEGIN" && strings.EqualFold(p.value, "VCALENDAR"):
			inCal = true
		case p.name == "X-WR-TIMEZONE":
			if l := lookupZone(p.value); l != nil {
				calZone = l
			}
		case p.name == "BEGIN" && strings.EqualFold(p.value, "VEVENT"):
			cur, hasDur, hasEnd, dur = &Event{}, false, false, 0
		case cur != nil && p.name == "BEGIN":
			depth++
		case cur != nil && p.name == "END" && depth > 0:
			depth--
		case cur != nil && p.name == "END" && strings.EqualFold(p.value, "VEVENT"):
			if !cur.Start.IsZero() {
				if !hasEnd {
					switch {
					case hasDur:
						cur.End = cur.Start.Add(dur)
					case cur.AllDay:
						cur.End = cur.Start.AddDate(0, 0, 1)
					default:
						cur.End = cur.Start
					}
				}
				events = append(events, *cur)
			}
			cur = nil
		case cur != nil && depth == 0:
			switch p.name {
			case "UID":
				cur.UID = p.value
			case "SUMMARY":
				cur.Summary = unescape(p.value)
			case "LOCATION":
				cur.Location = unescape(p.value)
			case "STATUS":
				cur.Cancelled = strings.EqualFold(p.value, "CANCELLED")
			case "DTSTART":
				t, allDay, err := parseTime(p, calZone)
				if err == nil {
					cur.Start, cur.AllDay = t, allDay
				}
			case "DTEND":
				if t, _, err := parseTime(p, calZone); err == nil {
					cur.End, hasEnd = t, true
				}
			case "DURATION":
				if d, err := parseDuration(p.value); err == nil {
					dur, hasDur = d, true
				}
			case "RRULE":
				cur.RRule = p.value
			case "EXDATE":
				for _, v := range strings.Split(p.value, ",") {
					q := p
					q.value = v
					if t, _, err := parseTime(q, calZone); err == nil {
						cur.ExDates = append(cur.ExDates, t)
					}
				}
			case "RECURRENCE-ID":
				if t, _, err := parseTime(p, calZone); err == nil {
					cur.RecurrenceID = t
				}
			}
		}
	}
	if !inCal {
		return nil, errors.New("not an iCal calendar (check the link)")
	}
	return events, nil
}

// unfold joins folded lines (continuation lines start with space or tab).
func unfold(r io.Reader) ([]string, error) {
	sc := bufio.NewScanner(io.LimitReader(r, 20<<20))
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	var out []string
	for sc.Scan() {
		l := strings.TrimRight(sc.Text(), "\r")
		if (strings.HasPrefix(l, " ") || strings.HasPrefix(l, "\t")) && len(out) > 0 {
			out[len(out)-1] += l[1:]
			continue
		}
		out = append(out, l)
	}
	return out, sc.Err()
}

// parseLine splits `NAME;P1=a;P2="b:c":value`.
func parseLine(line string) (prop, bool) {
	inQuote := false
	colon := -1
	for i, c := range line {
		if c == '"' {
			inQuote = !inQuote
		} else if c == ':' && !inQuote {
			colon = i
			break
		}
	}
	if colon <= 0 {
		return prop{}, false
	}
	head, value := line[:colon], line[colon+1:]
	parts := strings.Split(head, ";")
	p := prop{name: strings.ToUpper(parts[0]), params: map[string]string{}, value: value}
	for _, kv := range parts[1:] {
		if k, v, ok := strings.Cut(kv, "="); ok {
			p.params[strings.ToUpper(k)] = strings.Trim(v, `"`)
		}
	}
	return p, true
}

func unescape(s string) string {
	r := strings.NewReplacer(`\\`, `\`, `\,`, ",", `\;`, ";", `\n`, "\n", `\N`, "\n")
	return r.Replace(s)
}

// parseTime handles DATE (all-day), UTC (…Z), TZID and floating times.
func parseTime(p prop, def *time.Location) (time.Time, bool, error) {
	v := strings.TrimSpace(p.value)
	if strings.EqualFold(p.params["VALUE"], "DATE") || len(v) == 8 {
		t, err := time.ParseInLocation("20060102", v, def)
		return t, true, err
	}
	if strings.HasSuffix(v, "Z") {
		t, err := time.Parse("20060102T150405Z", v)
		return t, false, err
	}
	loc := def
	if tz := p.params["TZID"]; tz != "" {
		if l := lookupZone(tz); l != nil {
			loc = l
		}
	}
	t, err := time.ParseInLocation("20060102T150405", v, loc)
	return t, false, err
}

// Windows time zone names used by Outlook/Exchange.
var windowsZones = map[string]string{
	"W. Europe Standard Time":        "Europe/Berlin",
	"Central Europe Standard Time":   "Europe/Budapest",
	"Central European Standard Time": "Europe/Warsaw",
	"Romance Standard Time":          "Europe/Paris",
	"GMT Standard Time":              "Europe/London",
	"Greenwich Standard Time":        "Atlantic/Reykjavik",
	"FLE Standard Time":              "Europe/Helsinki",
	"E. Europe Standard Time":        "Europe/Chisinau",
	"Russian Standard Time":          "Europe/Moscow",
	"Eastern Standard Time":          "America/New_York",
	"Central Standard Time":          "America/Chicago",
	"Mountain Standard Time":         "America/Denver",
	"Pacific Standard Time":          "America/Los_Angeles",
	"UTC":                            "UTC",
	"Coordinated Universal Time":     "UTC",
	"(UTC+01:00) Amsterdam, Berlin, Bern, Rome, Stockholm, Vienna": "Europe/Stockholm",
}

func lookupZone(name string) *time.Location {
	name = strings.TrimSpace(strings.Trim(name, `"`))
	if w, ok := windowsZones[name]; ok {
		name = w
	}
	if l, err := time.LoadLocation(name); err == nil {
		return l
	}
	// e.g. "/mozilla.org/20050126_1/Europe/Stockholm"
	parts := strings.Split(name, "/")
	if len(parts) >= 2 {
		if l, err := time.LoadLocation(strings.Join(parts[len(parts)-2:], "/")); err == nil {
			return l
		}
	}
	return nil
}

// parseDuration handles ISO 8601 durations like PT1H30M, P1D, P1W, -PT15M.
func parseDuration(s string) (time.Duration, error) {
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimLeft(s, "+-")
	if !strings.HasPrefix(s, "P") {
		return 0, fmt.Errorf("bad duration %q", s)
	}
	var d time.Duration
	num := ""
	inTime := false
	for _, c := range s[1:] {
		switch {
		case c >= '0' && c <= '9':
			num += string(c)
		case c == 'T':
			inTime = true
		default:
			n, err := strconv.Atoi(num)
			if err != nil {
				return 0, fmt.Errorf("bad duration %q", s)
			}
			num = ""
			switch {
			case c == 'W':
				d += time.Duration(n) * 7 * 24 * time.Hour
			case c == 'D':
				d += time.Duration(n) * 24 * time.Hour
			case c == 'H' && inTime:
				d += time.Duration(n) * time.Hour
			case c == 'M' && inTime:
				d += time.Duration(n) * time.Minute
			case c == 'S' && inTime:
				d += time.Duration(n) * time.Second
			default:
				return 0, fmt.Errorf("bad duration %q", s)
			}
		}
	}
	if neg {
		d = -d
	}
	return d, nil
}
