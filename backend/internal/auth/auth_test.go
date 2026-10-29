package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var secret = []byte("0123456789abcdef0123456789abcdef")

func TestDisabledWithoutPassword(t *testing.T) {
	if New("admin", "", secret).Enabled() {
		t.Fatal("no password = login switched off")
	}
	if !New("admin", "pw", secret).Enabled() {
		t.Fatal("password set = login on")
	}
}

func TestCheck(t *testing.T) {
	a := New("admin", "hemligt", secret)
	for _, c := range []struct {
		user, pass string
		ok         bool
	}{
		{"admin", "hemligt", true},
		{"Admin", "hemligt", true}, // user name is not case sensitive
		{" admin ", "hemligt", true},
		{"admin", "Hemligt", false},
		{"admin", "", false},
		{"lars", "hemligt", false},
	} {
		if got := a.Check(c.user, c.pass); got != c.ok {
			t.Errorf("Check(%q,%q) = %v", c.user, c.pass, got)
		}
	}
}

func TestSessionTokens(t *testing.T) {
	a := New("admin", "hemligt", secret)
	now := time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC)
	a.now = func() time.Time { return now }

	tok, exp := a.Issue()
	if exp != now.Add(30*24*time.Hour) {
		t.Fatalf("expires %v", exp)
	}
	if !a.Verify(tok) {
		t.Fatal("fresh token should be valid")
	}
	if a.Verify(tok+"x") || a.Verify("") || a.Verify("nonsense") || a.Verify(strings.Replace(tok, ".", "", 1)) {
		t.Fatal("tampered tokens must fail")
	}

	// a token from another server (other secret) fails
	if New("admin", "hemligt", []byte("another-secret-another-secret-xx")).Verify(tok) {
		t.Fatal("other secret")
	}
	// changing the password logs everybody out
	if New("admin", "nytt", secret).Verify(tok) {
		t.Fatal("old sessions must end when the password changes")
	}

	now = now.Add(30*24*time.Hour + time.Second)
	if a.Verify(tok) {
		t.Fatal("expired token must fail")
	}
}

func TestLimiter(t *testing.T) {
	now := time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC)
	l := NewLimiter(5, 5*time.Minute)
	l.now = func() time.Time { return now }
	for i := 0; i < 5; i++ {
		if !l.Allow("10.0.0.2") {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
		l.Fail("10.0.0.2")
	}
	if l.Allow("10.0.0.2") {
		t.Fatal("6th attempt within 5 minutes should be refused")
	}
	if !l.Allow("10.0.0.3") {
		t.Fatal("other devices are not affected")
	}
	now = now.Add(5*time.Minute + time.Second)
	if !l.Allow("10.0.0.2") {
		t.Fatal("allowed again after the window")
	}
	l.Fail("10.0.0.2")
	l.Reset("10.0.0.2")
	for i := 0; i < 5; i++ {
		l.Fail("10.0.0.4")
	}
	if !l.Allow("10.0.0.2") {
		t.Fatal("a good login clears the failures")
	}
}

func TestSecretIsCreatedOnceAndPrivate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	s1, err := LoadOrCreateSecret(dir)
	if err != nil || len(s1) != 32 {
		t.Fatalf("%v %d", err, len(s1))
	}
	s2, _ := LoadOrCreateSecret(dir)
	if string(s1) != string(s2) {
		t.Fatal("secret must survive restarts (else everyone is logged out)")
	}
	fi, err := os.Stat(filepath.Join(dir, "session.key"))
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("%v %v", err, fi.Mode())
	}
}
