package devices

import "testing"

const sample = `[
 {"ieee_address":"0x00","friendly_name":"Coordinator","type":"Coordinator"},
 {"ieee_address":"0x01","friendly_name":"living/ceiling","type":"Router","supported":true,
  "definition":{"model":"LED1545G12","vendor":"IKEA","description":"bulb","exposes":[
   {"type":"light","features":[
     {"type":"binary","name":"state","property":"state"},
     {"type":"numeric","name":"brightness","property":"brightness","value_min":0,"value_max":254},
     {"type":"numeric","name":"color_temp","property":"color_temp","value_min":250,"value_max":454}]},
   {"type":"numeric","name":"linkquality","property":"linkquality"}]}},
 {"ieee_address":"0x02","friendly_name":"tv plug","type":"Router","supported":true,
  "definition":{"model":"E1603","vendor":"IKEA","description":"plug","exposes":[
   {"type":"switch","features":[{"type":"binary","name":"state","property":"state"}]},
   {"type":"numeric","name":"power","property":"power"}]}}
]`

func TestParseBridgeDevices(t *testing.T) {
	ds, err := ParseBridgeDevices([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 2 {
		t.Fatalf("want 2 devices (coordinator skipped), got %d", len(ds))
	}
	l := ds[0]
	if l.Kind != KindLight || !l.Capabilities.Brightness || !l.Capabilities.ColorTemp ||
		l.Capabilities.ColorTempMin != 250 || l.Capabilities.ColorTempMax != 454 {
		t.Fatalf("bad light: %+v", l)
	}
	p := ds[1]
	if p.Kind != KindSwitch || !p.Capabilities.OnOff || !p.Capabilities.Power || p.Capabilities.Brightness {
		t.Fatalf("bad plug: %+v", p)
	}
}

func TestRegistryStateBeforeDevices(t *testing.T) {
	r := NewRegistry()
	r.MergeState("tv plug", map[string]any{"state": "ON"})
	ds, _ := ParseBridgeDevices([]byte(sample))
	r.SetDevices(ds)
	d, ok := r.Get("0x02")
	if !ok || d.State["state"] != "ON" {
		t.Fatalf("state not merged: %+v", d)
	}
}

const remoteSample = `[
 {"ieee_address":"0x10","friendly_name":"Fjärr vardagsrum","type":"EndDevice","supported":true,
  "definition":{"model":"E2001/E2002","vendor":"IKEA","description":"STYRBAR remote control","exposes":[
   {"type":"numeric","name":"battery","property":"battery"},
   {"type":"enum","name":"action","property":"action","values":["on","off","brightness_move_up","brightness_stop"]},
   {"type":"numeric","name":"linkquality","property":"linkquality"}]}}
]`

func TestRemotesAreRecognised(t *testing.T) {
	ds, err := ParseBridgeDevices([]byte(remoteSample))
	if err != nil || len(ds) != 1 {
		t.Fatal(err, ds)
	}
	r := ds[0]
	if r.Kind != KindRemote || len(r.Capabilities.Actions) != 4 || r.Capabilities.Actions[2] != "brightness_move_up" || r.Capabilities.OnOff {
		t.Fatalf("%+v", r)
	}
}

func TestActionsAreEventsNotState(t *testing.T) {
	r := NewRegistry()
	ds, _ := ParseBridgeDevices([]byte(remoteSample))
	r.SetDevices(ds)
	events, cancel := r.Subscribe()
	defer cancel()

	r.MergeState("Fjärr vardagsrum", map[string]any{"action": "on", "battery": 87.0})
	var got []Event
	for len(got) < 2 {
		got = append(got, <-events)
	}
	if got[0].Type != "state" || got[0].State["battery"] != 87.0 || got[0].State["action"] != nil {
		t.Fatalf("state %+v", got[0])
	}
	if got[1].Type != "action" || got[1].ID != "0x10" || got[1].Action != "on" {
		t.Fatalf("action %+v", got[1])
	}
	// zigbee2mqtt clears the action right after: no event for that
	r.MergeState("Fjärr vardagsrum", map[string]any{"action": ""})
	select {
	case e := <-events:
		if e.Type == "action" {
			t.Fatalf("empty action: %+v", e)
		}
	default:
	}
	if d, _ := r.Get("0x10"); d.State["action"] != nil {
		t.Fatalf("kept in state: %+v", d.State)
	}
}
