package dashboard

import (
	"encoding/json"
	"fmt"
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
	g := n.Tabs[0].Columns[1].Cards[0]
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
	if c := n.Tabs[0].Columns[0].Cards[0]; c.Type != "weather" || c.DeviceID != "" {
		t.Fatalf("%+v", c)
	}
}

func TestCalendarCard(t *testing.T) {
	if _, err := parse(t, `{"version":1,"columns":[{"id":"c","cards":[{"id":"k","type":"calendar"}]}]}`).Normalize(); err != nil {
		t.Fatal(err)
	}
}

// --- tabs -------------------------------------------------------------------------

func TestOldLayoutBecomesTheFirstTab(t *testing.T) {
	old := parse(t, `{"version":1,"columns":[{"id":"c1","cards":[{"id":"a","type":"clock"}]}]}`)
	for name, d := range map[string]Dashboard{"upgrade": old.Upgrade()} {
		if d.Version != CurrentVersion || len(d.Tabs) != 1 || d.Columns != nil {
			t.Fatalf("%s: %+v", name, d)
		}
		tab := d.Tabs[0]
		if tab.ID != "home" || tab.Kind != KindCards || tab.Columns[0].Cards[0].Type != "clock" {
			t.Fatalf("%s: %+v", name, tab)
		}
	}
	n, err := old.Normalize()
	if err != nil || len(n.Tabs) != 1 || n.Tabs[0].Columns[0].Cards[0].ID != "a" {
		t.Fatalf("%v %+v", err, n)
	}
	b, _ := json.Marshal(n)
	if !strings.HasPrefix(string(b), `{"version":2,"tabs":[`) {
		t.Fatalf("saved form must use tabs only: %s", b)
	}
}

func TestTabs(t *testing.T) {
	d := parse(t, `{"version":2,"tabs":[
		{"id":"home","title":" Hem ","kind":"cards","columns":[{"id":"c","cards":[{"id":"a","type":"clock"}]}]},
		{"id":"el","title":"El","kind":"grafana","url":" https://grafana.hemma.lan/d/abc/el?orgId=1 ","columns":[{"id":"x"}]}
	]}`)
	n, err := d.Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if n.Tabs[0].Title != "Hem" || n.Tabs[1].URL != "https://grafana.hemma.lan/d/abc/el?orgId=1" {
		t.Fatalf("%+v", n.Tabs)
	}
	if n.Tabs[1].Columns != nil {
		t.Fatal("a Grafana tab has no cards")
	}
}

func TestInvalidTabs(t *testing.T) {
	many := make([]string, 0, MaxTabs+1)
	for i := 0; i <= MaxTabs; i++ {
		many = append(many, fmt.Sprintf(`{"id":"t%d","kind":"cards","columns":[{"id":"c%d"}]}`, i, i))
	}
	cases := map[string]string{
		"no tabs":              `{"version":2,"tabs":[]}`,
		"too many tabs":        `{"version":2,"tabs":[` + strings.Join(many, ",") + `]}`,
		"tab without id":       `{"version":2,"tabs":[{"kind":"cards","columns":[{"id":"c"}]}]}`,
		"duplicate tab id":     `{"version":2,"tabs":[{"id":"a","kind":"cards","columns":[{"id":"c"}]},{"id":"a","kind":"cards","columns":[{"id":"d"}]}]}`,
		"unknown kind":         `{"version":2,"tabs":[{"id":"a","kind":"video"}]}`,
		"cards tab no columns": `{"version":2,"tabs":[{"id":"a","kind":"cards","columns":[]}]}`,
		"grafana no url":       `{"version":2,"tabs":[{"id":"a","kind":"grafana"}]}`,
		"grafana bad scheme":   `{"version":2,"tabs":[{"id":"a","kind":"grafana","url":"javascript:alert(1)"}]}`,
		"grafana no host":      `{"version":2,"tabs":[{"id":"a","kind":"grafana","url":"https:///d/x"}]}`,
		"tab title too long":   `{"version":2,"tabs":[{"id":"a","title":"` + strings.Repeat("x", MaxTabTitleLen+1) + `","kind":"cards","columns":[{"id":"c"}]}]}`,
		"card id in two tabs": `{"version":2,"tabs":[{"id":"a","kind":"cards","columns":[{"id":"c","cards":[{"id":"x","type":"clock"}]}]},
			{"id":"b","kind":"cards","columns":[{"id":"d","cards":[{"id":"x","type":"clock"}]}]}]}`,
	}
	for name, s := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parse(t, s).Normalize(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestGrafanaCard(t *testing.T) {
	card := func(extra string) string {
		return `{"version":2,"tabs":[{"id":"a","kind":"cards","columns":[{"id":"c","cards":[{"id":"g","type":"grafana"` + extra + `}]}]}]}`
	}
	n, err := parse(t, card(`,"title":"Temperatur","url":"https://grafana.hemma.lan/d-solo/abc/x?panelId=2"`)).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if c := n.Tabs[0].Columns[0].Cards[0]; c.URL == "" || c.Height != DefaultGrafanaHeight {
		t.Fatalf("%+v", c)
	}
	n, _ = parse(t, card(`,"url":"https://g/x","height":5000`)).Normalize()
	if h := n.Tabs[0].Columns[0].Cards[0].Height; h != MaxGrafanaHeight {
		t.Fatalf("height capped: %d", h)
	}
	n, _ = parse(t, card(`,"url":"https://g/x","height":10`)).Normalize()
	if h := n.Tabs[0].Columns[0].Cards[0].Height; h != MinGrafanaHeight {
		t.Fatalf("height raised: %d", h)
	}
	for name, extra := range map[string]string{"no url": ``, "bad url": `,"url":"file:///etc/passwd"`} {
		if _, err := parse(t, card(extra)).Normalize(); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	// other cards don't keep a url
	n, _ = parse(t, `{"version":2,"tabs":[{"id":"a","kind":"cards","columns":[{"id":"c","cards":[{"id":"k","type":"clock","url":"https://x","height":300}]}]}]}`).Normalize()
	if c := n.Tabs[0].Columns[0].Cards[0]; c.URL != "" || c.Height != 0 {
		t.Fatalf("%+v", c)
	}
}
