package dashboard

import (
	"encoding/json"
	"strings"
	"testing"
)

func parse(t *testing.T, s string) Dashboard {
	t.Helper()
	var d Dashboard
	if err := json.Unmarshal([]byte(s), &d); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestValidDashboard(t *testing.T) {
	d := parse(t, `{"version":1,"columns":[
		{"id":"c1","cards":[{"id":"a","type":"clock"},{"id":"b","type":"device","deviceId":"0x01"}]},
		{"id":"c2","cards":[{"id":"g","type":"group","title":" Köket ","deviceIds":["0x01","0x02","0x01"]}]}
	]}`)
	n, err := d.Normalize()
	if err != nil {
		t.Fatal(err)
	}
	g := n.Columns[1].Cards[0]
	if g.Title != "Köket" {
		t.Errorf("title not trimmed: %q", g.Title)
	}
	if len(g.DeviceIDs) != 2 {
		t.Errorf("duplicate device ids not removed: %v", g.DeviceIDs)
	}
}

func TestInvalidDashboards(t *testing.T) {
	cases := map[string]string{
		"no columns":        `{"version":1,"columns":[]}`,
		"too many columns":  `{"version":1,"columns":[{"id":"1"},{"id":"2"},{"id":"3"},{"id":"4"},{"id":"5"},{"id":"6"},{"id":"7"}]}`,
		"unknown card type": `{"version":1,"columns":[{"id":"c","cards":[{"id":"a","type":"camera"}]}]}`,
		"device without id": `{"version":1,"columns":[{"id":"c","cards":[{"id":"a","type":"device"}]}]}`,
		"group no devices":  `{"version":1,"columns":[{"id":"c","cards":[{"id":"a","type":"group","title":"x","deviceIds":[]}]}]}`,
		"group no title":    `{"version":1,"columns":[{"id":"c","cards":[{"id":"a","type":"group","title":"  ","deviceIds":["0x1"]}]}]}`,
		"missing card id":   `{"version":1,"columns":[{"id":"c","cards":[{"type":"clock"}]}]}`,
		"duplicate card id": `{"version":1,"columns":[{"id":"c","cards":[{"id":"a","type":"clock"},{"id":"a","type":"clock"}]}]}`,
		"missing column id": `{"version":1,"columns":[{"cards":[]}]}`,
		"title too long":    `{"version":1,"columns":[{"id":"c","cards":[{"id":"a","type":"device","deviceId":"0x1","title":"` + strings.Repeat("x", 61) + `"}]}]}`,
	}
	for name, s := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parse(t, s).Normalize(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestNormalizeSetsVersionAndEmptyCards(t *testing.T) {
	n, err := parse(t, `{"columns":[{"id":"c"}]}`).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if n.Version != CurrentVersion {
		t.Errorf("version = %d", n.Version)
	}
	b, _ := json.Marshal(n)
	if !strings.Contains(string(b), `"cards":[]`) {
		t.Errorf("cards should serialize as [] not null: %s", b)
	}
}

func TestWeatherCard(t *testing.T) {
	n, err := parse(t, `{"version":1,"columns":[{"id":"c","cards":[{"id":"w","type":"weather","deviceId":"x"}]}]}`).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if c := n.Columns[0].Cards[0]; c.Type != "weather" || c.DeviceID != "" {
		t.Fatalf("%+v", c)
	}
}

func TestCalendarCard(t *testing.T) {
	if _, err := parse(t, `{"version":1,"columns":[{"id":"c","cards":[{"id":"k","type":"calendar"}]}]}`).Normalize(); err != nil {
		t.Fatal(err)
	}
}
