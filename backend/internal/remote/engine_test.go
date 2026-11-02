package remote

import (
	"io"
	"log/slog"
	"reflect"
	"slices"
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
	e       *Engine
	send    *fakeSender
	devs    *fakeDevices
	configs []Config
	presses []Pressed
	now     time.Time
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
	v.e = NewEngine(func() []Config { return v.configs }, groups, v.devs, v.send,
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

func a(button, event, action string) Action {
	return Action{Button: button, Event: event, Action: action}
}

func TestPressSwitchesTheRemotesTarget(t *testing.T) {
	v := newEngine(t)
	v.configs = []Config{{Remote: "styrbar", Target: "group:vardagsrum", Actions: []Action{a("on", Press, ActionOn), a("off", Press, ActionOff)}}}
	v.e.Handle("styrbar", "on")
	if got := v.payloads(); got != "Taklampa:state=ON Golvlampa:state=ON TV-uttag:state=ON" {
		t.Fatalf("on: %s", got)
	}
	v.e.Handle("styrbar", "off")
	if got := v.payloads(); got != "Taklampa:state=OFF Golvlampa:state=OFF TV-uttag:state=OFF" {
		t.Fatalf("off: %s", got)
	}
	v.configs[0].Target = "device:gamla"
	v.e.Handle("styrbar", "on")
	if got := v.payloads(); got != "Gammal:state_l1=ON" {
		t.Fatalf("one lamp: %s", got)
	}
	if len(v.presses) != 3 || v.presses[1] != (Pressed{Remote: "styrbar", Button: "off", Event: Press}) {
		t.Fatalf("presses %+v", v.presses)
	}
}

func TestToggleAGroupTogether(t *testing.T) {
	v := newEngine(t)
	v.configs = []Config{{Remote: "round", Target: "group:vardagsrum", Actions: []Action{a("toggle", Press, ActionToggle)}}}
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
	v.configs = []Config{{Remote: "styrbar", Target: "group:vardagsrum", Actions: []Action{a("on", Hold, ActionDimUp), a("off", Hold, ActionDimDown)}}}
	v.e.Handle("styrbar", "brightness_move_up")
	if got := v.payloads(); got != "Taklampa:brightness_move_onoff=64 Golvlampa:brightness_move_onoff=64" {
		t.Fatalf("dim up (outlets can't dim): %s", got)
	}
	v.e.Handle("styrbar", "brightness_stop")
	if got := v.payloads(); got != "Taklampa:brightness_move=stop Golvlampa:brightness_move=stop" {
		t.Fatalf("stop: %s", got)
	}
	v.e.Handle("styrbar", "brightness_move_down")
	if got := v.payloads(); got != "Taklampa:brightness_move=-64 Golvlampa:brightness_move=-64" {
		t.Fatalf("dim down: %s", got)
	}
	v.e.Handle("styrbar", "brightness_stop")
	v.payloads()
	v.e.Handle("styrbar", "brightness_stop") // nothing held any more
	if got := v.payloads(); got != "" {
		t.Fatalf("stray stop: %s", got)
	}
}

func TestRepeatedHoldMessagesActOnce(t *testing.T) {
	v := newEngine(t)
	v.configs = []Config{{Remote: "hue", Target: "device:tv", Actions: []Action{a("on", Hold, ActionToggle), a("on", Press, ActionOn)}}}
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
	if got := v.payloads(); got != "TV-uttag:state=ON" {
		t.Fatalf("short press: %s", got)
	}
}

func TestStepsAndSet(t *testing.T) {
	v := newEngine(t)
	v.configs = []Config{{Remote: "round", Target: "group:vardagsrum", Actions: []Action{
		a("brightness_up", Press, ActionBrighter),
		a("brightness_down", Press, ActionDarker),
		{Button: "arrow_left", Event: Press, Action: ActionSet, Brightness: 30, ColorTemp: 500},
	}}}
	v.e.Handle("round", "brightness_up_click")
	if got := v.payloads(); got != "Taklampa:brightness_step_onoff=51 Golvlampa:brightness_step_onoff=51" {
		t.Fatalf("brighter: %s", got)
	}
	v.e.Handle("round", "brightness_down_click")
	if got := v.payloads(); got != "Taklampa:brightness_step=-51 Golvlampa:brightness_step=-51" {
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
	v.configs = []Config{{Remote: "styrbar", Target: "group:borta", Actions: []Action{a("on", Press, ActionOn)}}}
	v.e.Handle("okänd", "on")                  // not a remote
	v.e.Handle("styrbar", "nonsens")           // not one of its actions
	v.e.Handle("styrbar", "arrow_right_click") // a button without an action: only reported
	v.e.Handle("styrbar", "on")                // its group is gone
	if got := v.payloads(); got != "" {
		t.Fatalf("%s", got)
	}
	if !reflect.DeepEqual(v.presses[0], Pressed{Remote: "styrbar", Button: "arrow_right", Event: Press}) {
		t.Fatalf("%+v", v.presses)
	}
}

func TestDefaults(t *testing.T) {
	str := func(l []Action) string {
		var out []string
		for _, x := range l {
			out = append(out, x.Button+"/"+x.Event+"="+x.Action)
		}
		return strings.Join(out, " ")
	}
	cases := []struct {
		name   string
		values []string
		want   string
	}{
		{"STYRBAR: on/off, dim while held, arrows free", styrbar, "on/press=on on/hold=dim_up off/press=off off/hold=dim_down"},
		{"RODRET", rodret, "on/press=on on/hold=dim_up off/press=off off/hold=dim_down"},
		{"TRÅDFRI round: up/down switch, middle toggles", tradfriRound,
			"brightness_down/press=off brightness_down/hold=dim_down brightness_up/press=on brightness_up/hold=dim_up toggle/press=toggle"},
		{"SOMRIG: 1 is up, 2 is down", somrig, "1/press=on 1/hold=dim_up 2/press=off 2/hold=dim_down"},
		{"Hue: on/off switch, up/down step and dim", hueDimmer,
			"on/press=on off/press=off up/press=brighter up/hold=dim_up down/press=darker down/hold=dim_down"},
		{"Aqara: one button toggles", aqara, "button/press=toggle"},
		{"unknown", unknown, ""},
	}
	for _, c := range cases {
		got := str(Defaults(Buttons(c.values)))
		// order within the list doesn't matter
		g, w := strings.Fields(got), strings.Fields(c.want)
		slices.Sort(g)
		slices.Sort(w)
		if !slices.Equal(g, w) {
			t.Errorf("%s:\n got %s\nwant %s", c.name, got, c.want)
		}
	}
}

func TestNormalizeConfigs(t *testing.T) {
	ok := []Config{
		{Remote: "r", Target: "group:g", Actions: []Action{a("on", Press, ActionToggle), a("on", Hold, ActionDimUp),
			{Button: "1", Event: Double, Action: ActionSet, Brightness: 50}, {Button: "2", Event: Press, Action: ActionSet, ColorTemp: 370}}},
		{Remote: "s", Target: "device:a", Actions: nil},
	}
	got, err := NormalizeConfigs(ok)
	if err != nil {
		t.Fatal(err)
	}
	if got[1].Actions == nil {
		t.Fatal("no actions: an empty list, not null")
	}
	bad := []Config{
		{Remote: "", Target: "device:a"},
		{Remote: "r", Target: ""},
		{Remote: "r", Target: "lamp:a"},
		{Remote: "r", Target: "device:a", Actions: []Action{a("", Press, ActionOn)}},
		{Remote: "r", Target: "device:a", Actions: []Action{a("on", Release, ActionOn)}},
		{Remote: "r", Target: "device:a", Actions: []Action{a("on", Press, "explode")}},
		{Remote: "r", Target: "device:a", Actions: []Action{a("on", Press, ActionDimUp)}}, // dimming needs hold
		{Remote: "r", Target: "device:a", Actions: []Action{a("on", Press, ActionSet)}},   // set what?
		{Remote: "r", Target: "device:a", Actions: []Action{{Button: "on", Event: Press, Action: ActionSet, Brightness: 101}}},
		{Remote: "r", Target: "device:a", Actions: []Action{a("on", Press, ActionOn), a("on", Press, ActionOff)}},
	}
	for _, b := range bad {
		if _, err := NormalizeConfigs([]Config{b}); err == nil {
			t.Errorf("%+v should be refused", b)
		}
	}
	if _, err := NormalizeConfigs([]Config{ok[1], ok[1]}); err == nil {
		t.Error("the same remote twice")
	}
}

func TestMigrateOldBindings(t *testing.T) {
	old := []Binding{
		{Remote: "r", Button: "on", Event: Press, Action: ActionToggle, Targets: []string{"group:g", "device:x"}},
		{Remote: "r", Button: "on", Event: Hold, Action: ActionDimUp, Targets: []string{"group:g"}},
		{Remote: "s", Button: "1", Event: Press, Action: ActionSet, Targets: []string{"device:a"}, Brightness: 40},
	}
	got := Migrate(old)
	want := []Config{
		{Remote: "r", Target: "group:g", Actions: []Action{a("on", Press, ActionToggle), a("on", Hold, ActionDimUp)}},
		{Remote: "s", Target: "device:a", Actions: []Action{{Button: "1", Event: Press, Action: ActionSet, Brightness: 40}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}
