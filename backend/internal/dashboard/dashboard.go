// Package dashboard defines the user-editable front-page layout: columns of
// cards (clock, single device, group of devices).
package dashboard

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	CurrentVersion  = 1
	MaxColumns      = 6
	MaxCards        = 200
	MaxGroupDevices = 50
	MaxTitleLen     = 60
)

const (
	TypeClock    = "clock"
	TypeDevice   = "device"
	TypeGroup    = "group"
	TypeWeather  = "weather"
	TypeCalendar = "calendar"
)

type Dashboard struct {
	Version int      `json:"version"`
	Columns []Column `json:"columns"`
}

type Column struct {
	ID    string `json:"id"`
	Cards []Card `json:"cards"`
}

type Card struct {
	ID        string   `json:"id"`
	Type      string   `json:"type"`
	Title     string   `json:"title,omitempty"`     // optional for device, required for group
	DeviceID  string   `json:"deviceId,omitempty"`  // device card
	DeviceIDs []string `json:"deviceIds,omitempty"` // group card
}

// Normalize validates d and returns a cleaned copy (trimmed titles, no
// duplicate group members, non-nil card slices, current version).
func (d Dashboard) Normalize() (Dashboard, error) {
	if len(d.Columns) == 0 {
		return d, errors.New("dashboard needs at least one column")
	}
	if len(d.Columns) > MaxColumns {
		return d, fmt.Errorf("at most %d columns", MaxColumns)
	}
	out := Dashboard{Version: CurrentVersion, Columns: make([]Column, 0, len(d.Columns))}
	seen := map[string]bool{}
	total := 0
	for i, col := range d.Columns {
		if strings.TrimSpace(col.ID) == "" {
			return d, fmt.Errorf("column %d has no id", i+1)
		}
		nc := Column{ID: col.ID, Cards: make([]Card, 0, len(col.Cards))}
		for _, c := range col.Cards {
			total++
			if total > MaxCards {
				return d, fmt.Errorf("at most %d cards", MaxCards)
			}
			nc2, err := normalizeCard(c)
			if err != nil {
				return d, err
			}
			if seen[nc2.ID] {
				return d, fmt.Errorf("duplicate card id %q", nc2.ID)
			}
			seen[nc2.ID] = true
			nc.Cards = append(nc.Cards, nc2)
		}
		out.Columns = append(out.Columns, nc)
	}
	return out, nil
}

func normalizeCard(c Card) (Card, error) {
	if strings.TrimSpace(c.ID) == "" {
		return c, errors.New("card without id")
	}
	c.Title = strings.TrimSpace(c.Title)
	if utf8.RuneCountInString(c.Title) > MaxTitleLen {
		return c, fmt.Errorf("card title longer than %d characters", MaxTitleLen)
	}
	switch c.Type {
	case TypeClock, TypeWeather, TypeCalendar:
		c.DeviceID, c.DeviceIDs = "", nil
	case TypeDevice:
		if c.DeviceID == "" {
			return c, fmt.Errorf("device card %q has no deviceId", c.ID)
		}
		c.DeviceIDs = nil
	case TypeGroup:
		if c.Title == "" {
			return c, fmt.Errorf("group card %q needs a title", c.ID)
		}
		ids := make([]string, 0, len(c.DeviceIDs))
		seen := map[string]bool{}
		for _, id := range c.DeviceIDs {
			if id != "" && !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
		if len(ids) == 0 {
			return c, fmt.Errorf("group %q has no devices", c.Title)
		}
		if len(ids) > MaxGroupDevices {
			return c, fmt.Errorf("group %q has more than %d devices", c.Title, MaxGroupDevices)
		}
		c.DeviceIDs, c.DeviceID = ids, ""
	default:
		return c, fmt.Errorf("unknown card type %q", c.Type)
	}
	return c, nil
}
