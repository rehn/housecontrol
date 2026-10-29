package calendar

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var sthlm = mustLoc("Europe/Stockholm")

func mustLoc(n string) *time.Location {
	l, err := time.LoadLocation(n)
	if err != nil {
		panic(err)
	}
	return l
}

// Window: Wednesday 28 Oct 2026 00:00 (CET) + 7 days.
var (
	from = time.Date(2026, 10, 28, 0, 0, 0, 0, sthlm)
	to   = from.AddDate(0, 0, 7)
)

func ics(events ...string) string {
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//test//EN\r\n" +
		strings.Join(events, "") + "END:VCALENDAR\r\n"
}

func vevent(lines ...string) string {
	return "BEGIN:VEVENT\r\n" + strings.Join(lines, "\r\n") + "\r\nEND:VEVENT\r\n"
}

func parse(t *testing.T, s string) []Event {
	t.Helper()
	evs, err := Parse(strings.NewReader(s), sthlm)
	if err != nil {
		t.Fatal(err)
	}
	return evs
}

func starts(occ []Occurrence) []string {
	out := make([]string, len(occ))
	for i, o := range occ {
		out[i] = o.Start.In(sthlm).Format("Mon 01-02 15:04")
	}
	return out
}

func eq(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, " | ") != strings.Join(want, " | ") {
		t.Fatalf("\ngot  %v\nwant %v", got, want)
	}
}

// --- parsing ----------------------------------------------------------------------

func TestParseBasics(t *testing.T) {
	evs := parse(t, ics(vevent(
		"UID:1",
		"SUMMARY:Träning\\, Enköpings IBK \\; F 12-14",
		" (lång rad som vikts)",
		"LOCATION:Bredsandshallen\\nHall 2",
		"DTSTART:20261028T160000Z",
		"DTEND:20261028T173000Z",
	)))
	if len(evs) != 1 {
		t.Fatalf("%d events", len(evs))
	}
	e := evs[0]
	if e.Summary != "Träning, Enköpings IBK ; F 12-14(lång rad som vikts)" {
		t.Errorf("summary %q", e.Summary)
	}
	if e.Location != "Bredsandshallen\nHall 2" {
		t.Errorf("location %q", e.Location)
	}
	if got := e.Start.In(sthlm).Format("15:04"); got != "17:00" || e.End.Sub(e.Start) != 90*time.Minute || e.AllDay {
		t.Errorf("times %v %v", e.Start, e.End)
	}
}

func TestParseAllDayAndDuration(t *testing.T) {
	evs := parse(t, ics(
		vevent("UID:a", "SUMMARY:Höstlov", "DTSTART;VALUE=DATE:20261102", "DTEND;VALUE=DATE:20261107"),
		vevent("UID:b", "SUMMARY:Möte", "DTSTART;TZID=Europe/Stockholm:20261029T090000", "DURATION:PT1H30M"),
		vevent("UID:c", "SUMMARY:Utan slut", "DTSTART;VALUE=DATE:20261030"),
	))
	if !evs[0].AllDay || evs[0].Start.Format("2006-01-02 15:04") != "2026-11-02 00:00" || evs[0].End.Format("2006-01-02") != "2026-11-07" {
		t.Errorf("all-day: %+v", evs[0])
	}
	if evs[1].End.Sub(evs[1].Start) != 90*time.Minute {
		t.Errorf("duration: %v", evs[1].End.Sub(evs[1].Start))
	}
	if !evs[2].AllDay || evs[2].End.Sub(evs[2].Start) != 24*time.Hour {
		t.Errorf("all-day without end lasts one day: %+v", evs[2])
	}
}

func TestParseTimeZones(t *testing.T) {
	evs := parse(t, ics(
		vevent("UID:1", "SUMMARY:A", `DTSTART;TZID="Europe/Stockholm":20261028T170000`, "DTEND;TZID=Europe/Stockholm:20261028T180000"),
		vevent("UID:2", "SUMMARY:Outlook", "DTSTART;TZID=W. Europe Standard Time:20261028T170000", "DTEND;TZID=W. Europe Standard Time:20261028T180000"),
		vevent("UID:3", "SUMMARY:Floating", "DTSTART:20261028T170000", "DTEND:20261028T180000"),
		vevent("UID:4", "SUMMARY:Unknown zone", "DTSTART;TZID=Mars/Olympus:20261028T170000", "DTEND;TZID=Mars/Olympus:20261028T180000"),
	))
	for _, e := range evs {
		if got := e.Start.In(sthlm).Format("15:04"); got != "17:00" {
			t.Errorf("%s: starts %s local, want 17:00", e.Summary, got)
		}
	}
}

