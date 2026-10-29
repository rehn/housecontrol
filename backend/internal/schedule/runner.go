package schedule

import (
	"context"
	"log/slog"
	"math"
	"time"
)

// TickEvery is how often the runner looks for events (so they fire within ~15 s).
const TickEvery = 15 * time.Second

// maxCatchUp: events missed by more than this (computer asleep, app down)
// are skipped rather than run late.
const maxCatchUp = 2 * time.Minute

// Sender switches a device by its zigbee2mqtt name (mqttclient.Manager).
type Sender interface {
	Set(name string, payload map[string]any) error
}

// Target is what the runner needs to know about a device.
type Target struct {
	Name          string // zigbee2mqtt friendly name
	StateProperty string // usually "state"
	Brightness    bool
	BrightnessMax int
}

// Devices looks devices up by id (an adapter around devices.Registry, kept
// small so this package doesn't depend on the registry).
type Devices interface {
	Target(id string) (Target, bool)
}

// Runner carries out the schedules.
type Runner struct {
	load func() ([]Schedule, Place) // read on every tick, so changes apply at once
	send Sender
	devs Devices
	log  *slog.Logger
	now  func() time.Time
	last time.Time
}

func NewRunner(load func() ([]Schedule, Place), send Sender, devs Devices, log *slog.Logger) *Runner {
	return &Runner{load: load, send: send, devs: devs, log: log, now: time.Now}
}

// Run ticks until ctx ends.
func (r *Runner) Run(ctx context.Context) {
	r.Tick()
	t := time.NewTicker(TickEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.Tick()
		}
	}
}

// Tick runs every event since the previous tick.
func (r *Runner) Tick() {
	now := r.now()
	from := r.last
	r.last = now
	if from.IsZero() {
		return // first tick: nothing is due "since" before we started
	}
	if now.Sub(from) > maxCatchUp {
		from = now.Add(-maxCatchUp)
	}
	list, place := r.load()
	byID := make(map[string]Schedule, len(list))
	for _, s := range list {
		byID[s.ID] = s
	}
	for _, ev := range Due(list, from, now, place) {
		r.run(byID[ev.ScheduleID], ev)
	}
}

func (r *Runner) run(s Schedule, ev Event) {
	r.log.Info("schedule", "name", s.Name, "action", ev.Action, "at", ev.At.Format("15:04"))
	for _, id := range s.DeviceIDs {
		d, ok := r.devs.Target(id)
		if !ok {
			r.log.Warn("schedule: device not found", "schedule", s.Name, "id", id)
			continue
		}
		if err := r.send.Set(d.Name, payload(d, s, ev.Action)); err != nil {
			r.log.Warn("schedule: could not switch", "schedule", s.Name, "device", d.Name, "err", err)
		}
	}
}

func payload(d Target, s Schedule, action string) map[string]any {
	prop := d.StateProperty
	if prop == "" {
		prop = "state"
	}
	if action == ActionOff {
		return map[string]any{prop: "OFF"}
	}
	p := map[string]any{prop: "ON"}
	if s.Brightness > 0 && d.Brightness {
		max := d.BrightnessMax
		if max <= 0 {
			max = 254
		}
		p["brightness"] = int(math.Max(1, math.Round(float64(s.Brightness)*float64(max)/100)))
	}
	return p
}
