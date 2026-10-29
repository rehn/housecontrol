package api

import (
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"

	"housecontrol/internal/auth"
	"housecontrol/internal/devices"
)

func newAuthEnv(t *testing.T) *testEnv {
	return newEnvWith(t, func(o *Options) {
		o.Auth = auth.New("admin", "hemligt", []byte("0123456789abcdef0123456789abcdef"))
		o.CORSOrigins = []string{"http://localhost:3000"}
	})
}

// client keeps cookies, like a browser.
func client(t *testing.T) *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar}
}

func call(t *testing.T, c *http.Client, method, url, body string) (int, map[string]any, *http.Response) {
	req, _ := http.NewRequest(method, url, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var m map[string]any
	_ = jsonDecode(res, &m)
	return res.StatusCode, m, res
}

func TestLoginOffWithoutPassword(t *testing.T) {
	e := newEnv(t)
	code, v := do(t, "GET", e.srv.URL+"/api/auth", "")
	if code != 200 || v["required"] != false || v["loggedIn"] != true {
		t.Fatalf("%d %v", code, v)
	}
	if code, _ := do(t, "PUT", e.srv.URL+"/api/settings/ui", `{"title":"Hemma"}`); code != 200 {
		t.Fatalf("no password = everything open: %d", code)
	}
}

func TestViewingNeedsNoLogin(t *testing.T) {
	e := newAuthEnv(t)
	for _, path := range []string{"/api/status", "/api/devices", "/api/dashboard", "/api/settings/ui", "/api/weather", "/api/calendar", "/", "/settings/"} {
		if code, _ := do(t, "GET", e.srv.URL+path, ""); code == 401 {
			t.Errorf("GET %s should be open for viewing", path)
		}
	}
	code, v := do(t, "GET", e.srv.URL+"/api/auth", "")
	if code != 200 || v["required"] != true || v["loggedIn"] != false {
		t.Fatalf("%d %v", code, v)
	}
}

func TestChangesAndSettingsNeedLogin(t *testing.T) {
	e := newAuthEnv(t)
	for _, c := range []struct{ method, path, body string }{
		{"PUT", "/api/dashboard", `{}`},
		{"PUT", "/api/settings/ui", `{"title":"x"}`},
		{"GET", "/api/settings/mqtt", ``},
		{"PUT", "/api/settings/mqtt", `{}`},
		{"POST", "/api/settings/mqtt/test", `{}`},
		{"GET", "/api/settings/weather", ``},
		{"PUT", "/api/settings/weather", `{}`},
		{"GET", "/api/weather/places?q=Enk", ``},
		{"GET", "/api/weather/area?lat=59&lon=17", ``},
		{"GET", "/api/weather/locate", ``},
		{"GET", "/api/settings/calendars", ``},
		{"PUT", "/api/settings/calendars", `{"calendars":[]}`},
		{"POST", "/api/settings/calendars/test", `{"url":"https://x/y.ics"}`},
	} {
		code, v := do(t, c.method, e.srv.URL+c.path, c.body)
		if code != 401 || v["code"] != "unauthorized" {
			t.Errorf("%s %s: %d %v", c.method, c.path, code, v)
		}
	}
}

func TestLoginLogout(t *testing.T) {
	e := newAuthEnv(t)
	c := client(t)

	code, v, _ := call(t, c, "POST", e.srv.URL+"/api/auth/login", `{"username":"admin","password":"fel"}`)
	if code != 401 || v["code"] != "bad_login" {
		t.Fatalf("%d %v", code, v)
	}

	code, v, res := call(t, c, "POST", e.srv.URL+"/api/auth/login", `{"username":"Admin","password":"hemligt"}`)
	if code != 200 || v["loggedIn"] != true || v["username"] != "admin" {
		t.Fatalf("%d %v", code, v)
	}
	ck := res.Header.Get("Set-Cookie")
	for _, want := range []string{"hc_session=", "HttpOnly", "SameSite=Lax", "Path=/", "Max-Age=2592000"} {
		if !strings.Contains(ck, want) {
			t.Errorf("cookie %q lacks %s", ck, want)
		}
	}

	if code, _, _ := call(t, c, "PUT", e.srv.URL+"/api/settings/ui", `{"title":"Hemma"}`); code != 200 {
		t.Fatalf("logged in: %d", code)
	}
	if code, _, _ := call(t, c, "GET", e.srv.URL+"/api/settings/mqtt", ""); code != 200 {
		t.Fatalf("logged in: %d", code)
	}
	if _, v, _ := call(t, c, "GET", e.srv.URL+"/api/auth", ""); v["loggedIn"] != true {
		t.Fatalf("%v", v)
	}

	code, _, res = call(t, c, "POST", e.srv.URL+"/api/auth/logout", "")
	if code != 200 || !strings.Contains(res.Header.Get("Set-Cookie"), "Max-Age=0") {
		t.Fatalf("%d %q", code, res.Header.Get("Set-Cookie"))
	}
	if code, _, _ := call(t, c, "PUT", e.srv.URL+"/api/settings/ui", `{"title":"x"}`); code != 401 {
		t.Fatalf("logged out: %d", code)
	}
}

func TestForgedCookieIsRejected(t *testing.T) {
	e := newAuthEnv(t)
	req, _ := http.NewRequest("GET", e.srv.URL+"/api/settings/mqtt", nil)
	req.AddCookie(&http.Cookie{Name: "hc_session", Value: "YWRtaW58OTk5OTk5OTk5OQ.AAAA"})
	res, _ := http.DefaultClient.Do(req)
	res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatal(res.StatusCode)
	}
}

