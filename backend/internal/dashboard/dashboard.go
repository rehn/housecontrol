// Package dashboard defines the user-editable layout: tabs, each either a
// page of cards in columns (clock, device, group, weather, calendar, Grafana
// panel) or a whole Grafana dashboard.
package dashboard

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"
)

const (
	CurrentVersion  = 2 // 1 = a single page of columns, 2 = tabs
	MaxTabs         = 12
	MaxTabTitleLen  = 30
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
	TypeGrafana  = "grafana"
)

const (
	KindCards   = "cards"   // a page of cards
	KindGrafana = "grafana" // a whole Grafana dashboard
)

const (
	DefaultGrafanaHeight = 300
	MinGrafanaHeight     = 120
	MaxGrafanaHeight     = 1200
)

type Dashboard struct {
	Version int   `json:"version"`
	Tabs    []Tab `json:"tabs"`
	// Columns is the version 1 layout (one page, no tabs). Upgrade moves it
	// into the first tab; it is never written any more.
	Columns []Column `json:"columns,omitempty"`
}

type Tab struct {
	ID      string   `json:"id"`
	Title   string   `json:"title,omitempty"` // empty = the UI's default name
	Kind    string   `json:"kind"`
	Columns []Column `json:"columns,omitempty"` // kind cards
	URL     string   `json:"url,omitempty"`     // kind grafana
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
	URL       string   `json:"url,omitempty"`       // grafana card (panel embed)
	Height    int      `json:"height,omitempty"`    // grafana card, px
}

// Upgrade turns a version 1 layout (columns only) into one "home" tab.
func (d Dashboard) Upgrade() Dashboard {
	if len(d.Tabs) == 0 && d.Columns != nil {
		d.Tabs = []Tab{{ID: "home", Kind: KindCards, Columns: d.Columns}}
	}
	d.Columns = nil
	d.Version = CurrentVersion
	return d
}

// Normalize validates d and returns a cleaned copy (upgraded to tabs,
// trimmed titles, no duplicate group members, non-nil card slices).
func (d Dashboard) Normalize() (Dashboard, error) {
	d = d.Upgrade()
	if len(d.Tabs) == 0 {
		return d, errors.New("dashboard needs at least one tab")
	}
	if len(d.Tabs) > MaxTabs {
		return d, fmt.Errorf("at most %d tabs", MaxTabs)
	}
	out := Dashboard{Version: CurrentVersion, Tabs: make([]Tab, 0, len(d.Tabs))}
	tabIDs := map[string]bool{}
	cardIDs := map[string]bool{}
	total := 0
	for i, tab := range d.Tabs {
		if strings.TrimSpace(tab.ID) == "" {
			return d, fmt.Errorf("tab %d has no id", i+1)
		}
		if tabIDs[tab.ID] {
			return d, fmt.Errorf("duplicate tab id %q", tab.ID)
		}
		tabIDs[tab.ID] = true
		nt := Tab{ID: tab.ID, Title: strings.TrimSpace(tab.Title), Kind: tab.Kind}
		if utf8.RuneCountInString(nt.Title) > MaxTabTitleLen {
			return d, fmt.Errorf("tab title longer than %d characters", MaxTabTitleLen)
		}
		switch tab.Kind {
		case KindCards:
			cols, err := normalizeColumns(tab.Columns, cardIDs, &total)
			if err != nil {
				return d, err
			}
			nt.Columns = cols
		case KindGrafana:
			u, err := embedURL(tab.URL)
			if err != nil {
				return d, fmt.Errorf("tab %q: %w", tab.ID, err)
			}
			nt.URL = u
		default:
			return d, fmt.Errorf("unknown tab kind %q", tab.Kind)
		}
		out.Tabs = append(out.Tabs, nt)
	}
	return out, nil
}

func normalizeColumns(cols []Column, seen map[string]bool, total *int) ([]Column, error) {
	if len(cols) == 0 {
		return nil, errors.New("a page of cards needs at least one column")
	}
	if len(cols) > MaxColumns {
		return nil, fmt.Errorf("at most %d columns", MaxColumns)
	}
	out := make([]Column, 0, len(cols))
	for i, col := range cols {
		if strings.TrimSpace(col.ID) == "" {
			return nil, fmt.Errorf("column %d has no id", i+1)
		}
		nc := Column{ID: col.ID, Cards: make([]Card, 0, len(col.Cards))}
		for _, c := range col.Cards {
			*total++
			if *total > MaxCards {
				return nil, fmt.Errorf("at most %d cards", MaxCards)
			}
			nc2, err := normalizeCard(c)
			if err != nil {
				return nil, err
			}
			if seen[nc2.ID] {
				return nil, fmt.Errorf("duplicate card id %q", nc2.ID)
			}
			seen[nc2.ID] = true
			nc.Cards = append(nc.Cards, nc2)
		}
		out = append(out, nc)
	}
	return out, nil
}

// embedURL accepts only absolute http(s) links (they end up in an iframe).
func embedURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if raw == "" || err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", errors.New("needs a http(s) link to Grafana")
	}
	return raw, nil
}

func normalizeCard(c Card) (Card, error) {
	if strings.TrimSpace(c.ID) == "" {
		return c, errors.New("card without id")
	}
	c.Title = strings.TrimSpace(c.Title)
	if utf8.RuneCountInString(c.Title) > MaxTitleLen {
		return c, fmt.Errorf("card title longer than %d characters", MaxTitleLen)
	}
	if c.Type != TypeGrafana {
		c.URL, c.Height = "", 0
	}
	switch c.Type {
	case TypeClock, TypeWeather, TypeCalendar:
		c.DeviceID, c.DeviceIDs = "", nil
	case TypeGrafana:
		u, err := embedURL(c.URL)
		if err != nil {
			return c, fmt.Errorf("card %q: %w", c.ID, err)
		}
		c.URL, c.DeviceID, c.DeviceIDs = u, "", nil
		switch {
		case c.Height == 0:
			c.Height = DefaultGrafanaHeight
		case c.Height < MinGrafanaHeight:
			c.Height = MinGrafanaHeight
		case c.Height > MaxGrafanaHeight:
			c.Height = MaxGrafanaHeight
		}
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
