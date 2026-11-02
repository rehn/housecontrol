package settings

import (
	"path/filepath"
	"testing"
	"time"
)

func TestAutoBackupNormalize(t *testing.T) {
	ok := []AutoBackup{{Time: "03:00", Keep: 14}, {Enabled: true, Time: " 23:59 ", Keep: 1}, {Time: "00:00", Keep: 365}}
	for _, a := range ok {
		if _, err := a.Normalize(); err != nil {
			t.Errorf("%+v: %v", a, err)
		}
	}
	bad := []AutoBackup{{Time: "3:00", Keep: 14}, {Time: "24:00", Keep: 14}, {Time: "", Keep: 14},
		{Time: "03:00", Keep: 0}, {Time: "03:00", Keep: 366}}
	for _, a := range bad {
		if _, err := a.Normalize(); err == nil {
			t.Errorf("%+v should be refused", a)
		}
	}
}

func TestAutoBackupTimes(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Stockholm")
	a := AutoBackup{Time: "03:00", Keep: 3}
	at := func(s string) time.Time { v, _ := time.ParseInLocation("2006-01-02 15:04", s, loc); return v }

	if got := a.Last(at("2026-10-30 14:00")); !got.Equal(at("2026-10-30 03:00")) {
		t.Errorf("last after: %v", got)
	}
	if got := a.Last(at("2026-10-30 02:59")); !got.Equal(at("2026-10-29 03:00")) {
		t.Errorf("last before: %v", got)
	}
	if got := a.Last(at("2026-10-30 03:00")); !got.Equal(at("2026-10-30 03:00")) {
		t.Errorf("last exactly: %v", got)
	}
	if got := a.Next(at("2026-10-30 03:00")); !got.Equal(at("2026-10-31 03:00")) {
		t.Errorf("next exactly: %v", got)
	}
	if got := a.Next(at("2026-10-30 01:00")); !got.Equal(at("2026-10-30 03:00")) {
		t.Errorf("next before: %v", got)
	}
	// the night summer time ends (25 h day) still runs at 03:00 local time
	if got := a.Next(at("2026-10-25 01:00")); got.Hour() != 3 || got.Day() != 25 {
		t.Errorf("dst: %v", got)
	}
}

func TestAutoBackupStored(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "d"))
	if a, err := s.LoadAutoBackup(); err != nil || a != DefaultAutoBackup() {
		t.Fatalf("default: %+v %v", a, err)
	}
	want := AutoBackup{Enabled: false, Time: "04:30", Keep: 7}
	if err := s.SaveAutoBackup(want); err != nil {
		t.Fatal(err)
	}
	if a, _ := s.LoadAutoBackup(); a != want {
		t.Fatalf("got %+v", a)
	}
	snap, _ := s.Snapshot()
	if snap.AutoBackup == nil || *snap.AutoBackup != want {
		t.Fatalf("snapshot %+v", snap.AutoBackup)
	}
}
