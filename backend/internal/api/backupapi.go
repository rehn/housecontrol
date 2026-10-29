package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"housecontrol/internal/backup"
	"housecontrol/internal/devices"
	"housecontrol/internal/schedule"
	"housecontrol/internal/settings"
	"housecontrol/internal/weather"
)

const backupApp = "housecontrol"

// backupFile is what a backup contains (before compression and encryption).
type backupFile struct {
	App      string            `json:"app"`
	Version  string            `json:"version,omitempty"`
	Exported time.Time         `json:"exported"`
	Settings settings.Snapshot `json:"settings"`
}

// exportBackup sends all settings as a file: compressed and encrypted with a
// password ({"protect": true, "password": "..."}), or as readable JSON
// ({"protect": false}).
func (s *Server) exportBackup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Protect  bool   `json:"protect"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	snap, err := s.store.Snapshot()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not read the settings: "+err.Error())
		return
	}
	now := time.Now()
	bf := backupFile{App: backupApp, Version: s.opts.Version, Exported: now, Settings: snap}
	name := "housecontrol-backup-" + now.Format("2006-01-02")
	w.Header().Set("Cache-Control", "no-store")
	if !in.Protect {
		plain, err := json.MarshalIndent(bf, "", "  ")
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		s.log.Info("backup exported", "protected", false)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.json"`, name))
		w.Write(plain)
		return
	}
	plain, err := json.Marshal(bf)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	sealed, err := backup.Seal(plain, in.Password)
	if errors.Is(err, backup.ErrWeakPassword) {
		writeErrCode(w, http.StatusBadRequest, codeBackupWeakPassword, err.Error())
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not create the backup: "+err.Error())
		return
	}
	s.log.Info("backup exported", "protected", true)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.hcbackup"`, name))
	w.Write(sealed)
}

// restoreBackup replaces all settings with a backup, after checking all of it.
// Body: {"password": "...", "data": "<the file, base64>"}; the password is only
// needed for a protected (.hcbackup) file.
func (s *Server) restoreBackup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Password string `json:"password"`
		Data     string `json:"data"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	file, err := base64.StdEncoding.DecodeString(in.Data)
	if err != nil {
		writeErrCode(w, http.StatusBadRequest, codeBackupNotBackup, backup.ErrNotBackup.Error())
		return
	}
	plain := file // an unprotected backup is plain JSON
	if backup.IsSealed(file) {
		if in.Password == "" {
			writeErrCode(w, http.StatusBadRequest, codeBackupNeedsPassword, "this backup is protected with a password")
			return
		}
		plain, err = backup.Open(file, in.Password)
	}
	switch {
	case errors.Is(err, backup.ErrWrongPassword):
		writeErrCode(w, http.StatusBadRequest, codeBackupWrongPassword, err.Error())
		return
	case err != nil:
		writeErrCode(w, http.StatusBadRequest, codeBackupNotBackup, err.Error())
		return
	}
	var bf backupFile
	if err := json.Unmarshal(plain, &bf); err != nil || bf.App != backupApp {
		writeErrCode(w, http.StatusBadRequest, codeBackupNotBackup, backup.ErrNotBackup.Error())
		return
	}
	snap, err := validSnapshot(bf.Settings)
	if err != nil {
		writeErrCode(w, http.StatusBadRequest, codeBackupInvalid, "the backup can't be used: "+err.Error())
		return
	}
	if err := s.store.Restore(snap); err != nil {
		writeErr(w, http.StatusInternalServerError, "could not save the settings: "+err.Error())
		return
	}
	s.log.Info("backup restored", "exported", bf.Exported, "version", bf.Version)

	// apply at once: broker, and tell every screen to reload what changed
	if snap.MQTT != nil {
		s.mqtt.Apply(*snap.MQTT, "ui")
	}
	if d, err := s.loadDashboard(); err == nil {
		s.reg.Broadcast(devices.Event{Type: "dashboard", Dashboard: d})
	}
	ui := s.currentUI()
	s.reg.Broadcast(devices.Event{Type: "ui", UI: &ui})
	s.reg.Broadcast(devices.Event{Type: "weather"})
	s.reg.Broadcast(devices.Event{Type: "calendar"})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "exported": bf.Exported, "version": bf.Version})
}

// validSnapshot checks every part of a backup the way the settings pages do,
// and returns it cleaned up (e.g. an old dashboard upgraded to tabs).
func validSnapshot(sn settings.Snapshot) (settings.Snapshot, error) {
	if sn.MQTT != nil {
		m, err := sn.MQTT.Normalize()
		if err != nil {
			return sn, fmt.Errorf("MQTT: %w", err)
		}
		sn.MQTT = &m
	}
	if sn.Dashboard != nil {
		d, err := sn.Dashboard.Normalize()
		if err != nil {
			return sn, fmt.Errorf("home page: %w", err)
		}
		sn.Dashboard = &d
	}
	if sn.UI != nil {
		if sn.UI.Language != "" && !languageRe.MatchString(sn.UI.Language) {
			return sn, errors.New("unknown language")
		}
		sn.UI.Title = strings.TrimSpace(sn.UI.Title)
		if utf8.RuneCountInString(sn.UI.Title) > settings.MaxTitleLen {
			return sn, errors.New("title too long")
		}
	}
	if w := sn.Weather; w != nil {
		if !slices.Contains(weather.ProviderIDs, w.Provider) || w.Latitude < -90 || w.Latitude > 90 || w.Longitude < -180 || w.Longitude > 180 {
			return sn, errors.New("weather: unknown provider or place")
		}
	}
	if len(sn.Calendars) > maxCalendars {
		return sn, fmt.Errorf("at most %d calendars", maxCalendars)
	}
	ids := map[string]bool{}
	for _, c := range sn.Calendars {
		name := strings.TrimSpace(c.Name)
		if c.ID == "" || ids[c.ID] || name == "" || utf8.RuneCountInString(name) > maxCalendarName ||
			!colorRe.MatchString(c.Color) || !validCalendarURL(c.URL) {
			return sn, fmt.Errorf("calendar %q is not valid", c.Name)
		}
		ids[c.ID] = true
	}
	if len(sn.Schedules) > schedule.MaxSchedules {
		return sn, errors.New("too many schedules")
	}
	seen := map[string]bool{}
	for i, sc := range sn.Schedules {
		n, err := sc.Normalize()
		if err != nil {
			return sn, fmt.Errorf("schedule: %w", err)
		}
		if seen[n.ID] {
			return sn, fmt.Errorf("duplicate schedule %q", n.Name)
		}
		seen[n.ID] = true
		sn.Schedules[i] = n
	}
	return sn, nil
}
