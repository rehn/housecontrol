package calendar

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

// Occurrence is one concrete happening of an event.
type Occurrence struct {
	Title    string
	Location string
	Start    time.Time
	End      time.Time
	AllDay   bool
}

// Expand returns all occurrences overlapping [from, to), sorted by start.
// It handles RRULE (DAILY/WEEKLY/MONTHLY/YEARLY with INTERVAL, COUNT, UNTIL,
// BYDAY, BYMONTHDAY, BYMONTH), EXDATE, moved occurrences (RECURRENCE-ID)
// and cancelled events.
func Expand(events []Event, from, to time.Time) []Occurrence {
	// occurrences replaced by a RECURRENCE-ID event, per UID
	replaced := map[string]map[int64]bool{}
	for _, e := range events {
		if !e.RecurrenceID.IsZero() {
			if replaced[e.UID] == nil {
				replaced[e.UID] = map[int64]bool{}
			}
			replaced[e.UID][e.RecurrenceID.Unix()] = true
		}
	}

	var out []Occurrence
	add := func(e Event, start time.Time) {
		end := start.Add(e.End.Sub(e.Start))
		if e.AllDay { // keep whole days across DST changes
			days := int(e.End.Sub(e.Start).Hours()/24 + 0.5)
			end = start.AddDate(0, 0, days)
		}
		if end.After(from) && start.Before(to) || (end.Equal(start) && !start.Before(from) && start.Before(to)) {
			out = append(out, Occurrence{Title: e.Summary, Location: e.Location, Start: start, End: end, AllDay: e.AllDay})
		}
	}

	for _, e := range events {
		if e.Cancelled {
			continue
		}
		if !e.RecurrenceID.IsZero() || e.RRule == "" {
			add(e, e.Start)
			continue
		}
		ex := map[int64]bool{}
		for _, t := range e.ExDates {
			ex[t.Unix()] = true
		}
		for _, start := range recurrences(e, to) {
			if ex[start.Unix()] || replaced[e.UID][start.Unix()] {
				continue
			}
			add(e, start)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	return out
}

type rule struct {
	freq       string
	interval   int
	count      int
	until      time.Time
	byDay      []weekdayNum
	byMonthDay []int
	byMonth    []int
}

type weekdayNum struct {
	n   int // 0 = every; 1 = first; -1 = last
	day time.Weekday
}

var weekdays = map[string]time.Weekday{
	"MO": time.Monday, "TU": time.Tuesday, "WE": time.Wednesday, "TH": time.Thursday,
	"FR": time.Friday, "SA": time.Saturday, "SU": time.Sunday,
}

func parseRule(s string, loc *time.Location) (rule, bool) {
	r := rule{interval: 1}
	for _, part := range strings.Split(s, ";") {
		k, v, _ := strings.Cut(part, "=")
		switch strings.ToUpper(k) {
		case "FREQ":
			r.freq = strings.ToUpper(v)
		case "INTERVAL":
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				r.interval = n
			}
		case "COUNT":
			r.count, _ = strconv.Atoi(v)
		case "UNTIL":
			if t, _, err := parseTime(prop{params: map[string]string{}, value: v}, loc); err == nil {
				if len(v) == 8 { // date only: until the end of that day
					t = t.AddDate(0, 0, 1).Add(-time.Second)
				}
				r.until = t
			}
		case "BYDAY":
			for _, d := range strings.Split(v, ",") {
				d = strings.ToUpper(strings.TrimSpace(d))
				if len(d) < 2 {
					continue
				}
				wd, ok := weekdays[d[len(d)-2:]]
				if !ok {
					continue
				}
				n, _ := strconv.Atoi(d[:len(d)-2])
				r.byDay = append(r.byDay, weekdayNum{n: n, day: wd})
			}
		case "BYMONTHDAY":
			r.byMonthDay = ints(v)
		case "BYMONTH":
			r.byMonth = ints(v)
		}
	}
	switch r.freq {
	case "DAILY", "WEEKLY", "MONTHLY", "YEARLY":
		return r, true
	}
	return r, false // HOURLY/MINUTELY etc.: not supported, shown once
}

func ints(v string) []int {
	var out []int
	for _, s := range strings.Split(v, ",") {
		if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			out = append(out, n)
		}
	}
	return out
}

