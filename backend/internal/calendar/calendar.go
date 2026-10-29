package calendar

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// Tagged is an occurrence plus the calendar it came from.
type Tagged struct {
	Occurrence
	CalendarID string
	Color      string
}

// DayEvent is what the calendar card shows for one event on one day.
type DayEvent struct {
	Title      string    `json:"title"`
	Location   string    `json:"location,omitempty"`
	Start      time.Time `json:"start"`
	End        time.Time `json:"end"`
	AllDay     bool      `json:"allDay"`
	CalendarID string    `json:"calendarId"`
	Color      string    `json:"color"`
}

type Day struct {
	Date   string     `json:"date"` // YYYY-MM-DD, local
	Events []DayEvent `json:"events"`
}

// GroupByDay puts occurrences on every local day they touch, starting at
// `from` for `days` days. Days without events are left out. All-day events
// come first, then by start time.
func GroupByDay(occ []Tagged, from time.Time, days int, loc *time.Location) []Day {
	from = from.In(loc)
	day0 := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, loc)
	var out []Day
	for i := 0; i < days; i++ {
		start := day0.AddDate(0, 0, i)
		end := day0.AddDate(0, 0, i+1)
		var evs []DayEvent
		for _, o := range occ {
			touches := o.Start.Before(end) && o.End.After(start)
			if o.End.Equal(o.Start) { // zero-length event
				touches = !o.Start.Before(start) && o.Start.Before(end)
			}
			if !touches {
				continue
			}
			evs = append(evs, DayEvent{
				Title: o.Title, Location: o.Location, Start: o.Start.In(loc), End: o.End.In(loc),
				AllDay: o.AllDay, CalendarID: o.CalendarID, Color: o.Color,
			})
		}
		if len(evs) == 0 {
			continue
		}
		sort.SliceStable(evs, func(a, b int) bool {
			if evs[a].AllDay != evs[b].AllDay {
				return evs[a].AllDay
			}
			return evs[a].Start.Before(evs[b].Start)
		})
		out = append(out, Day{Date: start.Format("2006-01-02"), Events: evs})
	}
	return out
}

// --- fetching ----------------------------------------------------------------------

// Source is one calendar to show.
type Source struct {
	ID    string
	URL   string
	Color string
}

type CalError struct {
	CalendarID string `json:"calendarId"`
	Error      string `json:"error"`
}

type Result struct {
	From   string     `json:"from"`  // first day shown, YYYY-MM-DD
	To     string     `json:"to"`    // last day shown (inclusive)
	Today  string     `json:"today"` // the home's today, so the card knows where "today" is
	Days   []Day      `json:"days"`
	Errors []CalError `json:"errors,omitempty"`
}

type cached struct {
	events  []Event
	fetched time.Time
}

// Service downloads calendars (cached 15 min; on errors the last copy is
// used for up to a day) and returns upcoming days.
type Service struct {
	client   *http.Client
	loc      *time.Location
	ttl      time.Duration
	maxStale time.Duration
	now      func() time.Time

	mu    sync.Mutex
	cache map[string]cached
}

func NewService(client *http.Client, loc *time.Location) *Service {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	if loc == nil {
		loc = time.Local
	}
	return &Service{client: client, loc: loc, ttl: 15 * time.Minute, maxStale: 24 * time.Hour,
		now: time.Now, cache: map[string]cached{}}
}

// Upcoming returns `days` days for all sources, starting at `start`
// (only its date is used) or today when `start` is zero.
func (s *Service) Upcoming(ctx context.Context, sources []Source, start time.Time, days int) Result {
	now := s.now().In(s.loc)
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.loc)
	if !start.IsZero() { // only the date counts, read in the home's time zone
		from = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, s.loc)
	}
	to := from.AddDate(0, 0, days)

	type one struct {
		events []Event
		err    error
	}
	results := make([]one, len(sources))
	var wg sync.WaitGroup
	for i, src := range sources {
		wg.Add(1)
		go func(i int, src Source) {
			defer wg.Done()
			evs, err := s.events(ctx, src.URL, now)
			results[i] = one{evs, err}
		}(i, src)
	}
	wg.Wait()

	res := Result{
		From:  from.Format("2006-01-02"),
		To:    from.AddDate(0, 0, days-1).Format("2006-01-02"),
		Today: now.Format("2006-01-02"),
		Days:  []Day{},
	}
	var all []Tagged
	for i, src := range sources {
		if results[i].err != nil {
			res.Errors = append(res.Errors, CalError{CalendarID: src.ID, Error: results[i].err.Error()})
			continue
		}
		for _, o := range Expand(results[i].events, from, to) {
			all = append(all, Tagged{Occurrence: o, CalendarID: src.ID, Color: src.Color})
		}
	}
	if days := GroupByDay(all, from, days, s.loc); days != nil {
		res.Days = days
	}
	return res
}

// Test downloads and parses a calendar link without caching it.
func (s *Service) Test(ctx context.Context, url string) (int, error) {
	evs, err := s.fetch(ctx, url)
	return len(evs), err
}

func (s *Service) events(ctx context.Context, url string, now time.Time) ([]Event, error) {
	s.mu.Lock()
	c, hit := s.cache[url]
	s.mu.Unlock()
	if hit && now.Sub(c.fetched) < s.ttl {
		return c.events, nil
	}
	evs, err := s.fetch(ctx, url)
	if err != nil {
		if hit && now.Sub(c.fetched) < s.maxStale {
			return c.events, nil
		}
		return nil, err
	}
	s.mu.Lock()
	s.cache[url] = cached{events: evs, fetched: now}
	s.mu.Unlock()
	return evs, nil
}

func (s *Service) fetch(ctx context.Context, url string) ([]Event, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, httpURL(url), nil)
	if err != nil {
		return nil, fmt.Errorf("bad calendar link: %w", err)
	}
	req.Header.Set("User-Agent", "housecontrol/0.1 (self-hosted home automation)")
	req.Header.Set("Accept", "text/calendar, */*")
	res, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("calendar server answered %s", res.Status)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 20<<20))
	if err != nil {
		return nil, err
	}
	return Parse(bytes.NewReader(body), s.loc)
}

// httpURL turns webcal:// (iCloud, Outlook) into https://.
func httpURL(u string) string {
	u = strings.TrimSpace(u)
	if strings.HasPrefix(strings.ToLower(u), "webcal://") {
		return "https://" + u[len("webcal://"):]
	}
	return u
}
