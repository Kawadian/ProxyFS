# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.24.13
ARG NODE_VERSION=22-bookworm-slim

# -----------------------------------------------------------------------------
# Frontend build
# -----------------------------------------------------------------------------
FROM node:${NODE_VERSION} AS frontend
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm ci --ignore-scripts 2>/dev/null || npm install
COPY frontend/ ./
RUN npm run build

# -----------------------------------------------------------------------------
# Go build (embed frontend assets)
# -----------------------------------------------------------------------------
FROM golang:${GO_VERSION}-bookworm AS backend
WORKDIR /src

RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc libc6-dev libfuse3-dev pkg-config \
    && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
COPY --from=frontend /src/web/dist ./cmd/lxcfh/static

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags "\
    -s -w \
    -X github.com/lxcfh/lxcfh/internal/version.Version=${VERSION}" \
    -o /out/lxcfh ./cmd/lxcfh

RUN CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags "-s -w" \
    -o /out/lxcfh-fuse ./cmd/fusemount

# -----------------------------------------------------------------------------
# Runtime
# -----------------------------------------------------------------------------
FROM debian:bookworm-slim AS runtime

LABEL org.opencontainers.image.title="LXC File Hub" \
      org.opencontainers.image.description="lxcfh hub with embedded web UI and optional FUSE export" \
      org.opencontainers.image.source="https://github.com/lxcfh/lxcfh"

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    fuse3 \
    samba \
    tini \
    wget \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 1000 lxcfh \
    && useradd --uid 1000 --gid lxcfh --create-home --shell /usr/sbin/nologin lxcfh

WORKDIR /app

COPY --from=backend /out/lxcfh /usr/local/bin/lxcfh
COPY config/samba/smb.conf /etc/samba/smb.conf
COPY scripts/entrypoint.sh /usr/local/bin/entrypoint.sh

RUN chmod +x /usr/local/bin/entrypoint.sh \
    && mkdir -p /var/lib/lxcfh /fuse-mount /run/secrets /run/samba /var/log/samba \
    && chown -R lxcfh:lxcfh /var/lib/lxcfh /fuse-mount

ENV LXCFH_BIND_HOST=0.0.0.0 \
    LXCFH_BIND_PORT=8080 \
    LXCFH_DATA_DIR=/var/lib/lxcfh \
    LXCFH_DB_PATH=/var/lib/lxcfh/lxcfh.db \
    LXCFH_MASTER_KEY_PATH=/run/secrets/master.key \
    LXCFH_FUSE_MOUNT=/fuse-mount

EXPOSE 8080 2022 445 139

VOLUME ["/var/lib/lxcfh"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/health/live | grep -q alive

ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/entrypoint.sh"]
CMD []
