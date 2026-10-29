package config

import (
	"os"
	"testing"
)

func TestNormalizeBrokerURL(t *testing.T) {
	cases := map[string]string{
		"192.168.1.201":            "tcp://192.168.1.201:1883",
		"192.168.1.201:1884":       "tcp://192.168.1.201:1884",
		"tcp://192.168.1.201:1883": "tcp://192.168.1.201:1883",
		"ssl://pi.lan":             "ssl://pi.lan:8883",
		" MQTT://pi.lan ":          "mqtt://pi.lan:1883",
		"ws://pi.lan:9001/mqtt":    "ws://pi.lan:9001/mqtt",
	}
	for in, want := range cases {
		got, err := NormalizeBrokerURL(in)
		if err != nil || got != want {
			t.Errorf("%q: got %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"http://pi.lan", "tcp://", "tcp://:1883"} {
		if _, err := NormalizeBrokerURL(bad); err == nil {
			t.Errorf("%q: expected error", bad)
		}
	}
}

func TestLoadWithoutMQTT(t *testing.T) {
	t.Setenv("MQTT_URL", "")
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("app must start without MQTT config: %v", err)
	}
	m, err := cfg.MQTT.Normalize()
	if err != nil || m.URL != "" || m.BaseTopic != "zigbee2mqtt" {
		t.Fatalf("got %+v, %v", m, err)
	}
}

func TestLoadAuth(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/config.yaml"
	if err := os.WriteFile(path, []byte("auth:\n  username: lars\n  password: frånfil\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ADMIN_USERNAME", "")
	os.Unsetenv("ADMIN_USERNAME")
	os.Unsetenv("ADMIN_PASSWORD")
	cfg, err := Load(path)
	if err != nil || cfg.Auth.Username != "lars" || cfg.Auth.Password != "frånfil" {
		t.Fatalf("%v %+v", err, cfg.Auth)
	}

	t.Setenv("ADMIN_PASSWORD", "frånenv")
	cfg, _ = Load(path)
	if cfg.Auth.Password != "frånenv" || cfg.Auth.Username != "lars" {
		t.Fatalf("env wins: %+v", cfg.Auth)
	}

	cfg, _ = Load("")
	if cfg.Auth.Username != "admin" {
		t.Fatalf("default user name: %+v", cfg.Auth)
	}
}

func TestTrustProxyFromEnv(t *testing.T) {
	t.Setenv("TRUST_PROXY", "true")
	cfg, _ := Load("")
	if !cfg.HTTP.TrustProxy {
		t.Fatal("TRUST_PROXY=true")
	}
	t.Setenv("TRUST_PROXY", "false")
	if cfg, _ := Load(""); cfg.HTTP.TrustProxy {
		t.Fatal("TRUST_PROXY=false")
	}
}
