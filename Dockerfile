# ── Stage 1: build ────────────────────────────────────────────
# Go 1.24 is the minimum version required by dependencies (fhttp needs
# go >= 1.24.1). The original "1.27" does not exist on Docker Hub.
# Using 1.24-alpine gives a reproducible, stable build.
FROM golang:1.24-alpine AS builder

WORKDIR /app

# cache deps first
COPY go.mod go.sum ./
RUN go mod download

# copy source
COPY *.go ./

# -trimpath  → reproducible binary, removes local path info
# -s -w      → strip debug symbols (saves ~30% binary size)
# CGO_ENABLED=0 → fully static, no glibc dependency
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -trimpath -o checker .

# ── Stage 2: minimal runtime ───────────────────────────────────
FROM alpine:3.20

# ca-certificates → TLS roots needed for HTTPS to Shopify
# tzdata          → time zone data (used by Go stdlib internally)
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /app/checker .
COPY sites.txt .

# ── Runtime environment ────────────────────────────────────────
# GOGC=400   → GC fires when heap doubles in size.
#              Default is 100 (fire when heap grows 100%).
#              Under high-throughput I/O workloads this halves GC CPU.
#              Render will also inject this from render.yaml envVars.
# GOMEMLIMIT → Prevent OOM-kill. Set ~85% of container RAM.
#              Render starter (2 GB) → 1700 MiB.
#              For the 24 GB Ender VPS, override at runtime:
#                  GOMEMLIMIT=20GiB ./checker
#              or export GOMEMLIMIT=20GiB before launching.
# PORT       → The port the Go binary listens on (reads os.Getenv).
# HOST       → Bind address (0.0.0.0 = all interfaces).
ENV PORT=8080 \
    HOST=0.0.0.0 \
    GOGC=400 \
    GOMEMLIMIT=1700MiB

EXPOSE 8080

# The binary binds to 0.0.0.0:$PORT internally (see main.go),
# so the reverse proxy / load balancer can reach it from outside
# the container without any extra configuration.
CMD ["./checker"]
