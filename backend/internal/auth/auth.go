// Package auth is the single admin login: password check, signed session
// tokens (kept in a cookie) and a limit on wrong guesses.
//
// Sessions are stateless: a token is "<user>|<expiry>" plus an HMAC. The
// signing key is derived from a random secret stored in the data directory
// *and* the password, so changing the password logs every device out.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// SessionTTL is how long a login lasts on a device.
const SessionTTL = 30 * 24 * time.Hour

type Auth struct {
	user string
	pass string
	key  []byte
	now  func() time.Time
}

// New returns the login for one admin. An empty password switches login off.
func New(username, password string, secret []byte) *Auth {
	user := strings.ToLower(strings.TrimSpace(username))
	if user == "" {
		user = "admin"
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte("session-v1|" + user + "|" + password))
	return &Auth{user: user, pass: password, key: mac.Sum(nil), now: time.Now}
}

// Enabled reports whether a password is set (login required to change things).
func (a *Auth) Enabled() bool { return a != nil && a.pass != "" }

// Username is the admin's user name.
func (a *Auth) Username() string { return a.user }

// Check compares user name and password in constant time.
func (a *Auth) Check(username, password string) bool {
	u := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(username))))
	wu := sha256.Sum256([]byte(a.user))
	p := sha256.Sum256([]byte(password))
	wp := sha256.Sum256([]byte(a.pass))
	okU := subtle.ConstantTimeCompare(u[:], wu[:])
	okP := subtle.ConstantTimeCompare(p[:], wp[:])
	return a.pass != "" && okU&okP == 1
}

// Issue creates a session token.
func (a *Auth) Issue() (string, time.Time) {
	exp := a.now().Add(SessionTTL)
	payload := a.user + "|" + strconv.FormatInt(exp.Unix(), 10)
	return enc([]byte(payload)) + "." + enc(a.sign(payload)), exp
}

// Verify checks a session token's signature and expiry.
func (a *Auth) Verify(token string) bool {
	p, s, ok := strings.Cut(token, ".")
	if !ok {
		return false
	}
	payload, err1 := base64.RawURLEncoding.DecodeString(p)
	sig, err2 := base64.RawURLEncoding.DecodeString(s)
	if err1 != nil || err2 != nil || !hmac.Equal(sig, a.sign(string(payload))) {
		return false
	}
	user, expStr, ok := strings.Cut(string(payload), "|")
	exp, err := strconv.ParseInt(expStr, 10, 64)
	return ok && err == nil && user == a.user && a.now().Before(time.Unix(exp, 0))
}

func (a *Auth) sign(payload string) []byte {
	mac := hmac.New(sha256.New, a.key)
	mac.Write([]byte(payload))
	return mac.Sum(nil)
}

func enc(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// LoadOrCreateSecret reads <dir>/session.key, creating it (32 random bytes,
// mode 0600) the first time, so logins survive restarts.
func LoadOrCreateSecret(dir string) ([]byte, error) {
	path := filepath.Join(dir, "session.key")
	if b, err := os.ReadFile(path); err == nil && len(b) == 32 {
		return b, nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return nil, fmt.Errorf("write %s: %w", path, err)
	}
	return b, nil
}

// Limiter refuses more than max wrong logins per client within window.
type Limiter struct {
	max    int
	window time.Duration
	now    func() time.Time

	mu    sync.Mutex
	fails map[string][]time.Time
}

func NewLimiter(max int, window time.Duration) *Limiter {
	return &Limiter{max: max, window: window, now: time.Now, fails: map[string][]time.Time{}}
}

// Allow reports whether the client may try to log in now.
func (l *Limiter) Allow(client string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.recent(client)) < l.max
}

// Fail records a wrong password.
func (l *Limiter) Fail(client string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fails[client] = append(l.recent(client), l.now())
}

// Reset forgets a client's failures (after a good login).
func (l *Limiter) Reset(client string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, client)
}

func (l *Limiter) recent(client string) []time.Time {
	cut := l.now().Add(-l.window)
	var keep []time.Time
	for _, t := range l.fails[client] {
		if t.After(cut) {
			keep = append(keep, t)
		}
	}
	if keep == nil {
		delete(l.fails, client)
	} else {
		l.fails[client] = keep
	}
	return keep
}
