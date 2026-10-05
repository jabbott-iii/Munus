# Builder and runtime use the same pinned Alpine release (same musl), which also
# keeps the package pins below valid; see intel/maint.md for how to bump them.
FROM golang:1.26-alpine3.24 AS source

WORKDIR /src

# Install build deps for CGO sqlite3 driver
RUN apk add --no-cache build-base=0.5-r4

# Cache dependencies first
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build for the image's own platform (go-sqlite3 needs CGO, which cannot
# cross-compile with the native toolchain). musl gives new threads a 128 KiB
# stack unless the program's PT_GNU_STACK asks for more; cgo calls into SQLite
# run on those threads, so both builds request glibc's usual 8 MiB.
FROM source AS builder
ARG VERSION=dev
RUN go version \
    && CGO_ENABLED=1 go build -ldflags "-X main.version=${VERSION} -extldflags '-Wl,-z,stack-size=8388608'" -o /out/munus .

# Fully static Linux release binary (cd.yml): musl instead of glibc, so no LGPL
# static-linking terms apply and it runs on any Linux distribution. Only built
# when this stage is targeted: docker build --target static --output ...
# The build tags must match linuxReleaseTags in tools/licenses; go version
# records the toolchain in the build log.
FROM source AS static-builder
ARG VERSION=dev
RUN go version \
    && CGO_ENABLED=1 go build \
    -tags sqlite_omit_load_extension,osusergo,netgo \
    -ldflags "-s -w -X main.version=${VERSION} -linkmode external -extldflags '-static -Wl,-z,stack-size=8388608'" \
    -o /out/munus .

FROM scratch AS static
COPY --from=static-builder /out/munus /munus

FROM alpine:3.24

# go-sqlite3 compiles SQLite into the binary, so no sqlite runtime package is
# needed. tzdata lets TZ (e.g. -e TZ=Europe/Berlin) select the local time zone
# for deadlines; without it Go silently falls back to UTC. Run as an
# unprivileged user that owns the data directory.
RUN apk add --no-cache ca-certificates=20260909-r0 tzdata=2026d-r0 \
    && addgroup -S -g 10001 munus \
    && adduser -S -D -H -u 10001 -G munus -h /app/data munus \
    && mkdir -p /app/data \
    && chown munus:munus /app/data

WORKDIR /app
COPY --from=builder /out/munus /usr/local/bin/munus
# License texts of Munus and of the third-party software compiled into it.
COPY --chmod=0644 LICENSE NOTICE THIRD_PARTY_LICENSES /usr/share/licenses/munus/

# Persist sqlite database file (munus.db) and import backups (HOME/.munus)
VOLUME ["/app/data"]

ENV MUNUS_DB_PATH=/app/data/munus.db \
    HOME=/app/data

USER 10001:10001

# This app is an interactive TUI/CLI
ENTRYPOINT ["munus"]
