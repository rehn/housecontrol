package remote

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"slices"
	"strings"
	"sync"
	"time"
)

// What a button can do.
const (
	ActionOn       = "on"
	ActionOff      = "off"
	ActionToggle   = "toggle"   // a group goes off if any of it is on, else on
	ActionBrighter = "brighter" // one step (20 %) brighter, turning lights on
	ActionDarker   = "darker"
	ActionDimUp    = "dim_up" // while the button is held (Hold only)
	ActionDimDown  = "dim_down"
	ActionSet      = "set" // on at a brightness and/or colour temperature

	MaxBindings = 500
	MaxTargets  = 50
)

var actions = []string{ActionOn, ActionOff, ActionToggle, ActionBrighter, ActionDarker, ActionDimUp, ActionDimDown, ActionSet}

// Binding is what one kind of press on one button does.
type Binding struct {
	Remote  string   `json:"remote"` // the remote's device id (IEEE address)
	Button  string   `json:"button"`
	Event   string   `json:"event"` // press | double | hold
	Action  string   `json:"action"`
	Targets []string `json:"targets"` // "device:<id>" or "group:<group card id>"
	// ActionSet: brightness in % (0 = leave) and colour temperature in mired (0 = leave).
	Brightness int `json:"brightness,omitempty"`
	ColorTemp  int `json:"colorTemp,omitempty"`
}

// Normalize validates b and returns a cleaned copy.
func (b Binding) Normalize() (Binding, error) {
	if b.Remote == "" || b.Button == "" || len(b.Remote) > 64 || len(b.Button) > 64 {
		return b, errors.New("remote and button are needed")
	}
	if !slices.Contains(Events, b.Event) {
		return b, fmt.Errorf("unknown press %q", b.Event)
	}
	if !slices.Contains(actions, b.Action) {
		return b, fmt.Errorf("unknown action %q", b.Action)
	}
	if (b.Action == ActionDimUp || b.Action == ActionDimDown) && b.Event != Hold {
		return b, errors.New("dimming while held needs a hold")
	}
	var ts []string
	for _, t := range b.Targets {
		kind, id, _ := strings.Cut(t, ":")
		if (kind != "device" && kind != "group") || id == "" || len(t) > 100 {
			return b, fmt.Errorf("unknown target %q", t)
		}
		if !slices.Contains(ts, t) {
			ts = append(ts, t)
		}
	}
	if len(ts) == 0 || len(ts) > MaxTargets {
		return b, fmt.Errorf("choose 1-%d lights, outlets or groups", MaxTargets)
	}
	b.Targets = ts
	if b.Action == ActionSet {
		if b.Brightness < 0 || b.Brightness > 100 || (b.ColorTemp != 0 && (b.ColorTemp < 150 || b.ColorTemp > 500)) {
			return b, errors.New("brightness 1-100 %, colour temperature 150-500 mired")
		}
		if b.Brightness == 0 && b.ColorTemp == 0 {
			return b, errors.New("choose a brightness or colour temperature")
		}
	} else {
		b.Brightness, b.ColorTemp = 0, 0
	}
	return b, nil
}

// NormalizeAll validates a list: every press has at most one action.
func NormalizeAll(list []Binding) ([]Binding, error) {
	if len(list) > MaxBindings {
		return nil, fmt.Errorf("at most %d remote actions", MaxBindings)
	}
	out := make([]Binding, 0, len(list))
	seen := map[string]bool{}
	for _, b := range list {
		n, err := b.Normalize()
		if err != nil {
			return nil, err
		}
		key := n.Remote + "\x00" + n.Button + "\x00" + n.Event
		if seen[key] {
			return nil, fmt.Errorf("button %q has two actions for the same press", n.Button)
		}
		seen[key] = true
		out = append(out, n)
	}
	return out, nil
}

// Target is what the engine needs to know about a light or outlet.
type Target struct {
	Name          string // zigbee2mqtt friendly name
	StateProperty string
	On            bool
	Brightness    bool
	BrightnessMax int
	ColorTemp     bool
	ColorTempMin  int
	ColorTempMax  int
}

// Devices looks things up in the device list.
type Devices interface {
	Target(id string) (Target, bool)
	// Actions returns a remote's action values (false: not a remote).
	Actions(id string) ([]string, bool)
}

// Sender switches a device by its zigbee2mqtt name.
type Sender interface {
	Set(name string, payload map[string]any) error
}

// Pressed is reported for every press, also on buttons without an action
// (so Settings can show which button is which).
type Pressed struct {
	Remote string `json:"remote"`
	Button string `json:"button"`
	Event  string `json:"event"`
}

// holdRepeat: some remotes (Hue) repeat "hold" while the button is down.
const holdRepeat = 3 * time.Second

type held struct {
	button string
	at     time.Time
	moving []string // lights dimming until the release
}

