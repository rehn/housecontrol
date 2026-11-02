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

	MaxRemotes = 100
)

var actions = []string{ActionOn, ActionOff, ActionToggle, ActionBrighter, ActionDarker, ActionDimUp, ActionDimDown, ActionSet}

// Config is one remote: what it controls (a group or a light/outlet), and
// what each kind of press on each button does to it.
type Config struct {
	Remote  string   `json:"remote"` // the remote's device id (IEEE address)
	Target  string   `json:"target"` // "device:<id>" or "group:<group card id>"
	Actions []Action `json:"actions"`
}

// Action is what one kind of press on one button does.
type Action struct {
	Button string `json:"button"`
	Event  string `json:"event"` // press | double | hold
	Action string `json:"action"`
	// ActionSet: brightness in % (0 = leave) and colour temperature in mired (0 = leave).
	Brightness int `json:"brightness,omitempty"`
	ColorTemp  int `json:"colorTemp,omitempty"`
}

// Binding is the first format (an action with its own targets per button),
// read only to move old settings over (see Migrate).
type Binding struct {
	Remote     string   `json:"remote"`
	Button     string   `json:"button"`
	Event      string   `json:"event"`
	Action     string   `json:"action"`
	Targets    []string `json:"targets"`
	Brightness int      `json:"brightness,omitempty"`
	ColorTemp  int      `json:"colorTemp,omitempty"`
}

func validTarget(t string) bool {
	kind, id, _ := strings.Cut(t, ":")
	return (kind == "device" || kind == "group") && id != "" && len(t) <= 100
}

// Normalize validates a and returns a cleaned copy.
func (a Action) Normalize() (Action, error) {
	if a.Button == "" || len(a.Button) > 64 {
		return a, errors.New("button missing")
	}
	if !slices.Contains(Events, a.Event) {
		return a, fmt.Errorf("unknown press %q", a.Event)
	}
	if !slices.Contains(actions, a.Action) {
		return a, fmt.Errorf("unknown action %q", a.Action)
	}
	if (a.Action == ActionDimUp || a.Action == ActionDimDown) && a.Event != Hold {
		return a, errors.New("dimming while held needs a hold")
	}
	if a.Action == ActionSet {
		if a.Brightness < 0 || a.Brightness > 100 || (a.ColorTemp != 0 && (a.ColorTemp < 150 || a.ColorTemp > 500)) {
			return a, errors.New("brightness 1-100 %, colour temperature 150-500 mired")
		}
		if a.Brightness == 0 && a.ColorTemp == 0 {
			return a, errors.New("choose a brightness or colour temperature")
		}
	} else {
		a.Brightness, a.ColorTemp = 0, 0
	}
	return a, nil
}

// Normalize validates c and returns a cleaned copy.
func (c Config) Normalize() (Config, error) {
	if c.Remote == "" || len(c.Remote) > 64 {
		return c, errors.New("remote missing")
	}
	if !validTarget(c.Target) {
		return c, fmt.Errorf("choose a group or a light/outlet (got %q)", c.Target)
	}
	out := make([]Action, 0, len(c.Actions))
	seen := map[string]bool{}
	for _, a := range c.Actions {
		n, err := a.Normalize()
		if err != nil {
			return c, err
		}
		key := n.Button + "\x00" + n.Event
		if seen[key] {
			return c, fmt.Errorf("button %q has two actions for the same press", n.Button)
		}
		seen[key] = true
		out = append(out, n)
	}
	c.Actions = out
	return c, nil
}

// NormalizeConfigs validates a list: each remote once.
func NormalizeConfigs(list []Config) ([]Config, error) {
	if len(list) > MaxRemotes {
		return nil, fmt.Errorf("at most %d remotes", MaxRemotes)
	}
	out := make([]Config, 0, len(list))
	seen := map[string]bool{}
	for _, c := range list {
		n, err := c.Normalize()
		if err != nil {
			return nil, err
		}
		if seen[n.Remote] {
			return nil, errors.New("the same remote twice")
		}
		seen[n.Remote] = true
		out = append(out, n)
	}
	return out, nil
}

