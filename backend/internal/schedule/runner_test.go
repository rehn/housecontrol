package schedule

import (
	"encoding/json"
	"io"
	"log/slog"
	"sort"
	"sync"
	"testing"
	"time"
)

type fakeSender struct {
	mu   sync.Mutex
	sent []string
}

func (f *fakeSender) Set(name string, payload map[string]any) error {
	b, _ := json.Marshal(payload)
	f.mu.Lock()
	f.sent = append(f.sent, name+" "+string(b))
	f.mu.Unlock()
	return nil
}

type fakeDevices map[string]Target

func (f fakeDevices) Target(id string) (Target, bool) { d, ok := f[id]; return d, ok }

func newRunner(list []Schedule, now *time.Time) (*Runner, *fakeSender) {
	send := &fakeSender{}
	devs := fakeDevices{
		"0x01": {Name: "ute/vägg", Brightness: true, BrightnessMax: 254},
		"0x02": {Name: "ute/uttag"},
	}
	r := NewRunner(func() ([]Schedule, Place) { return list, home }, send, devs, slog.New(slog.NewTextHandler(io.Discard, nil)))
	r.now = func() time.Time { return *now }
	return r, send
}

func TestRunnerSwitchesAtTheTime(t *testing.T) {
	list := []Schedule{{ID: "s", Name: "Ute", Enabled: true, DeviceIDs: []string{"0x01", "0x02", "0x99"},
		On: &Trigger{Kind: KindTime, Time: "07:00"}, Off: &Trigger{Kind: KindTime, Time: "08:00"}, Brightness: 40}}
	now := at(2026, 10, 29, 6, 59).Add(45 * time.Second)
	r, send := newRunner(list, &now)

	r.Tick() // first tick only remembers the time
	if len(send.sent) != 0 {
		t.Fatalf("%v", send.sent)
	}
	now = now.Add(15 * time.Second) // 07:00:00
	r.Tick()
	sort.Strings(send.sent)
	want := []string{`ute/uttag {"state":"ON"}`, `ute/vägg {"brightness":102,"state":"ON"}`} // 40 % of 254; 0x99 is gone
	if len(send.sent) != 2 || send.sent[0] != want[0] || send.sent[1] != want[1] {
		t.Fatalf("%v", send.sent)
	}
	now = now.Add(15 * time.Second)
	r.Tick()
	if len(send.sent) != 2 {
		t.Fatalf("must not repeat: %v", send.sent)
	}
	now = at(2026, 10, 29, 8, 0).Add(5 * time.Second)
	r.Tick()
	if len(send.sent) != 4 || send.sent[3] != `ute/vägg {"state":"OFF"}` && send.sent[2] != `ute/vägg {"state":"OFF"}` {
		t.Fatalf("%v", send.sent)
	}
}

func TestRunnerSkipsWhatWasMissedLongAgo(t *testing.T) {
	// the computer slept from 06:00 to 09:00: the 07:00 event is not run hours late
	list := []Schedule{{ID: "s", Name: "x", Enabled: true, DeviceIDs: []string{"0x02"}, On: &Trigger{Kind: KindTime, Time: "07:00"}}}
	now := at(2026, 10, 29, 6, 0)
	r, send := newRunner(list, &now)
	r.Tick()
	now = at(2026, 10, 29, 9, 0)
	r.Tick()
	if len(send.sent) != 0 {
		t.Fatalf("%v", send.sent)
	}
}
