package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"housecontrol/internal/schedule"
	"housecontrol/internal/settings"
)

type placeView struct {
	Known   bool   `json:"known"`
	Name    string `json:"name,omitempty"`
	Sunrise string `json:"sunrise,omitempty"` // today, RFC 3339
	Sunset  string `json:"sunset,omitempty"`
	Polar   string `json:"polar,omitempty"` // "day" / "night" when the sun doesn't rise or set today
}

type nextView struct {
	Action string `json:"action"`
	At     string `json:"at"`
}

// place reads the weather place; Known is false until one is saved.
func (s *Server) place() (schedule.Place, string) {
	w, _ := s.store.LoadWeather()
	return settings.SchedulePlace(w, time.Local)
}

func (s *Server) schedulesView(list []schedule.Schedule) map[string]any {
	if list == nil {
		list = []schedule.Schedule{}
	}
	p, name := s.place()
	now := time.Now()
	pv := placeView{Known: p.Known, Name: name}
	if p.Known {
		sun := schedule.SunOn(now.In(p.Loc), p.Lat, p.Lon)
		if sun.Polar != "" {
			pv.Polar = sun.Polar
		} else {
			pv.Sunrise = sun.Rise.In(p.Loc).Format(time.RFC3339)
			pv.Sunset = sun.Set.In(p.Loc).Format(time.RFC3339)
		}
	}
	next := map[string]nextView{}
	for _, sc := range list {
		if ev, ok := sc.Next(now, p); ok {
			next[sc.ID] = nextView{Action: ev.Action, At: ev.At.In(p.Loc).Format(time.RFC3339)}
		}
	}
	return map[string]any{"schedules": list, "place": pv, "next": next}
}

func (s *Server) getSchedules(w http.ResponseWriter, _ *http.Request) {
	list, err := s.store.LoadSchedules()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not read schedules: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s.schedulesView(list))
}

func (s *Server) putSchedules(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Schedules []schedule.Schedule `json:"schedules"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if len(in.Schedules) > schedule.MaxSchedules {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("at most %d schedules", schedule.MaxSchedules))
		return
	}
	out := make([]schedule.Schedule, 0, len(in.Schedules))
	seen := map[string]bool{}
	for _, sc := range in.Schedules {
		n, err := sc.Normalize()
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if seen[n.ID] {
			writeErr(w, http.StatusBadRequest, fmt.Sprintf("duplicate schedule id %q", n.ID))
			return
		}
		seen[n.ID] = true
		out = append(out, n)
	}
	if err := s.store.SaveSchedules(out); err != nil {
		writeErr(w, http.StatusInternalServerError, "could not save schedules: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s.schedulesView(out))
}
