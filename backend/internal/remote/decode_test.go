package remote

import (
	"reflect"
	"testing"
)

// Action lists as zigbee2mqtt reports them (definition.exposes "action" values).
var (
	styrbar = []string{"on", "off", "brightness_move_up", "brightness_move_down", "brightness_stop",
		"arrow_left_click", "arrow_right_click", "arrow_left_hold", "arrow_left_release", "arrow_right_hold", "arrow_right_release"}
	tradfriRound = []string{"arrow_left_click", "arrow_left_hold", "arrow_left_release", "arrow_right_click", "arrow_right_hold",
		"arrow_right_release", "brightness_down_click", "brightness_down_hold", "brightness_down_release", "brightness_up_click",
		"brightness_up_hold", "brightness_up_release", "toggle", "toggle_hold"}
	rodret = []string{"on", "off", "brightness_move_down", "brightness_move_up", "brightness_stop"}
	somrig = []string{"1_initial_press", "2_initial_press", "1_long_press", "2_long_press", "1_short_release", "2_short_release",
		"1_long_release", "2_long_release", "1_double_press", "2_double_press"}
	hueDimmer = []string{"on_press", "on_press_release", "on_hold", "on_hold_release", "up_press", "up_press_release", "up_hold",
		"up_hold_release", "down_press", "down_press_release", "down_hold", "down_hold_release", "off_press", "off_press_release",
		"off_hold", "off_hold_release"}
	hueOld  = []string{"on-press", "on-hold", "on-hold-release", "up-press", "up-hold", "up-hold-release", "down-press", "down-hold", "down-hold-release", "off-press", "off-hold", "off-hold-release"}
	aqara   = []string{"single", "double", "triple", "quadruple", "hold", "release"}
	aqara2  = []string{"button_1_single", "button_1_double", "button_1_hold", "button_2_single", "button_2_double", "button_2_hold", "button_1_release"}
	unknown = []string{"scene_1", "scene_2"}
)

func TestButtons(t *testing.T) {
	cases := []struct {
		name   string
		values []string
		want   []Button
	}{
		{"STYRBAR", styrbar, []Button{{"on", []string{Press, Hold}}, {"off", []string{Press, Hold}},
			{"arrow_left", []string{Press, Hold}}, {"arrow_right", []string{Press, Hold}}}},
		{"TRÅDFRI round", tradfriRound, []Button{{"arrow_left", []string{Press, Hold}}, {"arrow_right", []string{Press, Hold}},
			{"brightness_down", []string{Press, Hold}}, {"brightness_up", []string{Press, Hold}}, {"toggle", []string{Press, Hold}}}},
		{"RODRET", rodret, []Button{{"on", []string{Press, Hold}}, {"off", []string{Press, Hold}}}},
		{"SOMRIG", somrig, []Button{{"1", []string{Press, Double, Hold}}, {"2", []string{Press, Double, Hold}}}},
		{"Hue dimmer", hueDimmer, []Button{{"on", []string{Press, Hold}}, {"up", []string{Press, Hold}},
			{"down", []string{Press, Hold}}, {"off", []string{Press, Hold}}}},
		{"Hue old", hueOld, []Button{{"on", []string{Press, Hold}}, {"up", []string{Press, Hold}},
			{"down", []string{Press, Hold}}, {"off", []string{Press, Hold}}}},
		{"Aqara", aqara, []Button{{"button", []string{Press, Double, Hold}}}},
		{"Aqara two buttons", aqara2, []Button{{"button_1", []string{Press, Double, Hold}}, {"button_2", []string{Press, Double, Hold}}}},
		{"anything else", unknown, []Button{{"scene_1", []string{Press}}, {"scene_2", []string{Press}}}},
	}
	for _, c := range cases {
		if got := Buttons(c.values); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s:\n got %v\nwant %v", c.name, got, c.want)
		}
	}
}

func TestDecode(t *testing.T) {
	cases := []struct {
		values        []string
		action        string
		button, event string
		ok            bool
	}{
		{styrbar, "on", "on", Press, true},
		{styrbar, "brightness_move_up", "on", Hold, true},
		{styrbar, "brightness_move_down", "off", Hold, true},
		{styrbar, "brightness_stop", "", Release, true}, // which button: the one held
		{styrbar, "arrow_left_hold", "arrow_left", Hold, true},
		{styrbar, "arrow_left_release", "arrow_left", Release, true},
		{tradfriRound, "toggle", "toggle", Press, true},
		{tradfriRound, "toggle_hold", "toggle", Hold, true},
		{tradfriRound, "brightness_up_click", "brightness_up", Press, true},
		{somrig, "1_initial_press", "", "", false}, // the start of every press: wait for what it becomes
		{somrig, "1_short_release", "1", Press, true},
		{somrig, "2_long_press", "2", Hold, true},
		{somrig, "2_long_release", "2", Release, true},
		{somrig, "1_double_press", "1", Double, true},
		{hueDimmer, "on_press", "", "", false}, // followed by on_press_release or on_hold
		{hueDimmer, "on_press_release", "on", Press, true},
		{hueDimmer, "up_hold", "up", Hold, true},
		{hueDimmer, "up_hold_release", "up", Release, true},
		{hueOld, "on-press", "on", Press, true}, // no separate release: the press is the click
		{hueOld, "down-hold-release", "down", Release, true},
		{aqara, "single", "button", Press, true},
		{aqara, "release", "button", Release, true},
		{aqara, "triple", "", "", false},
		{aqara2, "button_2_double", "button_2", Double, true},
		{unknown, "scene_2", "scene_2", Press, true},
		{styrbar, "", "", "", false},
		{styrbar, "not_in_the_list", "", "", false},
	}
	for _, c := range cases {
		b, e, ok := Decode(c.values, c.action)
		if b != c.button || e != c.event || ok != c.ok {
			t.Errorf("%q: got %q %q %v, want %q %q %v", c.action, b, e, ok, c.button, c.event, c.ok)
		}
	}
}