// recurrences lists the start times of a recurring event up to `to`.
func recurrences(e Event, to time.Time) []time.Time {
	loc := e.Start.Location()
	r, ok := parseRule(e.RRule, loc)
	if !ok {
		return []time.Time{e.Start}
	}
	start := e.Start
	h, m, s := start.Clock()
	at := func(y int, mo time.Month, d int) (time.Time, bool) {
		t := time.Date(y, mo, d, h, m, s, 0, loc)
		return t, t.Day() == d && t.Month() == mo // false for e.g. 31 Nov
	}

	var out []time.Time
	n := 0
	emit := func(t time.Time) bool { // returns false to stop
		if t.Before(start) {
			return true
		}
		if !r.until.IsZero() && t.After(r.until) {
			return false
		}
		if r.count > 0 && n >= r.count {
			return false
		}
		if len(r.byMonth) > 0 && !containsInt(r.byMonth, int(t.Month())) {
			return true
		}
		n++
		if t.After(to) {
			return false
		}
		out = append(out, t)
		return true
	}

	const maxSteps = 10000
	switch r.freq {
	case "DAILY":
		for i := 0; i < maxSteps; i++ {
			t, _ := at(start.Year(), start.Month(), start.Day()+i*r.interval)
			if len(r.byDay) > 0 && !hasWeekday(r.byDay, t.Weekday()) {
				continue
			}
			if !emit(t) {
				return out
			}
		}
	case "WEEKLY":
		days := r.byDay
		if len(days) == 0 {
			days = []weekdayNum{{day: start.Weekday()}}
		}
		// Monday of the start week
		monday := start.AddDate(0, 0, -((int(start.Weekday()) + 6) % 7))
		for w := 0; w < maxSteps; w++ {
			base := monday.AddDate(0, 0, 7*w*r.interval)
			var week []time.Time
			for _, d := range days {
				off := (int(d.day) + 6) % 7
				t, _ := at(base.Year(), base.Month(), base.Day()+off)
				week = append(week, t)
			}
			sort.Slice(week, func(i, j int) bool { return week[i].Before(week[j]) })
			for _, t := range week {
				if !emit(t) {
					return out
				}
			}
		}
	case "MONTHLY":
		for i := 0; i < maxSteps; i++ {
			first := time.Date(start.Year(), start.Month()+time.Month(i*r.interval), 1, 0, 0, 0, 0, loc)
			for _, d := range monthDays(r, first, start.Day()) {
				t, ok := at(first.Year(), first.Month(), d)
				if ok && !emit(t) {
					return out
				}
			}
		}
	case "YEARLY":
		for i := 0; i < maxSteps; i++ {
			t, ok := at(start.Year()+i*r.interval, start.Month(), start.Day())
			if ok && !emit(t) {
				return out
			}
		}
	}
	return out
}

// monthDays returns the days of the month `first` that match the rule.
func monthDays(r rule, first time.Time, defaultDay int) []int {
	last := first.AddDate(0, 1, -1).Day()
	var days []int
	switch {
	case len(r.byMonthDay) > 0:
		for _, d := range r.byMonthDay {
			if d < 0 {
				d = last + 1 + d
			}
			if d >= 1 && d <= last {
				days = append(days, d)
			}
		}
	case len(r.byDay) > 0:
		for _, wd := range r.byDay {
			var matches []int
			for d := 1; d <= last; d++ {
				if time.Date(first.Year(), first.Month(), d, 12, 0, 0, 0, time.UTC).Weekday() == wd.day {
					matches = append(matches, d)
				}
			}
			switch {
			case wd.n == 0:
				days = append(days, matches...)
			case wd.n > 0 && wd.n <= len(matches):
				days = append(days, matches[wd.n-1])
			case wd.n < 0 && -wd.n <= len(matches):
				days = append(days, matches[len(matches)+wd.n])
			}
		}
	default:
		if defaultDay <= last {
			days = append(days, defaultDay)
		}
	}
	sort.Ints(days)
	return days
}

func hasWeekday(days []weekdayNum, wd time.Weekday) bool {
	for _, d := range days {
		if d.day == wd {
			return true
		}
	}
	return false
}

func containsInt(xs []int, x int) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
