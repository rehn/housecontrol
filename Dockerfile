# One image: Next.js frontend exported as static files, served by the Go backend.
#   docker build -t housecontrol .
#   docker buildx build --platform linux/amd64,linux/arm64 -t <user>/housecontrol .
# The build stages run on the build machine's own CPU (static files and a
# cross-compiled Go binary), so multi-arch builds need no slow emulation.
#   docker run -p 8080:8080 -v housecontrol-data:/data housecontrol
# then open http://localhost:8080 and configure MQTT under Settings.

# --- frontend -----------------------------------------------------------------
FROM --platform=$BUILDPLATFORM node:22-alpine AS web
WORKDIR /web
COPY frontend/package.json frontend/package-lock.json* ./
RUN if [ -f package-lock.json ]; then npm ci; else npm install; fi
COPY frontend/ ./
ENV NEXT_TELEMETRY_DISABLED=1
RUN npm run build   # -> /web/out

# --- backend ------------------------------------------------------------------
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS api
ARG TARGETOS TARGETARCH
ARG VERSION=dev
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -X main.version=$VERSION" -o /out/housecontrol ./cmd/housecontrol \
 && mkdir -p /out/data

# --- runtime ------------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=api /out/housecontrol /housecontrol
COPY --from=web /web/out /web
COPY --from=api --chown=65532:65532 /out/data /data
ENV HTTP_ADDR=:8080 \
    STATIC_DIR=/web \
    DATA_DIR=/data \
    CONFIG_FILE=/config/config.yaml
# /data holds settings saved from the UI (incl. the MQTT password).
VOLUME /data
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/housecontrol"]
