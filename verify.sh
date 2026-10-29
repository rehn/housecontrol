#!/usr/bin/env bash
# Build and test everything: Go backend, Next.js frontend, Docker image.
# Usage: ./verify.sh            (SKIP_DOCKER=1 ./verify.sh to skip the image build)
# Output is also written to verify.log so it can be shared.
set -uo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
LOG="$ROOT/verify.log"
: >"$LOG"
exec > >(tee -a "$LOG") 2>&1

RESULTS=()
FAILED=0
step() { # step <name> <dir> <command...>
  local name="$1" dir="$2"; shift 2
  printf '\n\033[1;34m==> %s\033[0m  (%s)\n' "$name" "$*"
  local start=$SECONDS
  if (cd "$dir" && "$@"); then
    RESULTS+=("PASS  $name ($((SECONDS - start))s)")
  else
    RESULTS+=("FAIL  $name")
    FAILED=1
  fi
}
gofmt_check() {
  local out; out="$(gofmt -l .)"
  [[ -z "$out" ]] || { echo "not gofmt'ed:"; echo "$out"; return 1; }
}

echo "housecontrol verify  $(date)"
echo "go: $(go version 2>/dev/null || echo missing)   node: $(node -v 2>/dev/null || echo missing)   docker: $(docker --version 2>/dev/null || echo missing)"

# --- backend -----------------------------------------------------------------
if command -v go >/dev/null; then
  step "go mod tidy"  "$ROOT/backend" go mod tidy
  step "gofmt"        "$ROOT/backend" gofmt_check
  step "go vet"       "$ROOT/backend" go vet ./...
  step "go test"      "$ROOT/backend" go test -count=1 ./...
  step "go build"     "$ROOT/backend" go build -o /dev/null ./cmd/housecontrol
else
  RESULTS+=("SKIP  backend (Go not installed: brew install go)"); FAILED=1
fi

# --- frontend ----------------------------------------------------------------
if command -v npm >/dev/null; then
  step "npm install"  "$ROOT/frontend" npm install --no-audit --no-fund
  step "npm test"     "$ROOT/frontend" npm test
  step "next build"   "$ROOT/frontend" npm run build
else
  RESULTS+=("SKIP  frontend (Node not installed: brew install node)"); FAILED=1
fi

# --- helm chart --------------------------------------------------------------
CHART="$ROOT/deploy/helm/housecontrol"
if command -v helm >/dev/null; then
  step "helm lint" "$ROOT" helm lint "$CHART" --set image.repository=example/housecontrol
  if helm plugin list 2>/dev/null | grep -q unittest; then
    step "helm unittest" "$ROOT" helm unittest "$CHART"
  else
    RESULTS+=("SKIP  helm unittest (helm plugin install https://github.com/helm-unittest/helm-unittest)")
  fi
else
  RESULTS+=("SKIP  helm chart (brew install helm)")
fi

# --- docker ------------------------------------------------------------------
if [[ "${SKIP_DOCKER:-}" == "1" ]]; then
  RESULTS+=("SKIP  docker build (SKIP_DOCKER=1)")
elif command -v docker >/dev/null && docker info >/dev/null 2>&1; then
  step "docker build" "$ROOT" docker build -t housecontrol:local .
else
  RESULTS+=("SKIP  docker build (Docker not running)")
fi

printf '\n\033[1m==== summary ====\033[0m\n'
printf '%s\n' "${RESULTS[@]}"
[[ $FAILED == 0 ]] && echo "ALL GOOD" || echo "SOMETHING FAILED - see above (also saved in verify.log)"
exit $FAILED
