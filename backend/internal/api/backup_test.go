package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"housecontrol/internal/backup"
	"housecontrol/internal/config"
	"housecontrol/internal/dashboard"
	"housecontrol/internal/schedule"
	"housecontrol/internal/settings"
)

func init() { backup.Iterations = 1000 } // fast tests

// exportBackup returns the downloaded file (password-protected).
func exportBackup(t *testing.T, e *testEnv, password string) (int, []byte, http.Header) {
	return exportWith(t, e, `{"protect":true,"password":"`+password+`"}`)
}

func exportWith(t *testing.T, e *testEnv, body string) (int, []byte, http.Header) {
	t.Helper()
	res, err := http.Post(e.srv.URL+"/api/settings/backup", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, b, res.Header
}

func restoreBackup(t *testing.T, e *testEnv, password string, file []byte) (int, map[string]any) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"password": password, "data": base64.StdEncoding.EncodeToString(file)})
	return do(t, "POST", e.srv.URL+"/api/settings/restore", string(body))
}

func fill(t *testing.T, s *settings.Store) {
	t.Helper()
	s.SaveUI(settings.UI{Language: "sv", Title: "Hemma"})
	s.SaveMQTT(config.MQTT{URL: "tcp://192.168.1.201:1883", Password: "mqtt-hemligt", BaseTopic: "zigbee2mqtt"})
	s.SaveWeather(settings.Weather{Provider: "smhi", Latitude: 59.64, Longitude: 17.08, Location: "Enköping"})
	s.SaveCalendars([]settings.Calendar{{ID: "c1", Name: "Familjen", URL: "https://cal.example/secret.ics", Color: "#f5c242"}})
	s.SaveSchedules([]schedule.Schedule{{ID: "s1", Name: "Ute", Enabled: true, DeviceIDs: []string{"0x01"},
		On: &schedule.Trigger{Kind: schedule.KindSunset, Offset: -30}}})
	d, _ := dashboard.Dashboard{Version: 2, Tabs: []dashboard.Tab{{ID: "home", Kind: dashboard.KindCards,
		Columns: []dashboard.Column{{ID: "c", Cards: []dashboard.Card{{ID: "k", Type: "clock"}}}}}}}.Normalize()
	s.SaveDashboard(d)
}

func TestBackupRoundTrip(t *testing.T) {
	from := newEnv(t)
	fill(t, from.store)

	code, file, h := exportBackup(t, from, "familjens-lösen")
	if code != 200 {
		t.Fatalf("%d %s", code, file)
	}
	if cd := h.Get("Content-Disposition"); !strings.Contains(cd, "attachment") || !strings.Contains(cd, ".hcbackup") {
		t.Fatalf("download name: %q", cd)
	}
	if !bytes.HasPrefix(file, []byte(backup.Magic)) || bytes.Contains(file, []byte("mqtt-hemligt")) || bytes.Contains(file, []byte("secret.ics")) {
		t.Fatal("the file must be encrypted")
	}

	to := newEnv(t)
	events, cancel := to.reg.Subscribe()
	defer cancel()
	code, v := restoreBackup(t, to, "familjens-lösen", file)
	if code != 200 || v["ok"] != true {
		t.Fatalf("%d %v", code, v)
	}
	if u, _ := to.store.LoadUI(); u == nil || u.Title != "Hemma" {
		t.Fatalf("ui %+v", u)
	}
	if c, _ := to.store.LoadCalendars(); len(c) != 1 || c[0].URL != "https://cal.example/secret.ics" {
		t.Fatalf("calendars %+v", c)
	}
	if s, _ := to.store.LoadSchedules(); len(s) != 1 || s[0].On.Offset != -30 {
		t.Fatalf("schedules %+v", s)
	}
	if d, _ := to.store.LoadDashboard(); d == nil || d.Tabs[0].Columns[0].Cards[0].Type != "clock" {
		t.Fatalf("dashboard %+v", d)
	}
	if to.mq.cfg.URL != "tcp://192.168.1.201:1883" || to.mq.cfg.Password != "mqtt-hemligt" || to.mq.source != "ui" {
		t.Fatalf("the restored broker must be connected at once: %+v", to.mq.cfg)
	}
	// every open screen reloads what changed
	want := map[string]bool{"dashboard": true, "ui": true, "weather": true, "calendar": true}
	deadline := time.After(time.Second)
	for len(want) > 0 {
		select {
		case ev := <-events:
			delete(want, ev.Type)
		case <-deadline:
			t.Fatalf("not told: %v", want)
		}
	}
}

