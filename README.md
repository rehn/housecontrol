# housecontrol

A slimmed-down Home Assistant alternative for Zigbee bulbs and outlets.

```
browser ──► Go backend :8080 ──MQTT──► Mosquitto (Pi, e.g. 192.168.1.201) ◄── zigbee2mqtt
             ├─ /        Next.js UI (static export)
             └─ /api     REST + WebSocket
```

The backend subscribes to `<baseTopic>/#`, discovers devices from
`bridge/devices`, keeps their state in memory and streams changes to the UI over
a WebSocket. Commands go to `<baseTopic>/<device>/set`.

The app **starts without MQTT**. If no broker is configured, or it can't be
reached, the UI says so and you set the broker under **Settings**. Settings
saved there are stored in `<dataDir>/settings.json` and reconnect immediately,
without a restart.

## Run with Docker

```sh
docker compose up --build      # http://localhost:8080 → Settings → enter 192.168.1.201
```

or without compose:

```sh
docker build -t housecontrol .
docker run -p 8080:8080 -v housecontrol-data:/data housecontrol
```

The `housecontrol-data` volume keeps the saved settings, including the MQTT
password, across container restarts.

### Weather

Weather cards use a free service chosen under **Settings → Weather** (no API keys):
SMHI (Sweden/Nordics, `snow1g` API), MET Norway/Yr (works worldwide) or
Open-Meteo (worldwide). "Today" follows the server's time zone, so set `TZ`
(e.g. `Europe/Stockholm`, already in docker-compose.yml).

### Calendar

