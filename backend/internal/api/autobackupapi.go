package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"housecontrol/internal/autobackup"
	"housecontrol/internal/settings"
)

// autoBackupView is the Settings → Backup page's "Automatic backups" part.
type autoBackupView struct {
	settings.AutoBackup
	Available bool              `json:"available"` // a backup folder is set (BACKUP_DIR)
	Dir       string            `json:"dir,omitempty"`
	Encrypted bool              `json:"encrypted"` // BACKUP_PASSWORD is set
	Next      *time.Time        `json:"next,omitempty"`
	LastRun   *time.Time        `json:"lastRun,omitempty"`
	LastError string            `json:"lastError,omitempty"`
	Backups   []autobackup.Info `json:"backups"`
}

func (s *Server) autoBackupState() (autoBackupView, error) {
	a, err := s.store.LoadAutoBackup()
	if err != nil {
		return autoBackupView{}, err
	}
	v := autoBackupView{AutoBackup: a, Backups: []autobackup.Info{}}
	m := s.opts.AutoBackup
	if m == nil {
		return v, nil
	}
	v.Available, v.Dir, v.Encrypted = true, m.Dir(), m.Encrypted()
	if a.Enabled {
		next := a.Next(time.Now())
		v.Next = &next
	}
	st := m.Status()
	if !st.LastRun.IsZero() {
		v.LastRun = &st.LastRun
	}
	v.LastError = st.LastError
	list, err := m.List()
	if err != nil {
		v.LastError = "could not list the backups: " + err.Error()
	} else if list != nil {
		v.Backups = list
	}
	return v, nil
}

func (s *Server) writeAutoBackup(w http.ResponseWriter) {
	v, err := s.autoBackupState()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) getAutoBackup(w http.ResponseWriter, _ *http.Request) {
	s.writeAutoBackup(w)
}

func (s *Server) putAutoBackup(w http.ResponseWriter, r *http.Request) {
	var in settings.AutoBackup
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	a, err := in.Normalize()
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.SaveAutoBackup(a); err != nil {
		writeErr(w, http.StatusInternalServerError, "could not save: "+err.Error())
		return
	}
	if m := s.opts.AutoBackup; m != nil {
		if err := m.Prune(a.Keep); err != nil {
			s.log.Warn("backup: could not delete old backups", "err", err)
		}
	}
	s.writeAutoBackup(w)
}

// noBackupFolder answers when BACKUP_DIR isn't set.
func (s *Server) noBackupFolder(w http.ResponseWriter) bool {
	if s.opts.AutoBackup != nil {
		return false
	}
	writeErrCode(w, http.StatusConflict, codeBackupNoFolder, "no backup folder: set BACKUP_DIR")
	return true
}

func (s *Server) runAutoBackup(w http.ResponseWriter, _ *http.Request) {
	if s.noBackupFolder(w) {
		return
	}
	if _, err := s.opts.AutoBackup.Create(); err != nil {
		writeErr(w, http.StatusInternalServerError, "could not make the backup: "+err.Error())
		return
	}
	s.writeAutoBackup(w)
}

func (s *Server) storedBackup(w http.ResponseWriter, r *http.Request) (string, []byte, bool) {
	if s.noBackupFolder(w) {
		return "", nil, false
	}
	name := r.PathValue("name")
	b, err := s.opts.AutoBackup.Read(name)
	if errors.Is(err, autobackup.ErrNotFound) {
		writeErr(w, http.StatusNotFound, err.Error())
		return "", nil, false
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return "", nil, false
	}
	return name, b, true
}

func (s *Server) downloadStoredBackup(w http.ResponseWriter, r *http.Request) {
	name, b, ok := s.storedBackup(w, r)
	if !ok {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if strings.HasSuffix(name, ".json") {
		w.Header().Set("Content-Type", "application/json")
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	w.Write(b)
}

// restoreStoredBackup restores a backup from the folder. Encrypted ones are
// opened with BACKUP_PASSWORD unless a password is given (an older backup
// made before the password was changed).
func (s *Server) restoreStoredBackup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Password string `json:"password"`
	}
	if r.ContentLength != 0 {
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&in); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
	}
	_, b, ok := s.storedBackup(w, r)
	if !ok {
		return
	}
	password := in.Password
	if password == "" {
		password = s.opts.AutoBackup.Password()
	}
	s.restoreFile(w, b, password)
}
