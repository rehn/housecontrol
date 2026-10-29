package devices

import "encoding/json"

// z2mDevice mirrors the parts of zigbee2mqtt/bridge/devices we use.
type z2mDevice struct {
	IEEEAddress  string `json:"ieee_address"`
	FriendlyName string `json:"friendly_name"`
	Type         string `json:"type"`
	Supported    bool   `json:"supported"`
	Disabled     bool   `json:"disabled"`
	Definition   *struct {
		Model       string   `json:"model"`
		Vendor      string   `json:"vendor"`
		Description string   `json:"description"`
		Exposes     []expose `json:"exposes"`
	} `json:"definition"`
}

type expose struct {
	Type     string   `json:"type"`
	Name     string   `json:"name"`
	Property string   `json:"property"`
	Endpoint string   `json:"endpoint"`
	ValueMin *float64 `json:"value_min"`
	ValueMax *float64 `json:"value_max"`
	Features []expose `json:"features"`
}

// ParseBridgeDevices converts the bridge/devices payload into Devices.
// The coordinator and disabled devices are skipped.
func ParseBridgeDevices(payload []byte) ([]Device, error) {
	var raw []z2mDevice
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, err
	}
	out := make([]Device, 0, len(raw))
	for _, z := range raw {
		if z.Type == "Coordinator" || z.Disabled {
			continue
		}
		d := Device{ID: z.IEEEAddress, Name: z.FriendlyName, Type: z.Type, Kind: KindOther}
		if z.Definition != nil {
			d.Vendor = z.Definition.Vendor
			d.Model = z.Definition.Model
			d.Description = z.Definition.Description
			d.Kind, d.Capabilities = capabilities(z.Definition.Exposes)
		}
		out = append(out, d)
	}
	return out, nil
}

func capabilities(exposes []expose) (Kind, Capabilities) {
	kind := KindOther
	var c Capabilities
	for _, e := range exposes {
		switch e.Type {
		case "light", "switch":
			// Only use the first un-suffixed endpoint; multi-gang devices
			// can be added later.
			if kind != KindOther || e.Endpoint != "" {
				continue
			}
			if e.Type == "light" {
				kind = KindLight
			} else {
				kind = KindSwitch
			}
			for _, f := range e.Features {
				switch f.Name {
				case "state":
					c.OnOff = true
					c.StateProperty = f.Property
				case "brightness":
					c.Brightness = true
					c.BrightnessMax = intOr(f.ValueMax, 254)
				case "color_temp":
					c.ColorTemp = true
					c.ColorTempMin = intOr(f.ValueMin, 150)
					c.ColorTempMax = intOr(f.ValueMax, 500)
				case "color_xy", "color_hs":
					c.Color = true
				}
			}
		case "numeric":
			if e.Name == "power" {
				c.Power = true
			}
		}
	}
	if c.StateProperty == "" && c.OnOff {
		c.StateProperty = "state"
	}
	return kind, c
}

func intOr(p *float64, def int) int {
	if p == nil {
		return def
	}
	return int(*p)
}
