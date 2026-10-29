// Package config loads housecontrol settings from an optional YAML file
// and environment variables. Environment variables win over the file.
//
// The MQTT settings here are only the *initial* defaults: once the broker is
// configured from the web UI, the saved settings (see package settings) win.
package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	MQTT    MQTT   `yaml:"mqtt"`
	HTTP    HTTP   `yaml:"http"`
	Auth    Auth   `yaml:"auth"`
	DataDir string `yaml:"dataDir"` // where UI-saved settings are stored
}

type MQTT struct {
	// URL of the broker, e.g. tcp://192.168.1.50:1883 or ssl://pi.lan:8883.
	// Empty = MQTT not configured yet (the app still starts).
	URL       string `yaml:"url" json:"url"`
	Username  string `yaml:"username" json:"username"`
	Password  string `yaml:"password" json:"password"`
	ClientID  string `yaml:"clientId" json:"clientId"`
	BaseTopic string `yaml:"baseTopic" json:"baseTopic"` // zigbee2mqtt base topic
}

// Auth is the admin login. No password = anyone can change things.
type Auth struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type HTTP struct {
	Addr string `yaml:"addr"`
	// TrustProxy: behind a reverse proxy / Kubernetes Gateway, take the client
	// address from X-Forwarded-For (env TRUST_PROXY=true).
	TrustProxy bool `yaml:"trustProxy"`
	// Origins allowed to call the API / open WebSockets from another origin
	// (e.g. http://localhost:3000 during development). Empty = same origin only.
	CORSOrigins []string `yaml:"corsOrigins"`
}

const DefaultBaseTopic = "zigbee2mqtt"

func defaults() Config {
	return Config{
		MQTT:    MQTT{BaseTopic: DefaultBaseTopic},
		HTTP:    HTTP{Addr: ":8080"},
		Auth:    Auth{Username: "admin"},
		DataDir: "data",
	}
}

// Load reads the YAML file at path (if it exists; "" skips the file), then
// applies environment overrides. A missing or invalid MQTT URL is not an
// error: the broker can be configured later from the UI.
func Load(path string) (Config, error) {
	cfg := defaults()

	if path != "" {
		b, err := os.ReadFile(path)
		switch {
		case err == nil:
			if err := yaml.Unmarshal(b, &cfg); err != nil {
				return cfg, fmt.Errorf("parse %s: %w", path, err)
			}
		case errors.Is(err, os.ErrNotExist):
			// optional file
		default:
			return cfg, fmt.Errorf("read %s: %w", path, err)
		}
	}

	env(&cfg.MQTT.URL, "MQTT_URL")
	env(&cfg.MQTT.Username, "MQTT_USERNAME")
	env(&cfg.MQTT.Password, "MQTT_PASSWORD")
	env(&cfg.MQTT.ClientID, "MQTT_CLIENT_ID")
	env(&cfg.MQTT.BaseTopic, "Z2M_BASE_TOPIC")
	env(&cfg.HTTP.Addr, "HTTP_ADDR")
	env(&cfg.DataDir, "DATA_DIR")
	env(&cfg.Auth.Username, "ADMIN_USERNAME")
	env(&cfg.Auth.Password, "ADMIN_PASSWORD")
	if strings.TrimSpace(cfg.Auth.Username) == "" {
		cfg.Auth.Username = "admin"
	}
	if v, ok := os.LookupEnv("TRUST_PROXY"); ok {
		cfg.HTTP.TrustProxy = v == "true" || v == "1" || v == "yes"
	}
	if v, ok := os.LookupEnv("CORS_ORIGINS"); ok {
		cfg.HTTP.CORSOrigins = splitList(v)
	}
	if cfg.DataDir == "" {
		cfg.DataDir = "data"
	}
	return cfg, nil
}

// Normalize cleans up MQTT settings: trims the base topic and fills in a
// scheme/port on the broker URL. It returns an error for unusable URLs.
func (m MQTT) Normalize() (MQTT, error) {
	m.BaseTopic = strings.Trim(strings.TrimSpace(m.BaseTopic), "/")
	if m.BaseTopic == "" {
		m.BaseTopic = DefaultBaseTopic
	}
	m.ClientID = strings.TrimSpace(m.ClientID)
	m.Username = strings.TrimSpace(m.Username)
	if strings.TrimSpace(m.URL) == "" {
		m.URL = ""
		return m, nil
	}
	u, err := NormalizeBrokerURL(m.URL)
	if err != nil {
		return m, err
	}
	m.URL = u
	return m, nil
}

var schemes = map[string]string{
	"tcp": "1883", "mqtt": "1883", "ws": "80",
	"ssl": "8883", "tls": "8883", "mqtts": "8883", "wss": "443",
}

// NormalizeBrokerURL accepts "192.168.1.201", "pi.lan:1883" or a full URL
// and returns e.g. "tcp://192.168.1.201:1883".
func NormalizeBrokerURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if !strings.Contains(raw, "://") {
		raw = "tcp://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid broker URL: %w", err)
	}
	u.Scheme = strings.ToLower(u.Scheme)
	defPort, ok := schemes[u.Scheme]
	if !ok {
		return "", fmt.Errorf("unsupported scheme %q (use tcp, ssl, ws or wss)", u.Scheme)
	}
	if u.Hostname() == "" {
		return "", errors.New("broker URL has no host")
	}
	if u.Port() == "" {
		u.Host = net.JoinHostPort(u.Hostname(), defPort)
	}
	return u.String(), nil
}

func env(dst *string, key string) {
	if v, ok := os.LookupEnv(key); ok {
		*dst = v
	}
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