func TestParseRejectsNonCalendar(t *testing.T) {
	if _, err := Parse(strings.NewReader("<html>login</html>"), sthlm); err == nil {
		t.Fatal("an HTML page (e.g. wrong link) must be an error")
	}
}

// --- recurrence -------------------------------------------------------------------

func TestWeeklyKeepsLocalTimeOverDST(t *testing.T) {
	// started in September (summer time), still 17:00 after the switch on 25 Oct
	occ := Expand(parse(t, ics(vevent("UID:1", "SUMMARY:Träning",
		"DTSTART;TZID=Europe/Stockholm:20260902T170000", "DTEND;TZID=Europe/Stockholm:20260902T183000",
		"RRULE:FREQ=WEEKLY;BYDAY=WE,TH"))), from, to)
	eq(t, starts(occ), []string{"Wed 10-28 17:00", "Thu 10-29 17:00", "Wed 11-04 17:00"}[:2])
	if occ[0].End.Sub(occ[0].Start) != 90*time.Minute {
		t.Errorf("duration %v", occ[0].End.Sub(occ[0].Start))
	}
}

func TestWeeklyIntervalCountUntil(t *testing.T) {
	evs := parse(t, ics(
		vevent("UID:i2", "SUMMARY:Varannan måndag", "DTSTART;TZID=Europe/Stockholm:20261019T180000",
			"DTEND;TZID=Europe/Stockholm:20261019T190000", "RRULE:FREQ=WEEKLY;INTERVAL=2"),
		vevent("UID:count", "SUMMARY:Tre gånger", "DTSTART;TZID=Europe/Stockholm:20261026T200000",
			"DTEND;TZID=Europe/Stockholm:20261026T210000", "RRULE:FREQ=DAILY;COUNT=3"),
		vevent("UID:until", "SUMMARY:Till fredag", "DTSTART;TZID=Europe/Stockholm:20261027T070000",
			"DTEND;TZID=Europe/Stockholm:20261027T080000", "RRULE:FREQ=DAILY;UNTIL=20261030T060000Z"),
	))
	byTitle := func(title string) []string {
		var out []Occurrence
		for _, o := range Expand(evs, from, to) {
			if o.Title == title {
				out = append(out, o)
			}
		}
		return starts(out)
	}
	eq(t, byTitle("Varannan måndag"), []string{"Mon 11-02 18:00"})
	eq(t, byTitle("Tre gånger"), []string{"Wed 10-28 20:00"}) // 26, 27, 28
	eq(t, byTitle("Till fredag"), []string{"Wed 10-28 07:00", "Thu 10-29 07:00", "Fri 10-30 07:00"})
}

func TestMonthlyAndYearly(t *testing.T) {
	wide := to.AddDate(0, 3, 0)
	evs := parse(t, ics(
		vevent("UID:lastfri", "SUMMARY:Sista fredagen", "DTSTART;TZID=Europe/Stockholm:20260925T160000",
			"DTEND;TZID=Europe/Stockholm:20260925T170000", "RRULE:FREQ=MONTHLY;BYDAY=-1FR;COUNT=4"),
		vevent("UID:15", "SUMMARY:Den 15:e", "DTSTART;TZID=Europe/Stockholm:20261015T120000",
			"DTEND;TZID=Europe/Stockholm:20261015T130000", "RRULE:FREQ=MONTHLY;BYMONTHDAY=15"),
		vevent("UID:31", "SUMMARY:Den 31:a", "DTSTART;TZID=Europe/Stockholm:20261031T120000",
			"DTEND;TZID=Europe/Stockholm:20261031T130000", "RRULE:FREQ=MONTHLY"),
		vevent("UID:bday", "SUMMARY:Molly fyller år", "DTSTART;VALUE=DATE:20141102", "DTEND;VALUE=DATE:20141103",
			"RRULE:FREQ=YEARLY"),
	))
	got := map[string][]string{}
	for _, o := range Expand(evs, from, wide) {
		got[o.Title] = append(got[o.Title], o.Start.In(sthlm).Format("01-02"))
	}
	eq(t, got["Sista fredagen"], []string{"10-30", "11-27", "12-25"}) // 4th (Sep) was before the window
	eq(t, got["Den 15:e"], []string{"11-15", "12-15", "01-15"})
	eq(t, got["Den 31:a"], []string{"10-31", "12-31", "01-31"}) // months without a 31st are skipped
	eq(t, got["Molly fyller år"], []string{"11-02"})
}

