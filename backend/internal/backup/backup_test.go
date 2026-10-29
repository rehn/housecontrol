package backup

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func init() { Iterations = 1000 } // fast tests; real backups use DefaultIterations

func TestSealAndOpen(t *testing.T) {
	plain := []byte(`{"settings":{"mqtt":{"password":"hemligt"}},"note":"` + strings.Repeat("lampa ", 200) + `"}`)
	sealed, err := Seal(plain, "rätt lösenord")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(sealed, []byte(Magic)) {
		t.Fatal("missing file header")
	}
	if bytes.Contains(sealed, []byte("hemligt")) || bytes.Contains(sealed, []byte("lampa")) {
		t.Fatal("contents must not be readable")
	}
	if len(sealed) >= len(plain) {
		t.Fatalf("not compressed: %d >= %d", len(sealed), len(plain))
	}
	got, err := Open(sealed, "rätt lösenord")
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("%v %q", err, got)
	}
}

func TestEverySealIsDifferent(t *testing.T) {
	a, _ := Seal([]byte("x"), "password1")
	b, _ := Seal([]byte("x"), "password1")
	if bytes.Equal(a, b) {
		t.Fatal("salt and nonce must be random")
	}
}

func TestWrongPassword(t *testing.T) {
	sealed, _ := Seal([]byte("data"), "password1")
	if _, err := Open(sealed, "password2"); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("%v", err)
	}
}

func TestTamperedFileIsRejected(t *testing.T) {
	sealed, _ := Seal([]byte("some settings"), "password1")
	for _, i := range []int{len(Magic) + 2, len(sealed) - 1} { // header (iterations) and ciphertext
		bad := bytes.Clone(sealed)
		bad[i] ^= 0x01
		if _, err := Open(bad, "password1"); err == nil {
			t.Errorf("byte %d changed but the file was accepted", i)
		}
	}
	if _, err := Open(sealed[:len(sealed)-5], "password1"); err == nil {
		t.Error("cut file accepted")
	}
}

func TestNotABackup(t *testing.T) {
	for _, data := range [][]byte{nil, []byte("{}"), []byte(Magic), []byte("PK\x03\x04zipfile")} {
		if _, err := Open(data, "password1"); !errors.Is(err, ErrNotBackup) {
			t.Errorf("%q: %v", data, err)
		}
	}
}

func TestPasswordRules(t *testing.T) {
	if _, err := Seal([]byte("x"), "short"); !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("%v", err)
	}
}

func TestIsSealed(t *testing.T) {
	sealed, _ := Seal([]byte("x"), "password1")
	if !IsSealed(sealed) || IsSealed([]byte(`{"app":"housecontrol"}`)) || IsSealed(nil) {
		t.Fatal("IsSealed")
	}
}