Calendar cards show iCal links added under **Settings → Calendars** (Google
Calendar's "secret address in iCal format", iCloud, Outlook, club schedules…).
Recurring events, exceptions, moved/cancelled occurrences, all-day events and
time zones (incl. Outlook's Windows names) are handled by `internal/calendar`.

### Login

Anyone who opens the app can use it: see the home page, weather and calendar,
and switch lights, outlets and groups (also brightness). Editing the home page
and all settings need the admin login. Set the password with `ADMIN_PASSWORD` (and optionally
`ADMIN_USERNAME`, default `admin`), or under `auth:` in `config.yaml`:

```sh
echo "ADMIN_PASSWORD='something long'" > .env   # read by start.sh and docker compose
```

A login lasts 30 days on a device (HttpOnly cookie). Changing the password logs
every device out. Five wrong passwords from one device block it for 5 minutes.
Without a password the login is switched off and everything is open (the
backend logs a warning and Settings shows one).

## Kubernetes (Helm)

The chart is in `deploy/helm/housecontrol`. It runs one pod (settings are files
on a small volume that survives `helm uninstall`), and can attach to a
Gateway API Gateway with an `HTTPRoute`.

Each release is published as an OCI chart on ghcr.io (no `helm repo add`
needed), already pointing at the matching Docker Hub image:

```sh
kubectl create namespace housecontrol
kubectl -n housecontrol create secret generic housecontrol-login --from-literal=ADMIN_PASSWORD='something long'

helm install housecontrol oci://ghcr.io/<github-user>/charts/housecontrol --version 0.1.0 \
  -n housecontrol \
  --set auth.existingSecret=housecontrol-login \
  --set mqtt.url=tcp://192.168.1.201:1883 \
  --set httpRoute.enabled=true \
  --set 'httpRoute.parentRefs[0].name=<your-gateway>' \
  --set 'httpRoute.parentRefs[0].namespace=<gateway-namespace>' \
  --set 'httpRoute.hostnames[0]=housecontrol.example.lan'

helm upgrade housecontrol oci://ghcr.io/<github-user>/charts/housecontrol --version 0.2.0 -n housecontrol --reuse-values
helm show values oci://ghcr.io/<github-user>/charts/housecontrol   # all options
```

From the source tree instead: `helm install housecontrol deploy/helm/housecontrol …`
plus `--set image.repository=docker.io/<dockerhub-user>/housecontrol --set image.tag=0.1.0`.

Or put the same in a `my-values.yaml` and use `-f my-values.yaml`; every
option is described in `values.yaml`. With the HTTPRoute on, the app takes the
client address from `X-Forwarded-For` (`TRUST_PROXY`), so a wrong-password
lockout only hits the guessing device. The pod needs to reach the MQTT broker
on your LAN (192.168.1.201:1883).

Chart tests: `helm lint` and [helm-unittest](https://github.com/helm-unittest/helm-unittest)
(`helm unittest deploy/helm/housecontrol`), also run by `verify.sh` when helm is installed.

## Releases (GitHub Actions → Docker Hub)

`.github/workflows/ci.yml` tests everything on every push. On `main` it also
pushes `<user>/housecontrol:main`; a version tag builds the release:

```sh
git tag v0.1.0 && git push origin v0.1.0
# -> docker.io/<user>/housecontrol:0.1.0, :0.1 and :latest (amd64 + arm64)
# -> oci://ghcr.io/<github-user>/charts/housecontrol:0.1.0 (the Helm chart)
# -> GitHub release v0.1.0 with housecontrol-0.1.0.tgz
```

One-time setup in the GitHub repo under Settings → Secrets and variables →
Actions: variable `DOCKERHUB_USERNAME`, secret `DOCKERHUB_TOKEN` (a Docker Hub
access token with Read & Write). The chart package on ghcr.io starts out
private: make it public once under your GitHub profile → Packages →
charts/housecontrol → Package settings → Change visibility.

## Develop (macOS)

```sh
chmod +x start.sh   # once
./start.sh          # backend :8080 + Next.js dev server :3000 with hot reload, Ctrl-C stops both
```

The first run creates `backend/config.yaml` and runs `go mod tidy` and `npm install`.
To run the parts by hand:

```sh
cd backend  && go run ./cmd/housecontrol          # API on :8080
cd frontend && npm run dev                         # UI on :3000, talks to :8080
```

## Configuration

MQTT settings are applied in this order (later wins):

1. `backend/config.yaml` (optional, see `config.example.yaml`)
2. env vars: `MQTT_URL`, `MQTT_USERNAME`, `MQTT_PASSWORD`, `MQTT_CLIENT_ID`, `Z2M_BASE_TOPIC`
3. settings saved in the UI (`<dataDir>/settings.json`)

Other settings: `HTTP_ADDR` (`:8080`), `DATA_DIR` (`data`), `STATIC_DIR` (serve a
built frontend), `CORS_ORIGINS`, `LOG_LEVEL=debug`, `ADMIN_USERNAME`, `ADMIN_PASSWORD`.

The broker can be given as `192.168.1.201`, `pi.lan:1883`, `tcp://…`, `ssl://…`
or `ws(s)://…`. A missing scheme or port is filled in automatically.

## API

| | |
|---|---|
| `GET /api/auth` | `{"required":true,"loggedIn":false}` |
| `POST /api/auth/login`, `POST /api/auth/logout` | `{"username","password"}` → session cookie `hc_session` (30 days) |
| `GET /api/devices` | devices with capabilities + state |
| `POST /api/devices/{ieee}/set` | `{"state":"ON"}`, `{"brightness":120}`, `{"color_temp":370}`, `transition`, `color` |
| `GET /api/settings/mqtt` | current broker settings + status (never returns the password) |
| `PUT /api/settings/mqtt` | save + reconnect. `password` omitted = keep, `""` = remove |
| `POST /api/settings/mqtt/test` | try a connection without changing anything |
| `GET /api/settings/weather`, `PUT …` | weather provider (`smhi` default, `metno`, `openmeteo`) + place |
| `GET /api/weather` | cached forecast (15 min): current + 5 days, normalised conditions |
| `GET /api/weather/places?q=…&lang=sv` | place search (Open-Meteo geocoding) |
| `GET /api/settings/calendars`, `PUT …` | iCal calendars (name, colour; the link is never sent back, only a hint) |
| `POST /api/settings/calendars/test` | try an iCal link |
| `GET /api/calendar[?from=YYYY-MM-DD]` | 7 days from `from` (default today), recurring events expanded, grouped per day; returns `from`, `to`, `today` |
| `GET /api/status` | `{"mqtt":{"state":"connected"…}}` (`unconfigured`/`connecting`/`connected`/`error`) |
| `GET /api/ws` | WebSocket: `status` + `devices` snapshot, then `state` / `availability` / `status` events |

With `ADMIN_PASSWORD` set, every change (`POST`/`PUT`) and every `GET` under
`/api/settings/` (except `ui`) and `/api/weather/…` answers `401` with
`"code":"unauthorized"` unless logged in. The cookie is only marked `Secure` behind
HTTPS, so keep the app on your LAN or put it behind a TLS proxy.

## Roadmap ideas

- rooms / renaming / favourites
- groups and scenes (`bridge/groups`)
- schedules and simple automations
- colour picker, multi-endpoint devices
