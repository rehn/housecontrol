package autobackup

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"housecontrol/internal/backup"
	"housecontrol/internal/settings"
)

func init() { backup.Iterations = 1000 } // fast tests

type fakeStore struct {
	snap settings.Snapshot
	ab   settings.AutoBackup
	err  error
}

func (f *fakeStore) Snapshot() (settings.Snapshot, error)         { return f.snap, f.err }
func (f *fakeStore) LoadAutoBackup() (settings.AutoBackup, error) { return f.ab, nil }

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newManager(t *testing.T, password string) (*Manager, *fakeStore, *clock, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "backups")
	st := &fakeStore{
		snap: settings.Snapshot{UI: &settings.UI{Language: "sv", Title: "Hemma"}},
		ab:   settings.AutoBackup{Enabled: true, Time: "03:00", Keep: 3},
	}
	loc, _ := time.LoadLocation("Europe/Stockholm")
	c := &clock{t: time.Date(2026, 10, 30, 14, 0, 0, 0, loc)}
	m := New(dir, password, "1.2.3", st, slog.New(slog.NewTextHandler(io.Discard, nil)))
	m.now = c.now
	if err := m.Check(); err != nil {
		t.Fatal(err)
	}
	return m, st, c, dir
}

func names(t *testing.T, m *Manager) []string {
	t.Helper()
	list, err := m.List()
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, i := range list {
		out = append(out, i.Name)
	}
	return out
}

func TestCreatePlainAndEncrypted(t *testing.T) {
	m, _, _, dir := newManager(t, "")
	info, err := m.Create()
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "housecontrol-20261030T130000Z.json" || info.Size == 0 {
		t.Fatalf("%+v", info)
	}
	b, _ := os.ReadFile(filepath.Join(dir, info.Name))
	var f File
	if err := json.Unmarshal(b, &f); err != nil || f.App != App || f.Version != "1.2.3" || f.Settings.UI.Title != "Hemma" {
		t.Fatalf("%v %+v", err, f)
	}
	if st, _ := os.Stat(filepath.Join(dir, info.Name)); st.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", st.Mode())
	}

	e, _, _, edir := newManager(t, "familjens-lösen")
	if !e.Encrypted() {
		t.Fatal("encrypted")
	}
	info, err = e.Create()
	if err != nil || !strings.HasSuffix(info.Name, ".hcbackup") {
		t.Fatalf("%+v %v", info, err)
	}
	b, _ = os.ReadFile(filepath.Join(edir, info.Name))
	plain, err := backup.Open(b, "familjens-lösen")
	if err != nil || !strings.Contains(string(plain), "Hemma") {
		t.Fatalf("%v", err)
	}
}

func TestWeakPasswordIsReported(t *testing.T) {
	m := New(t.TempDir(), "kort", "dev", &fakeStore{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := m.Check(); err == nil {
		t.Fatal("a too short BACKUP_PASSWORD must be reported at start")
	}
}

func TestRetentionKeepsTheNewest(t *testing.T) {
	m, st, c, dir := newManager(t, "")
	os.WriteFile(filepath.Join(dir, "my-notes.txt"), []byte("not ours"), 0o644)
	for i := 0; i < 5; i++ {
		if _, err := m.Create(); err != nil {
			t.Fatal(err)
		}
		c.t = c.t.Add(24 * time.Hour)
	}
	got := names(t, m)
	want := []string{"housecontrol-20261103T130000Z.json", "housecontrol-20261102T130000Z.json", "housecontrol-20261101T130000Z.json"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "my-notes.txt")); err != nil {
		t.Fatal("other files must be left alone")
	}
	st.ab.Keep = 1
	if err := m.Prune(1); err != nil {
		t.Fatal(err)
	}
	if got := names(t, m); len(got) != 1 || got[0] != want[0] {
		t.Fatalf("after prune %v", got)
	}
}

func TestTickRunsOncePerDay(t *testing.T) {
	m, st, c, _ := newManager(t, "")
	loc := c.t.Location()

	// enabled, and today's 03:00 has passed with no backup yet: make one
	m.Tick()
	if n := len(names(t, m)); n != 1 {
		t.Fatalf("first tick: %d", n)
	}
	m.Tick()
	if n := len(names(t, m)); n != 1 {
		t.Fatalf("same day again: %d", n)
	}
	c.t = time.Date(2026, 10, 31, 2, 59, 0, 0, loc)
	m.Tick()
	if n := len(names(t, m)); n != 1 {
		t.Fatalf("before 03:00: %d", n)
	}
	c.t = time.Date(2026, 10, 31, 3, 0, 15, 0, loc)
	m.Tick()
	if n := len(names(t, m)); n != 2 {
		t.Fatalf("at 03:00: %d", n)
	}
	if s := m.Status(); !s.LastRun.Equal(c.t) || s.LastError != "" {
		t.Fatalf("status %+v", s)
	}

	st.ab.Enabled = false
	c.t = c.t.Add(48 * time.Hour)
	m.Tick()
	if n := len(names(t, m)); n != 2 {
		t.Fatalf("switched off: %d", n)
	}
}

func TestTickBacksOffAfterAFailure(t *testing.T) {
	m, st, c, _ := newManager(t, "")
	st.err = errors.New("disk on fire")
	m.Tick()
	if s := m.Status(); !strings.Contains(s.LastError, "disk on fire") {
		t.Fatalf("status %+v", s)
	}
	st.err = nil
	c.t = c.t.Add(time.Minute)
	m.Tick()
	if n := len(names(t, m)); n != 0 {
		t.Fatal("must wait a while before trying again")
	}
	c.t = c.t.Add(RetryAfter)
	m.Tick()
	if n := len(names(t, m)); n != 1 || m.Status().LastError != "" {
		t.Fatalf("retry: %d %+v", n, m.Status())
	}
}

func TestReadOnlyOurFiles(t *testing.T) {
	m, _, _, dir := newManager(t, "")
	info, _ := m.Create()
	if b, err := m.Read(info.Name); err != nil || len(b) == 0 {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(filepath.Dir(dir), "settings.json"), []byte("{}"), 0o600)
	for _, bad := range []string{"../settings.json", "my-notes.txt", "housecontrol-x.json", ""} {
		if _, err := m.Read(bad); !errors.Is(err, ErrNotFound) {
			t.Errorf("%q: %v", bad, err)
		}
	}
	if _, err := m.Read("housecontrol-20200101T000000Z.json"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing file: %v", err)
	}
}
