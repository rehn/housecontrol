// Package autobackup makes backups of all settings on a schedule and keeps
// the newest ones in a folder (BACKUP_DIR, e.g. its own volume in Kubernetes).
package autobackup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"

	"housecontrol/internal/backup"
	"housecontrol/internal/settings"
)

// App marks a housecontrol backup file.
const App = "housecontrol"

// File is what a backup contains (before compression and encryption).
type File struct {
	App      string            `json:"app"`
	Version  string            `json:"version,omitempty"`
	Exported time.Time         `json:"exported"`
	Settings settings.Snapshot `json:"settings"`
}

// Encode makes a backup file: encrypted with password, or (password "")
// readable JSON.
func Encode(snap settings.Snapshot, version string, now time.Time, password string) ([]byte, error) {
	f := File{App: App, Version: version, Exported: now, Settings: snap}
	if password == "" {
		return json.MarshalIndent(f, "", "  ")
	}
	plain, err := json.Marshal(f)
	if err != nil {
		return nil, err
	}
	return backup.Seal(plain, password)
}

const (
	// TickEvery is how often the manager checks if a backup is due.
	TickEvery = 30 * time.Second
	// RetryAfter: after a failed backup, wait this long before trying again.
	RetryAfter = 10 * time.Minute
	stamp      = "20060102T150405Z"
)

var nameRe = regexp.MustCompile(`^housecontrol-(\d{8}T\d{6}Z)\.(json|hcbackup)$`)

var ErrNotFound = errors.New("no such backup")

// Store is where the settings come from (settings.Store).
type Store interface {
	Snapshot() (settings.Snapshot, error)
	LoadAutoBackup() (settings.AutoBackup, error)
}

// Info describes one stored backup.
type Info struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	Created time.Time `json:"created"`
}

// Status is how the last automatic run went.
type Status struct {
	LastRun   time.Time `json:"lastRun,omitempty"`
	LastError string    `json:"lastError,omitempty"`
}

// Manager writes, lists and prunes backups in one folder.
type Manager struct {
	dir, password, version string
	store                  Store
	log                    *slog.Logger
	now                    func() time.Time

	mu       sync.Mutex
	lastRun  time.Time
	lastErr  string
	failedAt time.Time
}

func New(dir, password, version string, store Store, log *slog.Logger) *Manager {
	return &Manager{dir: dir, password: password, version: version, store: store, log: log, now: time.Now}
}

func (m *Manager) Dir() string     { return m.dir }
func (m *Manager) Encrypted() bool { return m.password != "" }

// Password is what stored backups are encrypted with ("" = not encrypted).
func (m *Manager) Password() string { return m.password }

// Check makes sure the folder exists and is writable, and that the password
// is usable. Call it at start.
func (m *Manager) Check() error {
	if m.password != "" && len([]rune(m.password)) < backup.MinPassword {
		return fmt.Errorf("BACKUP_PASSWORD must be at least %d characters", backup.MinPassword)
	}
	if err := os.MkdirAll(m.dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(m.dir, ".check-*")
	if err != nil {
		return fmt.Errorf("backup folder %s is not writable: %w", m.dir, err)
	}
	f.Close()
	return os.Remove(f.Name())
}

// Create makes a backup now and then deletes the oldest beyond the retention.
func (m *Manager) Create() (Info, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.create()
}

func (m *Manager) create() (Info, error) {
	snap, err := m.store.Snapshot()
	if err != nil {
		return Info{}, fmt.Errorf("read settings: %w", err)
	}
	now := m.now()
	data, err := Encode(snap, m.version, now, m.password)
	if err != nil {
		return Info{}, err
	}
	ext := ".json"
	if m.password != "" {
		ext = ".hcbackup"
	}
	name := "housecontrol-" + now.UTC().Format(stamp) + ext
	if err := writeNew(filepath.Join(m.dir, name), data); err != nil {
		return Info{}, err
	}
	if a, err := m.store.LoadAutoBackup(); err == nil {
		if err := m.prune(a.Keep); err != nil {
			m.log.Warn("backup: could not delete old backups", "err", err)
		}
	}
	m.log.Info("backup saved", "file", name, "encrypted", m.password != "")
	return Info{Name: name, Size: int64(len(data)), Created: now.UTC().Truncate(time.Second)}, nil
}

// writeNew writes atomically, owner-readable only, never over an existing file.
func writeNew(path string, data []byte) error {
	if _, err := os.Stat(path); err == nil {
		return errors.New("a backup was just made, try again in a second")
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".backup-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// List returns the stored backups, newest first. Other files are ignored.
func (m *Manager) List() ([]Info, error) {
	entries, err := os.ReadDir(m.dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Info
	for _, e := range entries {
		mt := nameRe.FindStringSubmatch(e.Name())
		if mt == nil || !e.Type().IsRegular() {
			continue
		}
		created, err := time.Parse(stamp, mt[1])
		if err != nil {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, Info{Name: e.Name(), Size: fi.Size(), Created: created})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created.After(out[j].Created) })
	return out, nil
}

// Read returns one stored backup by name.
func (m *Manager) Read(name string) ([]byte, error) {
	if !nameRe.MatchString(name) {
		return nil, ErrNotFound
	}
	b, err := os.ReadFile(filepath.Join(m.dir, name))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	return b, err
}

// Prune deletes all but the newest keep backups.
func (m *Manager) Prune(keep int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.prune(keep)
}

func (m *Manager) prune(keep int) error {
	if keep < settings.MinBackupKeep {
		keep = settings.MinBackupKeep
	}
	list, err := m.List()
	if err != nil {
		return err
	}
	var errs []error
	for _, old := range list[min(keep, len(list)):] {
		if err := os.Remove(filepath.Join(m.dir, old.Name)); err != nil {
			errs = append(errs, err)
		} else {
			m.log.Info("old backup deleted", "file", old.Name)
		}
	}
	return errors.Join(errs...)
}

// Status is how the last automatic backup went.
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return Status{LastRun: m.lastRun, LastError: m.lastErr}
}

// Tick makes a backup if one is due: switched on, and no backup since the
// latest planned time (so a backup missed while the app was down is made as
// soon as it is back).
func (m *Manager) Tick() {
	a, err := m.store.LoadAutoBackup()
	if err != nil || !a.Enabled {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	if !m.failedAt.IsZero() && now.Sub(m.failedAt) < RetryAfter {
		return
	}
	list, err := m.List()
	if err != nil {
		m.fail(now, err)
		return
	}
	if len(list) > 0 && !list[0].Created.Before(a.Last(now).Truncate(time.Second)) {
		return
	}
	if _, err := m.create(); err != nil {
		m.fail(now, err)
		return
	}
	m.lastRun, m.lastErr, m.failedAt = now, "", time.Time{}
}

func (m *Manager) fail(now time.Time, err error) {
	m.log.Error("automatic backup failed", "err", err)
	m.lastRun, m.lastErr, m.failedAt = now, err.Error(), now
}

// Run checks for due backups until ctx ends.
func (m *Manager) Run(ctx context.Context) {
	m.Tick()
	t := time.NewTicker(TickEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.Tick()
		}
	}
}