func TestExceptionsMovedAndCancelled(t *testing.T) {
	evs := parse(t, ics(
		vevent("UID:t", "SUMMARY:Träning", "DTSTART;TZID=Europe/Stockholm:20261005T170000",
			"DTEND;TZID=Europe/Stockholm:20261005T180000", "RRULE:FREQ=DAILY",
			"EXDATE;TZID=Europe/Stockholm:20261029T170000,20261030T170000"),
		// the 31st moved to 10:00
		vevent("UID:t", "SUMMARY:Träning (flyttad)", "RECURRENCE-ID;TZID=Europe/Stockholm:20261031T170000",
			"DTSTART;TZID=Europe/Stockholm:20261031T100000", "DTEND;TZID=Europe/Stockholm:20261031T110000"),
		// the 1st cancelled
		vevent("UID:t", "SUMMARY:Träning", "STATUS:CANCELLED", "RECURRENCE-ID;TZID=Europe/Stockholm:20261101T170000",
			"DTSTART;TZID=Europe/Stockholm:20261101T170000", "DTEND;TZID=Europe/Stockholm:20261101T180000"),
		vevent("UID:x", "SUMMARY:Inställd", "STATUS:CANCELLED", "DTSTART;TZID=Europe/Stockholm:20261028T090000",
			"DTEND;TZID=Europe/Stockholm:20261028T100000"),
	))
	var got []string
	for _, o := range Expand(evs, from, to) {
		got = append(got, o.Start.In(sthlm).Format("01-02 15:04 ")+o.Title)
	}
	eq(t, got, []string{
		"10-28 17:00 Träning",
		"10-31 10:00 Träning (flyttad)",
		"11-02 17:00 Träning",
		"11-03 17:00 Träning",
	})
}

func TestOngoingEventStartedBeforeWindow(t *testing.T) {
	evs := parse(t, ics(vevent("UID:v", "SUMMARY:Höstlov", "DTSTART;VALUE=DATE:20261026", "DTEND;VALUE=DATE:20261031")))
	occ := Expand(evs, from, to)
	if len(occ) != 1 {
		t.Fatalf("an event already running must be included: %v", starts(occ))
	}
}

// --- days -------------------------------------------------------------------------

func TestGroupByDay(t *testing.T) {
	evs := parse(t, ics(
		vevent("UID:1", "SUMMARY:Kväll", "DTSTART;TZID=Europe/Stockholm:20261028T200000", "DTEND;TZID=Europe/Stockholm:20261028T220000"),
		vevent("UID:2", "SUMMARY:Morgon", "DTSTART;TZID=Europe/Stockholm:20261028T070000", "DTEND;TZID=Europe/Stockholm:20261028T080000"),
		vevent("UID:3", "SUMMARY:Lov", "DTSTART;VALUE=DATE:20261028", "DTEND;VALUE=DATE:20261030"),
		vevent("UID:4", "SUMMARY:Till midnatt", "DTSTART;TZID=Europe/Stockholm:20261102T220000", "DTEND;TZID=Europe/Stockholm:20261103T000000"),
	))
	var tagged []Tagged
	for _, o := range Expand(evs, from, to) {
		tagged = append(tagged, Tagged{Occurrence: o, CalendarID: "c1", Color: "#f5c242"})
	}
	days := GroupByDay(tagged, from, 7, sthlm)

	var got []string
	for _, d := range days {
		var titles []string
		for _, e := range d.Events {
			titles = append(titles, e.Title)
		}
		got = append(got, d.Date+": "+strings.Join(titles, ", "))
	}
	eq(t, got, []string{
		"2026-10-28: Lov, Morgon, Kväll", // all-day first, then by time
		"2026-10-29: Lov",
		"2026-11-02: Till midnatt", // ends at 00:00: not on the 3rd
	})
	if days[0].Events[0].Color != "#f5c242" || days[0].Events[0].CalendarID != "c1" || !days[0].Events[0].AllDay {
		t.Errorf("%+v", days[0].Events[0])
	}
}

// --- fetching ---------------------------------------------------------------------