// Migrate turns first-format bindings into one config per remote; its target
// is the first one chosen for it.
func Migrate(old []Binding) []Config {
	var out []Config
	idx := map[string]int{}
	for _, b := range old {
		i, ok := idx[b.Remote]
		if !ok {
			i = len(out)
			idx[b.Remote] = i
			out = append(out, Config{Remote: b.Remote, Actions: []Action{}})
		}
		if out[i].Target == "" && len(b.Targets) > 0 {
			out[i].Target = b.Targets[0]
		}
		out[i].Actions = append(out[i].Actions, Action{Button: b.Button, Event: b.Event, Action: b.Action,
			Brightness: b.Brightness, ColorTemp: b.ColorTemp})
	}
	return out
}

// Defaults are what a remote's buttons do until changed: up/on switches on
// (dims up while held), down/off switches off (dims down while held). With
// both on/off and up/down buttons (Hue), up/down step and dim instead. A
// middle or single button toggles.
func Defaults(buttons []Button) []Action {
	has := func(id string) bool {
		return slices.ContainsFunc(buttons, func(b Button) bool { return b.ID == id })
	}
	hasOnOff := has("on") || has("off")
	hasUpDown := has("up") || has("down") || has("brightness_up") || has("brightness_down")
	var out []Action
	for _, b := range buttons {
		var press, hold string
		switch b.ID {
		case "on":
			press = ActionOn
			if !hasUpDown {
				hold = ActionDimUp
			}
		case "off":
			press = ActionOff
			if !hasUpDown {
				hold = ActionDimDown
			}
		case "up", "brightness_up":
			press, hold = ActionOn, ActionDimUp
			if hasOnOff {
				press = ActionBrighter
			}
		case "down", "brightness_down":
			press, hold = ActionOff, ActionDimDown
			if hasOnOff {
				press = ActionDarker
			}
		case "1":
			if has("2") {
				press, hold = ActionOn, ActionDimUp
			}
		case "2":
			if has("1") {
				press, hold = ActionOff, ActionDimDown
			}
		case "toggle", "button":
			press = ActionToggle
		}
		if press != "" && slices.Contains(b.Events, Press) {
			out = append(out, Action{Button: b.ID, Event: Press, Action: press})
		}
		if hold != "" && slices.Contains(b.Events, Hold) {
			out = append(out, Action{Button: b.ID, Event: Hold, Action: hold})
		}
	}
	return out
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
	load   func() []Config
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

func NewEngine(load func() []Config, groups func(string) ([]string, bool), devs Devices, send Sender, log *slog.Logger) *Engine {
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
	for _, c := range e.load() {
		if c.Remote != remoteID {
			continue
		}
		for _, a := range c.Actions {
			if a.Button == button && a.Event == event {
				moving := e.run(c, a)
				if h != nil {
					h.moving = moving
				}
				return
			}
		}
		return
	}
}

func (e *Engine) targets(c Config) []Target {
	var out []Target
	seen := map[string]bool{}
	add := func(id string) {
		t, ok := e.devs.Target(id)
		if !ok {
			e.log.Warn("remote: device not found", "remote", c.Remote, "id", id)
			return
		}
		if !seen[t.Name] {
			seen[t.Name] = true
			out = append(out, t)
		}
	}
	kind, id, _ := strings.Cut(c.Target, ":")
	if kind == "device" {
		add(id)
		return out
	}
	ids, ok := e.groups(id)
	if !ok {
		e.log.Warn("remote: group not found", "remote", c.Remote, "group", id)
		return nil
	}
	for _, d := range ids {
		add(d)
	}
	return out
}

// run carries out an action; it returns the lights it started dimming.
func (e *Engine) run(c Config, b Action) []string {
	ts := e.targets(c)
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