func TestTooManyWrongPasswords(t *testing.T) {
	e := newAuthEnv(t)
	c := client(t)
	for i := 0; i < 5; i++ {
		call(t, c, "POST", e.srv.URL+"/api/auth/login", `{"username":"admin","password":"gissning"}`)
	}
	code, v, _ := call(t, c, "POST", e.srv.URL+"/api/auth/login", `{"username":"admin","password":"hemligt"}`)
	if code != 429 || v["code"] != "too_many_attempts" {
		t.Fatalf("%d %v", code, v)
	}
}

func TestCORSAllowsCookies(t *testing.T) {
	e := newAuthEnv(t)
	req, _ := http.NewRequest("OPTIONS", e.srv.URL+"/api/settings/ui", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	res, _ := http.DefaultClient.Do(req)
	res.Body.Close()
	if res.Header.Get("Access-Control-Allow-Credentials") != "true" || res.StatusCode != 204 {
		t.Fatalf("%d %v", res.StatusCode, res.Header)
	}
}

func jsonDecode(res *http.Response, v any) error { return json.NewDecoder(res.Body).Decode(v) }

func wrongLogin(t *testing.T, url, forwardedFor string) int {
	req, _ := http.NewRequest("POST", url+"/api/auth/login", strings.NewReader(`{"username":"admin","password":"fel"}`))
	req.Header.Set("Content-Type", "application/json")
	if forwardedFor != "" {
		req.Header.Set("X-Forwarded-For", forwardedFor)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	return res.StatusCode
}

func TestBehindProxyEachClientHasItsOwnLimit(t *testing.T) {
	e := newEnvWith(t, func(o *Options) {
		o.Auth = auth.New("admin", "hemligt", []byte("0123456789abcdef0123456789abcdef"))
		o.TrustProxy = true // e.g. behind a Kubernetes Gateway: every request comes from its IP
	})
	// the proxy appends the address it saw: 10.0.0.9 is guessing passwords
	for i := 0; i < 5; i++ {
		wrongLogin(t, e.srv.URL, "10.0.0.9")
	}
	if code := wrongLogin(t, e.srv.URL, "10.0.0.7"); code != 401 {
		t.Fatalf("the tablet (10.0.0.7) must not be locked out by another device's guesses: %d", code)
	}
	// the guesser can't pretend to be someone else: only the last hop (added by our proxy) counts
	if code := wrongLogin(t, e.srv.URL, "10.0.0.7, 10.0.0.9"); code != 429 {
		t.Fatalf("a made-up first hop must not get around the limit: %d", code)
	}
}

func TestWithoutProxyForwardedForIsIgnored(t *testing.T) {
	e := newAuthEnv(t)
	for i := 0; i < 5; i++ {
		wrongLogin(t, e.srv.URL, "10.0.0.1")
	}
	if code := wrongLogin(t, e.srv.URL, "10.0.0.2"); code != 429 {
		t.Fatalf("a made-up X-Forwarded-For must not get around the limit: %d", code)
	}
}

func TestDevicesCanBeControlledWithoutLogin(t *testing.T) {
	e := newAuthEnv(t)
	e.reg.SetDevices([]devices.Device{
		{ID: "0x01", Name: "kitchen/1", Kind: devices.KindLight},
		{ID: "0x02", Name: "kitchen/2", Kind: devices.KindLight},
	})
	e.mq.connected = true

	if code, v := do(t, "POST", e.srv.URL+"/api/devices/0x01/set", `{"state":"ON","brightness":120}`); code != 202 {
		t.Fatalf("switching a lamp needs no login: %d %v", code, v)
	}
	if code, v := do(t, "POST", e.srv.URL+"/api/devices/set-many", `{"ids":["0x01","0x02"],"payload":{"state":"OFF"}}`); code != 202 {
		t.Fatalf("switching a group needs no login: %d %v", code, v)
	}
	if n := len(e.mq.sentCommands()); n != 3 {
		t.Fatalf("sent %d commands", n)
	}
}