func TestServiceFetchesCachesAndServesStale(t *testing.T) {
	var hits atomic.Int32
	fail := atomic.Bool{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if fail.Load() {
			http.Error(w, "down", http.StatusBadGateway)
			return
		}
		if !strings.Contains(r.Header.Get("User-Agent"), "housecontrol") {
			t.Error("missing User-Agent")
		}
		w.Write([]byte(ics(vevent("UID:1", "SUMMARY:Träning", "DTSTART;TZID=Europe/Stockholm:20261028T170000",
			"DTEND;TZID=Europe/Stockholm:20261028T180000"))))
	}))
	defer srv.Close()

	clock := time.Date(2026, 10, 28, 12, 0, 0, 0, sthlm)
	s := NewService(srv.Client(), sthlm)
	s.now = func() time.Time { return clock }
	src := []Source{{ID: "c1", URL: srv.URL + "/basic.ics", Color: "#f00"}}

	res := s.Upcoming(context.Background(), src, time.Time{}, 7)
	if len(res.Days) != 1 || res.Days[0].Events[0].Title != "Träning" || len(res.Errors) != 0 {
		t.Fatalf("%+v", res)
	}
	s.Upcoming(context.Background(), src, time.Time{}, 7)
	if hits.Load() != 1 {
		t.Fatalf("should be cached, hits=%d", hits.Load())
	}

	clock = clock.Add(20 * time.Minute)
	fail.Store(true)
	res = s.Upcoming(context.Background(), src, time.Time{}, 7)
	if hits.Load() != 2 || len(res.Days) != 1 || len(res.Errors) != 0 {
		t.Fatalf("a failing refresh should keep the last calendar: hits=%d %+v", hits.Load(), res)
	}

	s2 := NewService(srv.Client(), sthlm)
	res = s2.Upcoming(context.Background(), src, time.Time{}, 7)
	if len(res.Errors) != 1 || res.Errors[0].CalendarID != "c1" {
		t.Fatalf("errors must be reported per calendar: %+v", res)
	}
}

func TestServiceShowsAnyPeriod(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(ics(
			vevent("UID:1", "SUMMARY:Förra veckan", "DTSTART;TZID=Europe/Stockholm:20261022T170000",
				"DTEND;TZID=Europe/Stockholm:20261022T180000"),
			vevent("UID:2", "SUMMARY:Träning", "DTSTART;TZID=Europe/Stockholm:20261028T170000",
				"DTEND;TZID=Europe/Stockholm:20261028T180000", "RRULE:FREQ=WEEKLY"),
		)))
	}))
	defer srv.Close()
	s := NewService(srv.Client(), sthlm)
	s.now = func() time.Time { return time.Date(2026, 10, 28, 12, 0, 0, 0, sthlm) }
	src := []Source{{ID: "c1", URL: srv.URL, Color: "#f00"}}

	// no start date: today and the six days after
	res := s.Upcoming(context.Background(), src, time.Time{}, 7)
	if res.From != "2026-10-28" || res.To != "2026-11-03" || res.Today != "2026-10-28" {
		t.Fatalf("period %s – %s, today %s", res.From, res.To, res.Today)
	}

	// a week back: the old event and the first training
	res = s.Upcoming(context.Background(), src, time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC), 7)
	if res.From != "2026-10-21" || res.To != "2026-10-27" || res.Today != "2026-10-28" {
		t.Fatalf("period %s – %s, today %s", res.From, res.To, res.Today)
	}
	if len(res.Days) != 1 || res.Days[0].Date != "2026-10-22" || res.Days[0].Events[0].Title != "Förra veckan" {
		t.Fatalf("%+v", res.Days)
	}

	// weeks ahead: the weekly training repeats
	res = s.Upcoming(context.Background(), src, time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC), 7)
	if len(res.Days) != 1 || res.Days[0].Date != "2026-11-25" || res.Days[0].Events[0].Start.Hour() != 17 {
		t.Fatalf("%+v", res.Days)
	}
}

func TestWebcalURL(t *testing.T) {
	if got := httpURL("webcal://p01-calendars.icloud.com/x.ics"); got != "https://p01-calendars.icloud.com/x.ics" {
		t.Fatal(got)
	}
	if got := httpURL("https://calendar.google.com/x/basic.ics"); got != "https://calendar.google.com/x/basic.ics" {
		t.Fatal(got)
	}
}
