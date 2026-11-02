package remote

import (
	"io"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"
)

type sent struct {
	name    string
	payload map[string]any
}

type fakeSender struct{ log []sent }

func (f *fakeSender) Set(name string, p map[string]any) error {
	f.log = append(f.log, sent{name, p})
	return nil
}

type fakeDevices struct {
	targets map[string]Target
	remotes map[string][]string
}

func (f *fakeDevices) Target(id string) (Target, bool) { t, ok := f.targets[id]; return t, ok }
func (f *fakeDevices) Actions(id string) ([]string, bool) {
	v, ok := f.remotes[id]
	return v, ok
}

type env struct {
	e        *Engine
	send     *fakeSender
	devs     *fakeDevices
	bindings []Binding
	presses  []Pressed
	now      time.Time
}

func newEngine(t *testing.T) *env {
	t.Helper()
	v := &env{send: &fakeSender{}, now: time.Date(2026, 11, 2, 18, 0, 0, 0, time.UTC)}
	v.devs = &fakeDevices{
		targets: map[string]Target{
			"tak":   {Name: "Taklampa", StateProperty: "state", Brightness: true, BrightnessMax: 254, ColorTemp: true, ColorTempMin: 250, ColorTempMax: 454},
			"golv":  {Name: "Golvlampa", StateProperty: "state", Brightness: true, BrightnessMax: 254},
			"tv":    {Name: "TV-uttag", StateProperty: "state"},
			"gamla": {Name: "Gammal", StateProperty: "state_l1"},
		},
		remotes: map[string][]string{"styrbar": styrbar, "hue": hueDimmer, "round": tradfriRound},
	}
	groups := func(id string) ([]string, bool) {
		if id == "vardagsrum" {
			return []string{"tak", "golv", "tv"}, true
		}
		return nil, false
	}
	v.e = NewEngine(func() []Binding { return v.bindings }, groups, v.devs, v.send,
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	v.e.now = func() time.Time { return v.now }
	v.e.OnPress = func(p Pressed) { v.presses = append(v.presses, p) }
	return v
}

func (v *env) payloads() string {
	var out []string
	for _, s := range v.send.log {
		var kv []string
		for _, k := range []string{"state", "state_l1", "brightness", "color_temp", "brightness_step", "brightness_step_onoff", "brightness_move", "brightness_move_onoff"} {
			if x, ok := s.payload[k]; ok {
				kv = append(kv, k+"="+toS(x))
			}
		}
		out = append(out, s.name+":"+strings.Join(kv, ","))
	}
	v.send.log = nil
	return strings.Join(out, " ")
}

func toS(x any) string {
	switch y := x.(type) {
	case string:
		return y
	case int:
		return itoa(y)
	}
	return "?"
}

func itoa(i int) string {
	neg := i < 0
	if neg {
		i = -i
	}
	s := ""
	for {
		s = string(rune('0'+i%10)) + s
		i /= 10
		if i == 0 {
			break
		}
	}
	if neg {
		s = "-" + s
	}
	return s
}

func TestPressSwitchesTargets(t *testing.T) {
	v := newEngine(t)
	v.bindings = []Binding{
		{Remote: "styrbar", Button: "on", Event: Press, Action: ActionOn, Targets: []string{"device:tak", "device:tv"}},
		{Remote: "styrbar", Button: "off", Event: Press, Action: ActionOff, Targets: []string{"group:vardagsrum"}},
	}
	v.e.Handle("styrbar", "on")
	if got := v.payloads(); got != "Taklampa:state=ON TV-uttag:state=ON" {
		t.Fatalf("on: %s", got)
	}
	v.e.Handle("styrbar", "off")
	if got := v.payloads(); got != "Taklampa:state=OFF Golvlampa:state=OFF TV-uttag:state=OFF" {
		t.Fatalf("off group: %s", got)
	}
	if len(v.presses) != 2 || v.presses[1] != (Pressed{Remote: "styrbar", Button: "off", Event: Press}) {
		t.Fatalf("presses %+v", v.presses)
	}
}

func TestToggleAGroupTogether(t *testing.T) {
	v := newEngine(t)
	v.bindings = []Binding{{Remote: "round", Button: "toggle", Event: Press, Action: ActionToggle, Targets: []string{"group:vardagsrum"}}}
	v.e.Handle("round", "toggle") // all off: all on
	if got := v.payloads(); got != "Taklampa:state=ON Golvlampa:state=ON TV-uttag:state=ON" {
		t.Fatalf("all off: %s", got)
	}
	tak := v.devs.targets["tak"]
	tak.On = true
	v.devs.targets["tak"] = tak
	v.e.Handle("round", "toggle") // one on: all off
	if got := v.payloads(); got != "Taklampa:state=OFF Golvlampa:state=OFF TV-uttag:state=OFF" {
		t.Fatalf("one on: %s", got)
	}
}

func TestHoldDimsUntilRelease(t *testing.T) {
	v := newEngine(t)
	v.bindings = []Binding{
		{Remote: "styrbar", Button: "on", Event: Hold, Action: ActionDimUp, Targets: []string{"group:vardagsrum"}},
		{Remote: "styrbar", Button: "off", Event: Hold, Action: ActionDimDown, Targets: []string{"device:tak"}},
	}
	v.e.Handle("styrbar", "brightness_move_up")
	if got := v.payloads(); got != "Taklampa:brightness_move_onoff=64 Golvlampa:brightness_move_onoff=64" {
		t.Fatalf("dim up (outlets can't dim): %s", got)
	}
	v.e.Handle("styrbar", "brightness_stop")
	if got := v.payloads(); got != "Taklampa:brightness_move=stop Golvlampa:brightness_move=stop" {
		t.Fatalf("stop: %s", got)
	}
	v.e.Handle("styrbar", "brightness_move_down")
	if got := v.payloads(); got != "Taklampa:brightness_move=-64" {
		t.Fatalf("dim down: %s", got)
	}
	v.e.Handle("styrbar", "brightness_stop")
	if got := v.payloads(); got != "Taklampa:brightness_move=stop" {
		t.Fatalf("stop 2: %s", got)
	}
	v.e.Handle("styrbar", "brightness_stop") // nothing held any more
	if got := v.payloads(); got != "" {
		t.Fatalf("stray stop: %s", got)
	}
}

func TestRepeatedHoldMessagesActOnce(t *testing.T) {
	v := newEngine(t)
	v.bindings = []Binding{
		{Remote: "hue", Button: "on", Event: Hold, Action: ActionToggle, Targets: []string{"device:tv"}},
		{Remote: "hue", Button: "on", Event: Press, Action: ActionOn, Targets: []string{"device:tak"}},
	}
	v.e.Handle("hue", "on_press") // the start of a press: nothing yet
	if got := v.payloads(); got != "" || len(v.presses) != 0 {
		t.Fatalf("on_press: %s %v", got, v.presses)
	}
	v.e.Handle("hue", "on_hold")
	v.now = v.now.Add(800 * time.Millisecond)
	v.e.Handle("hue", "on_hold") // Hue repeats "hold" while the button is down
	v.now = v.now.Add(800 * time.Millisecond)
	v.e.Handle("hue", "on_hold")
	if got := v.payloads(); got != "TV-uttag:state=ON" {
		t.Fatalf("hold once: %s", got)
	}
	v.e.Handle("hue", "on_hold_release")
	v.e.Handle("hue", "on_press")
	v.e.Handle("hue", "on_press_release")
	if got := v.payloads(); got != "Taklampa:state=ON" {
		t.Fatalf("short press: %s", got)
	}
	v.now = v.now.Add(time.Second)
	v.e.Handle("hue", "on_hold") // a new hold after the release
	if got := v.payloads(); got != "TV-uttag:state=ON" {
		t.Fatalf("second hold: %s", got)
	}
}

func TestStepsAndSet(t *testing.T) {
	v := newEngine(t)
	v.bindings = []Binding{
		{Remote: "round", Button: "brightness_up", Event: Press, Action: ActionBrighter, Targets: []string{"device:tak", "device:tv"}},
		{Remote: "round", Button: "brightness_down", Event: Press, Action: ActionDarker, Targets: []string{"device:tak"}},
		{Remote: "round", Button: "arrow_left", Event: Press, Action: ActionSet, Targets: []string{"group:vardagsrum"}, Brightness: 30, ColorTemp: 500},
	}
	v.e.Handle("round", "brightness_up_click")
	if got := v.payloads(); got != "Taklampa:brightness_step_onoff=51" {
		t.Fatalf("brighter: %s", got)
	}
	v.e.Handle("round", "brightness_down_click")
	if got := v.payloads(); got != "Taklampa:brightness_step=-51" {
		t.Fatalf("darker: %s", got)
	}
	v.e.Handle("round", "arrow_left_click")
	// colour temperature only where supported, and within the lamp's range
	if got := v.payloads(); got != "Taklampa:state=ON,brightness=76,color_temp=454 Golvlampa:state=ON,brightness=76 TV-uttag:state=ON" {
		t.Fatalf("set: %s", got)
	}
}

func TestUnknownThingsAreIgnored(t *testing.T) {
	v := newEngine(t)
	v.bindings = []Binding{
		{Remote: "styrbar", Button: "on", Event: Press, Action: ActionOn, Targets: []string{"device:borta", "group:borta", "device:gamla"}},
	}
	v.e.Handle("okänd", "on")                  // not a remote
	v.e.Handle("styrbar", "nonsens")           // not one of its actions
	v.e.Handle("styrbar", "arrow_right_click") // a button without a binding: only reported
	if got := v.payloads(); got != "" {
		t.Fatalf("%s", got)
	}
	v.e.Handle("styrbar", "on") // missing targets are skipped, others still switch
	if got := v.payloads(); got != "Gammal:state_l1=ON" {
		t.Fatalf("%s", got)
	}
	if !reflect.DeepEqual(v.presses[0], Pressed{Remote: "styrbar", Button: "arrow_right", Event: Press}) {
		t.Fatalf("%+v", v.presses)
	}
}

func TestNormalizeBindings(t *testing.T) {
	ok := []Binding{
		{Remote: "r", Button: "on", Event: Press, Action: ActionToggle, Targets: []string{"device:a", "group:g", "device:a"}},
		{Remote: "r", Button: "on", Event: Hold, Action: ActionDimUp, Targets: []string{"group:g"}},
		{Remote: "r", Button: "1", Event: Double, Action: ActionSet, Targets: []string{"device:a"}, Brightness: 50},
		{Remote: "r", Button: "2", Event: Press, Action: ActionSet, Targets: []string{"device:a"}, ColorTemp: 370},
	}
	got, err := NormalizeAll(ok)
	if err != nil {
		t.Fatal(err)
	}
	if len(got[0].Targets) != 2 {
		t.Fatalf("duplicate targets: %v", got[0].Targets)
	}
	bad := []Binding{
		{Remote: "", Button: "on", Event: Press, Action: ActionOn, Targets: []string{"device:a"}},
		{Remote: "r", Button: "", Event: Press, Action: ActionOn, Targets: []string{"device:a"}},
		{Remote: "r", Button: "on", Event: Release, Action: ActionOn, Targets: []string{"device:a"}},
		{Remote: "r", Button: "on", Event: Press, Action: "explode", Targets: []string{"device:a"}},
		{Remote: "r", Button: "on", Event: Press, Action: ActionOn},
		{Remote: "r", Button: "on", Event: Press, Action: ActionOn, Targets: []string{"lamp:a"}},
		{Remote: "r", Button: "on", Event: Press, Action: ActionDimUp, Targets: []string{"device:a"}}, // dimming needs hold
		{Remote: "r", Button: "on", Event: Press, Action: ActionSet, Targets: []string{"device:a"}},   // set what?
		{Remote: "r", Button: "on", Event: Press, Action: ActionSet, Targets: []string{"device:a"}, Brightness: 101},
		{Remote: "r", Button: "on", Event: Press, Action: ActionSet, Targets: []string{"device:a"}, ColorTemp: 100},
	}
	for _, b := range bad {
		if _, err := NormalizeAll([]Binding{b}); err == nil {
			t.Errorf("%+v should be refused", b)
		}
	}
	if _, err := NormalizeAll([]Binding{ok[0], ok[0]}); err == nil {
		t.Error("two actions for the same press")
	}
}
