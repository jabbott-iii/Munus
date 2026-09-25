# Builder and runtime use the same pinned Alpine release (same musl), which also
# keeps the package pins below valid; see intel/maint.md for how to bump them.
FROM golang:1.26-alpine3.24 AS builder

WORKDIR /src

# Install build deps for CGO sqlite3 driver
RUN apk add --no-cache build-base=0.5-r4

# Cache dependencies first
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build for the image's own platform (go-sqlite3 needs CGO,
# which cannot cross-compile with the native toolchain).
COPY . .
RUN CGO_ENABLED=1 go build -o /out/munus .

FROM alpine:3.24

# go-sqlite3 compiles SQLite into the binary, so no sqlite runtime package is
# needed. Run as an unprivileged user that owns the data directory.
RUN apk add --no-cache ca-certificates=20260909-r0 \
    && addgroup -S -g 10001 munus \
    && adduser -S -D -H -u 10001 -G munus -h /app/data munus \
    && mkdir -p /app/data \
    && chown munus:munus /app/data

WORKDIR /app
COPY --from=builder /out/munus /usr/local/bin/munus

# Persist sqlite database file (munus.db) and import backups (HOME/.munus)
VOLUME ["/app/data"]

ENV MUNUS_DB_PATH=/app/data/munus.db \
    HOME=/app/data

USER 10001:10001

# This app is an interactive TUI/CLI
ENTRYPOINT ["munus"]
