# syntax=docker/dockerfile:1

# --- Builder ---------------------------------------------------------------
FROM golang:1.19 AS build
WORKDIR /src

# Cache module downloads in their own layer, separate from source changes.
COPY go.mod go.sum ./
RUN go mod download

# Full source, including web/ — needed at build time because the GUI is
# compiled into the binary via //go:embed web (see web_embed.go).
COPY . .

# CGO_ENABLED=0 is mandatory: it keeps the pure-Go modernc.org/sqlite driver
# pure-Go and preserves the static-binary invariant already enforced by the
# Makefile's `build` target.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o /proxy ./cmd/proxy

# --- Final -------------------------------------------------------------------
# Distroless: no shell, no package manager, no web/ copy — the GUI assets
# already live inside the binary.
FROM gcr.io/distroless/static:nonroot AS final

COPY --from=build /proxy /proxy

# Runtime config is entirely env-driven (see internal/config, note 01):
#   PROXY_LISTEN  - listen address (default here documents the canon port only)
#   LOKI_URL      - must be supplied at deploy time; the localhost canon
#                   default is only correct for the native macOS binary
#   GRAFANA_URL   - must be supplied at deploy time, same reason as LOKI_URL
#   DB_PATH       - must point inside a mounted volume writable by uid 65532
#                   (nonroot) so the token store persists across container
#                   replacement
USER nonroot:nonroot
EXPOSE 4318

ENTRYPOINT ["/proxy"]
