# syntax=docker/dockerfile:1

# ---- Build stage ----
FROM golang:1.25-alpine AS build

WORKDIR /src

# Pure-Go SQLite (modernc) — no CGO needed.
ENV CGO_ENABLED=0 \
    GOOS=linux

# Cache deps first for faster rebuilds.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Build the router (root package embeds web/static via go:embed).
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /out/flow-router .

# ---- Runtime stage ----
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S flow && adduser -S -G flow flow

# Data dir (SQLite, identity keys, etc.). Override with FLOW_ROUTER_DATA.
ENV FLOW_ROUTER_DATA=/data
RUN mkdir -p /data && chown -R flow:flow /data
VOLUME ["/data"]

COPY --from=build /out/flow-router /usr/local/bin/flow-router

USER flow
EXPOSE 2402

# Bind to all interfaces so the container is reachable from the host.
ENTRYPOINT ["/usr/local/bin/flow-router"]
CMD ["-addr", "0.0.0.0:2402"]
