package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // time zones also in the minimal Docker image (TZ=Europe/Stockholm)

	"housecontrol/internal/api"
	"housecontrol/internal/auth"
	"housecontrol/internal/autobackup"
	"housecontrol/internal/calendar"
	"housecontrol/internal/config"
	"housecontrol/internal/devices"
	"housecontrol/internal/mqttclient"
	"housecontrol/internal/remote"
	"housecontrol/internal/schedule"
	"housecontrol/internal/settings"
	"housecontrol/internal/weather"
)

// version is set at build time (-ldflags "-X main.version=1.2.3").
var version = "dev"

func main() {
	cfgPath := flag.String("config", envOr("CONFIG_FILE", "config.yaml"), "path to optional YAML config")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel()}))

	log.Info("housecontrol starting", "version", version)

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}

	// MQTT settings: saved from the UI > config file / env. A broken or
	// missing broker config never stops the app; it is fixed from the UI.
	store := settings.NewStore(cfg.DataDir)
	mqttCfg, source := cfg.MQTT, "config"
	if saved, err := store.LoadMQTT(); err != nil {
		log.Error("ignoring unreadable settings file", "path", store.Path(), "err", err)
	} else if saved != nil {
		mqttCfg, source = *saved, "ui"
	}
	if n, err := mqttCfg.Normalize(); err != nil {
		log.Error("invalid mqtt settings, starting without mqtt", "source", source, "err", err)
		mqttCfg = config.MQTT{BaseTopic: config.DefaultBaseTopic}
	} else {
		mqttCfg = n
	}

	reg := devices.NewRegistry()
	mq := mqttclient.NewManager(reg, log)
	mq.Apply(mqttCfg, source)
	defer mq.Close()

	// Admin login: viewing is open, changing things needs ADMIN_PASSWORD.
	var login *auth.Auth
	if cfg.Auth.Password == "" {
		log.Warn("no ADMIN_PASSWORD set: anyone on the network can edit the home page and change settings")
	} else if secret, err := auth.LoadOrCreateSecret(cfg.DataDir); err != nil {
		log.Error("cannot store the login key, refusing to start", "err", err)
		os.Exit(1)
	} else {
		login = auth.New(cfg.Auth.Username, cfg.Auth.Password, secret)
		log.Info("login required for changes", "user", login.Username())
	}

	// Automatic backups to BACKUP_DIR (e.g. its own volume), on the schedule
	// and with the retention set in Settings → Backup.
	var backups *autobackup.Manager
	if cfg.Backup.Dir != "" {
		backups = autobackup.New(cfg.Backup.Dir, cfg.Backup.Password, version, store, log)
		if err := backups.Check(); err != nil {
			log.Error("automatic backups switched off", "dir", cfg.Backup.Dir, "err", err)
			backups = nil
		} else {
			log.Info("automatic backups", "dir", cfg.Backup.Dir, "encrypted", backups.Encrypted())
		}
	}

	staticDir := os.Getenv("STATIC_DIR")
	geoClient := &http.Client{Timeout: 10 * time.Second}
	srv := &http.Server{
		Addr: cfg.HTTP.Addr,
		Handler: api.New(reg, mq, store, log, api.Options{
			CORSOrigins: cfg.HTTP.CORSOrigins,
			TrustProxy:  cfg.HTTP.TrustProxy,
			Version:     version,
			StaticDir:   staticDir,
			Weather: weather.NewService(
				weather.DefaultProviders(),
				&weather.OpenMeteoGeocoder{Client: geoClient},
				time.Local, // set TZ to choose the time zone for "today"
			).WithGeo(&weather.NominatimReverse{Client: geoClient}, &weather.IPLocator{Client: geoClient}),
			Calendar:   calendar.NewService(&http.Client{Timeout: 20 * time.Second}, time.Local),
			Auth:       login,
			AutoBackup: backups,
		}).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Schedules: switch lights at set times / sunrise / sunset (checked every 15 s).
	runner := schedule.NewRunner(func() ([]schedule.Schedule, schedule.Place) {
		list, err := store.LoadSchedules()
		if err != nil {
			log.Error("read schedules", "err", err)
		}
		w, _ := store.LoadWeather()
		p, _ := settings.SchedulePlace(w, time.Local)
		return list, p
	}, mq, registryTargets{reg}, log)
	go runner.Run(ctx)

	// Remotes: buttons on zigbee2mqtt remotes switch lights, outlets and groups.
	remotes := remote.NewEngine(func() []remote.Binding {
		l, err := store.LoadRemotes()
		if err != nil {
			log.Error("read remote actions", "err", err)
		}
		return l
	}, func(id string) ([]string, bool) { return groupDevices(store, id) }, remoteTargets{registryTargets{reg}}, mq, log)
	remotes.OnPress = func(p remote.Pressed) {
		reg.Broadcast(devices.Event{Type: "remote", ID: p.Remote, Button: p.Button, Press: p.Event})
	}
	go func() {
		events, cancel := reg.Subscribe()
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				return
			case e := <-events:
				if e.Type == "action" {
					remotes.Handle(e.ID, e.Action)
				}
			}
		}
	}()
	if backups != nil {
		go backups.Run(ctx)
	}

	go func() {
		log.Info("http listening", "addr", cfg.HTTP.Addr, "staticDir", staticDir, "dataDir", cfg.DataDir)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func logLevel() slog.Level {
	if os.Getenv("LOG_LEVEL") == "debug" {
		return slog.LevelDebug
	}
	return slog.LevelInfo
}

// registryTargets lets the schedule runner look devices up in the registry.
type registryTargets struct{ reg *devices.Registry }

func (r registryTargets) Target(id string) (schedule.Target, bool) {
	d, ok := r.reg.Get(id)
	if !ok {
		return schedule.Target{}, false
	}
	return schedule.Target{
		Name:          d.Name,
		StateProperty: d.Capabilities.StateProperty,
		Brightness:    d.Capabilities.Brightness,
		BrightnessMax: d.Capabilities.BrightnessMax,
	}, true
}

// Actions returns a remote's action values (for the remote engine).
func (r registryTargets) Actions(id string) ([]string, bool) {
	d, ok := r.reg.Get(id)
	if !ok || d.Kind != devices.KindRemote {
		return nil, false
	}
	return d.Capabilities.Actions, true
}

// remoteTargets adapts the registry for the remote engine (its Target differs
// from the schedule runner's).
type remoteTargets struct{ registryTargets }

func (r remoteTargets) Target(id string) (remote.Target, bool) {
	d, ok := r.reg.Get(id)
	if !ok || !d.Capabilities.OnOff {
		return remote.Target{}, false
	}
	prop := d.Capabilities.StateProperty
	if prop == "" {
		prop = "state"
	}
	on, _ := d.State[prop].(string)
	return remote.Target{
		Name:          d.Name,
		StateProperty: prop,
		On:            on == "ON",
		Brightness:    d.Capabilities.Brightness,
		BrightnessMax: d.Capabilities.BrightnessMax,
		ColorTemp:     d.Capabilities.ColorTemp,
		ColorTempMin:  d.Capabilities.ColorTempMin,
		ColorTempMax:  d.Capabilities.ColorTempMax,
	}, true
}

// groupDevices finds a group card on the home page (any tab) by id.
func groupDevices(store *settings.Store, id string) ([]string, bool) {
	d, err := store.LoadDashboard()
	if err != nil || d == nil {
		return nil, false
	}
	up := d.Upgrade()
	for _, tab := range up.Tabs {
		for _, col := range tab.Columns {
			for _, c := range col.Cards {
				if c.ID == id && c.Type == "group" {
					return c.DeviceIDs, true
				}
			}
		}
	}
	return nil, false
}
