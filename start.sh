#!/usr/bin/env bash
# Start housecontrol locally on macOS: Go backend (:8080) + Next.js dev server (:3000).
# Usage: ./start.sh            (Ctrl-C stops both)
#        MQTT_URL=tcp://192.168.1.201:1883 ./start.sh   (override the broker)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
BACKEND="$ROOT/backend"
FRONTEND="$ROOT/frontend"
BACKEND_PORT="${BACKEND_PORT:-8080}"
FRONTEND_PORT="${FRONTEND_PORT:-3000}"

info() { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
fail() { printf '\033[1;31merror:\033[0m %s\n' "$*" >&2; exit 1; }

# optional .env next to this script (same file docker compose reads), e.g.
#   ADMIN_PASSWORD='something long'
if [[ -f "$ROOT/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$ROOT/.env"
  set +a
fi

# --- prerequisites -----------------------------------------------------------
command -v go   >/dev/null || fail "Go not found. Install with: brew install go"
command -v node >/dev/null || fail "Node.js not found. Install with: brew install node"
command -v npm  >/dev/null || fail "npm not found. Install with: brew install node"

for port in "$BACKEND_PORT" "$FRONTEND_PORT"; do
  if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
    fail "port $port is already in use (lsof -nP -iTCP:$port -sTCP:LISTEN)"
  fi
done

# --- backend setup -----------------------------------------------------------
if [[ ! -f "$BACKEND/config.yaml" ]]; then
  info "creating backend/config.yaml from config.example.yaml"
  cp "$BACKEND/config.example.yaml" "$BACKEND/config.yaml"
fi
if [[ ! -f "$BACKEND/go.sum" ]]; then
  info "fetching Go dependencies (go mod tidy)"
  (cd "$BACKEND" && go mod tidy)
fi

# --- frontend setup ----------------------------------------------------------
# install when node_modules is missing or package.json changed since the last install
if [[ ! -d "$FRONTEND/node_modules" || "$FRONTEND/package.json" -nt "$FRONTEND/node_modules/.package-lock.json" ]]; then
  info "installing frontend dependencies (npm install)"
  (cd "$FRONTEND" && npm install)
fi

# --- run ---------------------------------------------------------------------
PIDS=()
cleanup() {
  local rc=$?
  trap - INT TERM EXIT
  info "stopping..."
  # Signal the whole process group so `go run`'s binary and Next.js's
  # child processes stop too; this script ignores the signal itself.
  trap '' TERM
  kill -TERM 0 2>/dev/null || true
  wait 2>/dev/null || true
  exit "$rc"
}
trap cleanup INT TERM EXIT

info "starting backend on :$BACKEND_PORT"
(cd "$BACKEND" && HTTP_ADDR=":$BACKEND_PORT" \
  CORS_ORIGINS="${CORS_ORIGINS:-http://localhost:$FRONTEND_PORT}" \
  exec go run ./cmd/housecontrol) 2>&1 | sed -u 's/^/[backend]  /' &
PIDS+=($!)

[[ -n "${ADMIN_PASSWORD:-}" ]] || info "no ADMIN_PASSWORD: login is off, anyone can change things (see README → Login)"

# wait for the backend to answer before starting the frontend
for _ in $(seq 1 60); do
  curl -fs "http://localhost:$BACKEND_PORT/healthz" >/dev/null 2>&1 && break
  sleep 1
done
curl -fs "http://localhost:$BACKEND_PORT/healthz" >/dev/null 2>&1 \
  || fail "backend did not start, check the [backend] output above"

info "starting frontend on :$FRONTEND_PORT"
(cd "$FRONTEND" && NEXT_PUBLIC_API_BASE="http://localhost:$BACKEND_PORT" NEXT_DIST_DIR=.next-dev \
  exec npx next dev -p "$FRONTEND_PORT") 2>&1 | sed -u 's/^/[frontend] /' &
PIDS+=($!)

info "open http://localhost:$FRONTEND_PORT  (Ctrl-C to stop)"
( sleep 4 && open "http://localhost:$FRONTEND_PORT" ) >/dev/null 2>&1 &

# exit when either process dies (bash 3.2 on macOS has no `wait -n`)
while true; do
  for pid in "${PIDS[@]}"; do
    kill -0 "$pid" 2>/dev/null || { info "a process exited"; exit 1; }
  done
  sleep 2
done
