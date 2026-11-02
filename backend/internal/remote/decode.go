// Package remote turns button presses on zigbee2mqtt remotes into actions on
// lights and outlets (or groups of them), chosen per button and per kind of
// press: a short press, a double press or holding the button.
//
// Remotes report presses as "action" values, and every brand names them
// differently (IKEA "arrow_left_hold", Hue "up_hold_release", Aqara
// "double"...). Decode maps them onto buttons and the press kinds here; values
// it doesn't recognise become buttons of their own with a short press, so any
// remote works at least for that.
package remote

import (
	"slices"
	"strings"
)

// Kinds of presses. Release ends a Hold and can't be chosen on its own.
const (
	Press   = "press"
	Double  = "double"
	Hold    = "hold"
	Release = "release"
)

// Events are the press kinds an action can be chosen for, in display order.
var Events = []string{Press, Double, Hold}

// Button is one button and the press kinds the remote reports for it.
type Button struct {
	ID     string   `json:"id"`
	Events []string `json:"events"`
}

var suffixes = []struct{ suffix, event string }{
	{"_press_release", Press}, // Hue: a short press, after "_press"
	{"_hold_release", Release},
	{"_long_release", Release},
	{"_short_release", Press}, // IKEA SOMRIG
	{"_initial_press", ""},    // the start of every press: wait for what it becomes
	{"_double_press", Double},
	{"_triple_press", ""},
	{"_long_press", Hold},
	{"_release", Release},
	{"_hold", Hold},
	{"_double", Double},
	{"_triple", ""},
	{"_quadruple", ""},
	{"_click", Press},
	{"_single", Press},
	{"_press", Press}, // unless the remote also sends "_press_release"
	{"_long", Hold},
}

func normalize(s string) string { return strings.ReplaceAll(s, "-", "_") }

// Decode says which button an action value is, and how it was pressed. The
// button is "" for "brightness_stop": it ends whichever button is held.
// ok is false for values that are only part of a press, or not the remote's.
func Decode(values []string, action string) (button, event string, ok bool) {
	if action == "" || !slices.Contains(values, action) {
		return "", "", false
	}
	b, e := parse(action, values)
	if e == "" {
		return "", "", false
	}
	return b, e, true
}

func parse(action string, values []string) (string, string) {
	has := func(v string) bool {
		return slices.ContainsFunc(values, func(x string) bool { return normalize(x) == v })
	}
	switch action {
	case "brightness_move_up": // IKEA: holding the "on" (or up) button
		if has("on") {
			return "on", Hold
		}
		return "up", Hold
	case "brightness_move_down":
		if has("off") {
			return "off", Hold
		}
		return "down", Hold
	case "brightness_stop":
		return "", Release
	case "single":
		return "button", Press
	case "double":
		return "button", Double
	case "hold", "long":
		return "button", Hold
	case "release":
		return "button", Release
	case "triple", "quadruple", "many":
		return "", ""
	}
	n := normalize(action)
	for _, s := range suffixes {
		if !strings.HasSuffix(n, s.suffix) || len(n) == len(s.suffix) {
			continue
		}
		base := strings.TrimSuffix(n, s.suffix)
		if s.suffix == "_press" && has(base+"_press_release") {
			return "", ""
		}
		return base, s.event
	}
	return action, Press
}

// Buttons lists a remote's buttons, in the order the remote reports them.
func Buttons(values []string) []Button {
	var order []string
	events := map[string]map[string]bool{}
	for _, v := range values {
		b, e := parse(v, values)
		if b == "" || e == "" || e == Release {
			continue
		}
		if events[b] == nil {
			events[b] = map[string]bool{}
			order = append(order, b)
		}
		events[b][e] = true
	}
	out := make([]Button, 0, len(order))
	for _, b := range order {
		btn := Button{ID: b}
		for _, e := range Events {
			if events[b][e] {
				btn.Events = append(btn.Events, e)
			}
		}
		out = append(out, btn)
	}
	return out
}