func TestBackupWrongPasswordChangesNothing(t *testing.T) {
	from := newEnv(t)
	fill(t, from.store)
	_, file, _ := exportBackup(t, from, "familjens-lösen")

	to := newEnv(t)
	to.store.SaveUI(settings.UI{Title: "Orörd"})
	code, v := restoreBackup(t, to, "fel-lösenord", file)
	if code != 400 || v["code"] != "backup_wrong_password" {
		t.Fatalf("%d %v", code, v)
	}
	if u, _ := to.store.LoadUI(); u.Title != "Orörd" {
		t.Fatal("nothing may change")
	}
}

func TestBackupRejectsOtherFiles(t *testing.T) {
	e := newEnv(t)
	for name, file := range map[string][]byte{
		"json":   []byte(`{"settings":{}}`),
		"random": bytes.Repeat([]byte{7}, 200),
	} {
		if code, v := restoreBackup(t, e, "whatever-pw", file); code != 400 || v["code"] != "backup_not_backup" {
			t.Errorf("%s: %d %v", name, code, v)
		}
	}
	if code, v := do(t, "POST", e.srv.URL+"/api/settings/restore", `{"password":"x","data":"%%%"}`); code != 400 {
		t.Errorf("bad base64: %d %v", code, v)
	}
}

func TestBackupWithInvalidContentChangesNothing(t *testing.T) {
	// a well-formed, correctly encrypted file whose settings are invalid
	bad, _ := json.Marshal(map[string]any{"app": "housecontrol", "settings": map[string]any{
		"dashboard": map[string]any{"version": 2, "tabs": []any{map[string]any{"id": "x", "kind": "video"}}},
	}})
	file, _ := backup.Seal(bad, "familjens-lösen")
	e := newEnv(t)
	e.store.SaveUI(settings.UI{Title: "Orörd"})
	code, v := restoreBackup(t, e, "familjens-lösen", file)
	if code != 400 || v["code"] != "backup_invalid" || !strings.Contains(v["error"].(string), "tab") {
		t.Fatalf("%d %v", code, v)
	}
	if u, _ := e.store.LoadUI(); u.Title != "Orörd" {
		t.Fatal("nothing may change")
	}

	// a backup from another app
	other, _ := backup.Seal([]byte(`{"app":"something-else","settings":{}}`), "familjens-lösen")
	if code, v := restoreBackup(t, e, "familjens-lösen", other); code != 400 || v["code"] != "backup_not_backup" {
		t.Fatalf("%d %v", code, v)
	}
}

func TestBackupPasswordRules(t *testing.T) {
	e := newEnv(t)
	code, _, _ := exportBackup(t, e, "kort")
	if code != 400 {
		t.Fatalf("short password: %d", code)
	}
}

func TestBackupNeedsLogin(t *testing.T) {
	e := newAuthEnv(t)
	if code, _, _ := exportBackup(t, e, "familjens-lösen"); code != 401 {
		t.Fatalf("export: %d", code)
	}
	if code, _ := restoreBackup(t, e, "familjens-lösen", []byte("x")); code != 401 {
		t.Fatalf("restore: %d", code)
	}
}

func TestUnprotectedBackupIsReadableJSON(t *testing.T) {
	from := newEnv(t)
	fill(t, from.store)
	code, file, h := exportWith(t, from, `{"protect":false}`)
	if code != 200 {
		t.Fatalf("%d %s", code, file)
	}
	if cd := h.Get("Content-Disposition"); !strings.Contains(cd, ".json") {
		t.Fatalf("%q", cd)
	}
	var bf struct {
		App      string `json:"app"`
		Settings struct {
			UI struct{ Title string } `json:"ui"`
		} `json:"settings"`
	}
	if err := json.Unmarshal(file, &bf); err != nil || bf.App != "housecontrol" || bf.Settings.UI.Title != "Hemma" {
		t.Fatalf("%v %s", err, file)
	}

	// restored without a password
	to := newEnv(t)
	if code, v := restoreBackup(t, to, "", file); code != 200 {
		t.Fatalf("%d %v", code, v)
	}
	if u, _ := to.store.LoadUI(); u == nil || u.Title != "Hemma" {
		t.Fatalf("%+v", u)
	}
}

func TestProtectedBackupNeedsItsPassword(t *testing.T) {
	from := newEnv(t)
	fill(t, from.store)
	_, file, _ := exportBackup(t, from, "familjens-lösen")
	to := newEnv(t)
	if code, v := restoreBackup(t, to, "", file); code != 400 || v["code"] != "backup_needs_password" {
		t.Fatalf("%d %v", code, v)
	}
}
