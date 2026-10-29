package api

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"
)

const sessionCookie = "hc_session"

// requireLogin lets anyone look and switch devices (lights, outlets, groups,
// brightness), but editing the home page and all settings need a login.
func (s *Server) requireLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.opts.Auth.Enabled() && needsLogin(r) && !s.loggedIn(r) {
			writeErrCode(w, http.StatusUnauthorized, codeUnauthorized, "log in to do this")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func needsLogin(r *http.Request) bool {
	p := r.URL.Path
	if !strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/api/auth") {
		return false // the web app itself, and logging in
	}
	if r.Method == http.MethodPost && strings.HasPrefix(p, "/api/devices/") {
		return false // controlling devices is for everyone in the house
	}
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		// settings and the place/position helpers used while editing them
		return (strings.HasPrefix(p, "/api/settings/") && p != "/api/settings/ui") ||
			strings.HasPrefix(p, "/api/weather/")
	case http.MethodOptions:
		return false
	}
	return true // every change
}

func (s *Server) loggedIn(r *http.Request) bool {
	if !s.opts.Auth.Enabled() {
		return true
	}
	c, err := r.Cookie(sessionCookie)
	return err == nil && s.opts.Auth.Verify(c.Value)
}

func (s *Server) authView(r *http.Request, loggedIn bool) map[string]any {
	v := map[string]any{"required": s.opts.Auth.Enabled(), "loggedIn": loggedIn}
	if loggedIn && s.opts.Auth.Enabled() {
		v["username"] = s.opts.Auth.Username()
	}
	return v
}

func (s *Server) authStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.authView(r, s.loggedIn(r)))
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if !s.opts.Auth.Enabled() {
		writeJSON(w, http.StatusOK, s.authView(r, true))
		return
	}
	client := s.clientIP(r)
	if !s.limiter.Allow(client) {
		writeErrCode(w, http.StatusTooManyRequests, codeTooManyAttempts, "too many wrong passwords, wait a few minutes")
		return
	}
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if !s.opts.Auth.Check(in.Username, in.Password) {
		s.limiter.Fail(client)
		s.log.Warn("wrong password", "client", client)
		time.Sleep(300 * time.Millisecond) // slows down guessing a little more
		writeErrCode(w, http.StatusUnauthorized, codeBadLogin, "wrong user name or password")
		return
	}
	s.limiter.Reset(client)
	token, exp := s.opts.Auth.Issue()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  exp,
		MaxAge:   int(time.Until(exp).Round(time.Second).Seconds()),
		HttpOnly: true,
		Secure:   secure(r),
		SameSite: http.SameSiteLaxMode,
	})
	s.log.Info("logged in", "client", client)
	writeJSON(w, http.StatusOK, s.authView(r, true))
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: secure(r), SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, s.authView(r, !s.opts.Auth.Enabled()))
}

func secure(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func (s *Server) clientIP(r *http.Request) string {
	if s.opts.TrustProxy {
		// our proxy appends the address it saw last; earlier entries are client-supplied
		if xff := r.Header.Values("X-Forwarded-For"); len(xff) > 0 {
			hops := strings.Split(xff[len(xff)-1], ",")
			if last := strings.TrimSpace(hops[len(hops)-1]); last != "" {
				return last
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
