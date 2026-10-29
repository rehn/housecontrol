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
