package api

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"housecontrol/internal/autobackup"
	"housecontrol/internal/settings"
)

// newEnvWithBackups is a server with a backup folder (BACKUP_DIR).
func newEnvWithBackups(t *testing.T, password string) (*testEnv, *autobackup.Manager) {
	t.Helper()
	var m *autobackup.Manager
	e := newEnvSetup(t, func(e *testEnv, o *Options) {
		m = autobackup.New(filepath.Join(t.TempDir(), "backups"), password, "1.2.3", e.store,
			slog.New(slog.NewTextHandler(io.Discard, nil)))
		if err := m.Check(); err != nil {
			t.Fatal(err)
		}
		o.AutoBackup = m
	})
	return e, m
}

func writeBackupFile(t *testing.T, m *autobackup.Manager, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(m.Dir(), name), []byte(`{"app":"housecontrol","settings":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestAutoBackupUnavailableWithoutFolder(t *testing.T) {
	e := newEnv(t)
	code, v := do(t, "GET", e.srv.URL+"/api/settings/autobackup", "")
	if code != 200 || v["available"] != false || v["enabled"] != true || v["time"] != "03:00" || v["keep"] != float64(14) {
		t.Fatalf("%d %v", code, v)
	}
	if code, _ := do(t, "POST", e.srv.URL+"/api/settings/autobackup/run", ""); code != http.StatusConflict {
		t.Fatalf("run without folder: %d", code)
	}
}

func TestAutoBackupSettingsAndRun(t *testing.T) {
	e, _ := newEnvWithBackups(t, "")
	code, v := do(t, "PUT", e.srv.URL+"/api/settings/autobackup", `{"enabled":true,"time":"04:15","keep":2}`)
	if code != 200 || v["time"] != "04:15" || v["keep"] != float64(2) || v["available"] != true || v["encrypted"] != false {
		t.Fatalf("%d %v", code, v)
	}
	if v["next"] == nil {
		t.Fatalf("next run missing: %v", v)
	}
	if a, _ := e.store.LoadAutoBackup(); a.Time != "04:15" || a.Keep != 2 {
		t.Fatalf("saved %+v", a)
	}
	if code, v := do(t, "PUT", e.srv.URL+"/api/settings/autobackup", `{"enabled":true,"time":"25:00","keep":2}`); code != 400 {
		t.Fatalf("bad time: %d %v", code, v)
	}

	code, v = do(t, "POST", e.srv.URL+"/api/settings/autobackup/run", "")
	if code != 200 {
		t.Fatalf("run: %d %v", code, v)
	}
	list := v["backups"].([]any)
	if len(list) != 1 {
		t.Fatalf("backups %v", v)
	}
	name := list[0].(map[string]any)["name"].(string)

	res, err := http.Get(e.srv.URL + "/api/settings/autobackup/files/" + name)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || !strings.Contains(res.Header.Get("Content-Disposition"), name) || !strings.Contains(string(b), `"app": "housecontrol"`) {
		t.Fatalf("download %d %q %s", res.StatusCode, res.Header.Get("Content-Disposition"), b)
	}
	if res, _ := http.Get(e.srv.URL + "/api/settings/autobackup/files/..%2Fsettings.json"); res.StatusCode != 404 {
		t.Fatalf("path escape: %d", res.StatusCode)
	}
}

func TestAutoBackupLowerKeepPrunesAtOnce(t *testing.T) {
	e, m := newEnvWithBackups(t, "")
	fill(t, e.store)
	for _, ts := range []string{"20261028T020000Z", "20261029T020000Z", "20261030T020000Z"} {
		writeBackupFile(t, m, "housecontrol-"+ts+".json")
	}
	code, v := do(t, "PUT", e.srv.URL+"/api/settings/autobackup", `{"enabled":true,"time":"03:00","keep":1}`)
	if code != 200 || len(v["backups"].([]any)) != 1 {
		t.Fatalf("%d %v", code, v)
	}
}

func TestAutoBackupRestoreStoredFile(t *testing.T) {
	e, m := newEnvWithBackups(t, "familjens-lösen")
	fill(t, e.store)
	info, err := m.Create()
	if err != nil {
		t.Fatal(err)
	}
	e.store.SaveUI(settings.UI{Language: "en", Title: "Ändrad"})
	e.store.SaveAutoBackup(settings.AutoBackup{Enabled: true, Time: "05:00", Keep: 5})

	code, v := do(t, "POST", e.srv.URL+"/api/settings/autobackup/files/"+info.Name+"/restore", "{}")
	if code != 200 || v["ok"] != true {
		t.Fatalf("%d %v", code, v)
	}
	if u, _ := e.store.LoadUI(); u == nil || u.Title != "Hemma" {
		t.Fatalf("ui %+v", u)
	}
	// the backup had no automatic-backup settings: today's are kept
	if a, _ := e.store.LoadAutoBackup(); a.Time != "05:00" {
		t.Fatalf("autobackup %+v", a)
	}
	if code, v := do(t, "POST", e.srv.URL+"/api/settings/autobackup/files/"+info.Name+"/restore", `{"password":"fel-lösenord"}`); code != 400 || v["code"] != codeBackupWrongPassword {
		t.Fatalf("explicit wrong password: %d %v", code, v)
	}
	if code, _ := do(t, "POST", e.srv.URL+"/api/settings/autobackup/files/housecontrol-20200101T000000Z.json/restore", "{}"); code != 404 {
		t.Fatalf("missing: %d", code)
	}
}

func TestAutoBackupNeedsLogin(t *testing.T) {
	e := newAuthEnv(t)
	for _, r := range []struct{ method, path string }{
		{"GET", "/api/settings/autobackup"},
		{"PUT", "/api/settings/autobackup"},
		{"POST", "/api/settings/autobackup/run"},
		{"GET", "/api/settings/autobackup/files/housecontrol-20261030T020000Z.json"},
		{"POST", "/api/settings/autobackup/files/housecontrol-20261030T020000Z.json/restore"},
	} {
		if code, _ := do(t, r.method, e.srv.URL+r.path, "{}"); code != 401 {
			t.Errorf("%s %s: %d", r.method, r.path, code)
		}
	}
}
