// Package devices keeps the in-memory view of zigbee2mqtt devices and their
// latest state, and fans out changes to subscribers.
package devices

import (
	"sort"
	"sync"
	"time"

	"housecontrol/internal/dashboard"
	"housecontrol/internal/settings"
)

type Kind string

const (
	KindLight  Kind = "light"
	KindSwitch Kind = "switch" // smart plugs / outlets / relays
	KindOther  Kind = "other"
)

// Capabilities describes what the UI can control on a device.
type Capabilities struct {
	OnOff         bool   `json:"onOff"`
	StateProperty string `json:"stateProperty,omitempty"` // usually "state"
	Brightness    bool   `json:"brightness"`
	BrightnessMax int    `json:"brightnessMax,omitempty"`
	ColorTemp     bool   `json:"colorTemp"`
	ColorTempMin  int    `json:"colorTempMin,omitempty"`
	ColorTempMax  int    `json:"colorTempMax,omitempty"`
	Color         bool   `json:"color"`
	Power         bool   `json:"power"` // reports power (W)
}

type Device struct {
	ID           string         `json:"id"` // IEEE address, stable
	Name         string         `json:"name"`
	Kind         Kind           `json:"kind"`
	Type         string         `json:"type"` // Router / EndDevice
	Vendor       string         `json:"vendor,omitempty"`
	Model        string         `json:"model,omitempty"`
	Description  string         `json:"description,omitempty"`
	Capabilities Capabilities   `json:"capabilities"`
	Availability string         `json:"availability"` // online / offline / unknown
	State        map[string]any `json:"state"`
}

// MQTTStatus describes the broker connection, shown in the UI.
type MQTTStatus struct {
	// "unconfigured" | "connecting" | "connected" | "error"
	State  string    `json:"state"`
	Broker string    `json:"broker,omitempty"`
	Error  string    `json:"error,omitempty"`
	Since  time.Time `json:"since"`
}

// Event is pushed to subscribers (and on to WebSocket clients).
type Event struct {
	Type    string         `json:"type"` // "devices" | "state" | "availability" | "status"
	Devices []Device       `json:"devices,omitempty"`
	ID      string         `json:"id,omitempty"`
	State   map[string]any `json:"state,omitempty"`
	Avail   string         `json:"availability,omitempty"`
	MQTT    *MQTTStatus    `json:"mqtt,omitempty"`
	// Dashboard is set on "dashboard" events (nil = none saved yet).
	Dashboard *dashboard.Dashboard `json:"dashboard,omitempty"`
	// UI is set on "ui" events (language etc.).
	UI *settings.UI `json:"ui,omitempty"`
}

type Registry struct {
	mu      sync.RWMutex
	devices map[string]*Device        // by ID
	byName  map[string]string         // friendly name -> ID
	states  map[string]map[string]any // by friendly name (may arrive before device list)
	avail   map[string]string         // by friendly name

	subMu sync.Mutex
	subs  map[chan Event]struct{}
}

func NewRegistry() *Registry {
	return &Registry{
		devices: map[string]*Device{},
		byName:  map[string]string{},
		states:  map[string]map[string]any{},
		avail:   map[string]string{},
		subs:    map[chan Event]struct{}{},
	}
}

// Reset forgets all devices and state (used when switching broker).
func (r *Registry) Reset() {
	r.mu.Lock()
	r.devices = map[string]*Device{}
	r.byName = map[string]string{}
	r.states = map[string]map[string]any{}
	r.avail = map[string]string{}
	r.mu.Unlock()
	r.publish(Event{Type: "devices", Devices: []Device{}})
}

// Broadcast sends an arbitrary event to all subscribers.
func (r *Registry) Broadcast(e Event) { r.publish(e) }

// SetDevices replaces the device list (from zigbee2mqtt/bridge/devices).
func (r *Registry) SetDevices(list []Device) {
	r.mu.Lock()
	r.devices = make(map[string]*Device, len(list))
	r.byName = make(map[string]string, len(list))
	for i := range list {
		d := list[i]
		r.devices[d.ID] = &d
		r.byName[d.Name] = d.ID
	}
	r.mu.Unlock()
	r.publish(Event{Type: "devices", Devices: r.List()})
}

// MergeState merges a state payload published on zigbee2mqtt/<name>.
func (r *Registry) MergeState(name string, payload map[string]any) {
	r.mu.Lock()
	st := r.states[name]
	if st == nil {
		st = map[string]any{}
		r.states[name] = st
	}
	for k, v := range payload {
		st[k] = v
	}
	id := r.byName[name]
	snapshot := copyMap(st)
	r.mu.Unlock()
	if id != "" {
		r.publish(Event{Type: "state", ID: id, State: snapshot})
	}
}

func (r *Registry) SetAvailability(name, value string) {
	r.mu.Lock()
	r.avail[name] = value
	id := r.byName[name]
	r.mu.Unlock()
	if id != "" {
		r.publish(Event{Type: "availability", ID: id, Avail: value})
	}
}

// IsDevice reports whether name is a known friendly name.
func (r *Registry) IsDevice(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.byName[name]
	return ok
}

func (r *Registry) Get(id string) (Device, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.devices[id]
	if !ok {
		return Device{}, false
	}
	return r.view(d), true
}

// List returns all devices sorted by name, with current state.
func (r *Registry) List() []Device {
	r.mu.RLock()
	out := make([]Device, 0, len(r.devices))
	for _, d := range r.devices {
		out = append(out, r.view(d))
	}
	r.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// view must be called with r.mu held.
func (r *Registry) view(d *Device) Device {
	v := *d
	v.State = copyMap(r.states[d.Name])
	v.Availability = r.avail[d.Name]
	if v.Availability == "" {
		v.Availability = "unknown"
	}
	return v
}

// Subscribe returns a channel of events and a cancel func. Slow subscribers
// drop events rather than block the MQTT handler.
func (r *Registry) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 64)
	r.subMu.Lock()
	r.subs[ch] = struct{}{}
	r.subMu.Unlock()
	return ch, func() {
		r.subMu.Lock()
		if _, ok := r.subs[ch]; ok {
			delete(r.subs, ch)
			close(ch)
		}
		r.subMu.Unlock()
	}
}

func (r *Registry) publish(e Event) {
	r.subMu.Lock()
	defer r.subMu.Unlock()
	for ch := range r.subs {
		select {
		case ch <- e:
		default:
		}
	}
}

func copyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
