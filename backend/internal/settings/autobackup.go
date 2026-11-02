package settings

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	MinBackupKeep = 1
	MaxBackupKeep = 365
)

// AutoBackup is when backups are made automatically, and how many are kept.
type AutoBackup struct {
	Enabled bool   `json:"enabled"`
	Time    string `json:"time"` // HH:MM, every day, local time
	Keep    int    `json:"keep"` // the newest Keep backups are kept, older ones deleted
}

// DefaultAutoBackup: on, every night at 03:00, two weeks kept.
func DefaultAutoBackup() AutoBackup {
	return AutoBackup{Enabled: true, Time: "03:00", Keep: 14}
}

// Normalize validates a and returns a cleaned copy.
func (a AutoBackup) Normalize() (AutoBackup, error) {
	a.Time = strings.TrimSpace(a.Time)
	if _, err := time.Parse("15:04", a.Time); err != nil || len(a.Time) != 5 {
		return a, errors.New("time must be HH:MM")
	}
	if a.Keep < MinBackupKeep || a.Keep > MaxBackupKeep {
		return a, fmt.Errorf("keep %d-%d backups", MinBackupKeep, MaxBackupKeep)
	}
	return a, nil
}

// At is when the backup runs on the day of t (in t's time zone).
func (a AutoBackup) At(t time.Time) time.Time {
	c, _ := time.Parse("15:04", a.Time)
	y, m, d := t.Date()
	return time.Date(y, m, d, c.Hour(), c.Minute(), 0, 0, t.Location())
}

// Last is the latest planned run at or before now.
func (a AutoBackup) Last(now time.Time) time.Time {
	at := a.At(now)
	if at.After(now) {
		at = a.At(now.AddDate(0, 0, -1))
	}
	return at
}

// Next is the first planned run after now.
func (a AutoBackup) Next(now time.Time) time.Time {
	at := a.At(now)
	if !at.After(now) {
		at = a.At(now.AddDate(0, 0, 1))
	}
	return at
}