// Engine carries out remote presses.
type Engine struct {
	load   func() []Binding
	groups func(id string) ([]string, bool) // a group card's devices
	devs   Devices
	send   Sender
	log    *slog.Logger
	now    func() time.Time
	// OnPress, if set, hears about every press.
	OnPress func(Pressed)

	mu   sync.Mutex
	held map[string]*held // by remote
}

func NewEngine(load func() []Binding, groups func(string) ([]string, bool), devs Devices, send Sender, log *slog.Logger) *Engine {
	return &Engine{load: load, groups: groups, devs: devs, send: send, log: log, now: time.Now, held: map[string]*held{}}
}

// Handle carries out one action value reported by a remote.
func (e *Engine) Handle(remoteID, action string) {
	values, ok := e.devs.Actions(remoteID)
	if !ok {
		return
	}
	button, event, ok := Decode(values, action)
	if !ok {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	now := e.now()
	h := e.held[remoteID]
	switch event {
	case Release:
		if h != nil && (button == "" || button == h.button) {
			e.stop(h.moving)
			delete(e.held, remoteID)
		}
		return
	case Hold:
		if h != nil && h.button == button && now.Sub(h.at) < holdRepeat {
			h.at = now // still the same hold
			return
		}
		if h != nil {
			e.stop(h.moving)
		}
		h = &held{button: button, at: now}
		e.held[remoteID] = h
	default:
		delete(e.held, remoteID)
		h = nil
	}
	if e.OnPress != nil {
		e.OnPress(Pressed{Remote: remoteID, Button: button, Event: event})
	}
	for _, b := range e.load() {
		if b.Remote == remoteID && b.Button == button && b.Event == event {
			moving := e.run(b)
			if h != nil {
				h.moving = moving
			}
			return
		}
	}
}

func (e *Engine) targets(b Binding) []Target {
	var out []Target
	seen := map[string]bool{}
	add := func(id string) {
		t, ok := e.devs.Target(id)
		if !ok {
			e.log.Warn("remote: device not found", "remote", b.Remote, "button", b.Button, "id", id)
			return
		}
		if !seen[t.Name] {
			seen[t.Name] = true
			out = append(out, t)
		}
	}
	for _, tg := range b.Targets {
		kind, id, _ := strings.Cut(tg, ":")
		if kind == "device" {
			add(id)
			continue
		}
		ids, ok := e.groups(id)
		if !ok {
			e.log.Warn("remote: group not found", "remote", b.Remote, "button", b.Button, "group", id)
			continue
		}
		for _, d := range ids {
			add(d)
		}
	}
	return out
}

// run carries out a binding; it returns the lights it started dimming.
func (e *Engine) run(b Binding) []string {
	ts := e.targets(b)
	var moving []string
	anyOn := slices.ContainsFunc(ts, func(t Target) bool { return t.On })
	for _, t := range ts {
		prop := t.StateProperty
		if prop == "" {
			prop = "state"
		}
		max := t.BrightnessMax
		if max <= 0 {
			max = 254
		}
		var p map[string]any
		switch b.Action {
		case ActionOn:
			p = map[string]any{prop: "ON"}
		case ActionOff:
			p = map[string]any{prop: "OFF"}
		case ActionToggle:
			p = map[string]any{prop: map[bool]string{true: "OFF", false: "ON"}[anyOn]}
		case ActionBrighter, ActionDarker:
			if !t.Brightness {
				continue
			}
			step := int(math.Round(float64(max) * 0.2))
			if b.Action == ActionBrighter {
				p = map[string]any{"brightness_step_onoff": step}
			} else {
				p = map[string]any{"brightness_step": -step}
			}
		case ActionDimUp, ActionDimDown:
			if !t.Brightness {
				continue
			}
			rate := int(math.Round(float64(max) / 4)) // the whole range in about 4 s
			if b.Action == ActionDimUp {
				p = map[string]any{"brightness_move_onoff": rate}
			} else {
				p = map[string]any{"brightness_move": -rate}
			}
			moving = append(moving, t.Name)
		case ActionSet:
			p = map[string]any{prop: "ON"}
			if b.Brightness > 0 && t.Brightness {
				p["brightness"] = int(math.Max(1, math.Round(float64(b.Brightness)*float64(max)/100)))
			}
			if b.ColorTemp > 0 && t.ColorTemp {
				ct := b.ColorTemp
				if t.ColorTempMin > 0 && ct < t.ColorTempMin {
					ct = t.ColorTempMin
				}
				if t.ColorTempMax > 0 && ct > t.ColorTempMax {
					ct = t.ColorTempMax
				}
				p["color_temp"] = ct
			}
		}
		if err := e.send.Set(t.Name, p); err != nil {
			e.log.Warn("remote: could not switch", "device", t.Name, "err", err)
		}
	}
	return moving
}

func (e *Engine) stop(names []string) {
	for _, n := range names {
		if err := e.send.Set(n, map[string]any{"brightness_move": "stop"}); err != nil {
			e.log.Warn("remote: could not stop dimming", "device", n, "err", err)
		}
	}
}
