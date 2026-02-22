# Stage 1: Build frontend
FROM node:22-bookworm-slim AS ui-builder
WORKDIR /ui
COPY ui/package.json ./
RUN npm install
COPY ui/ .
RUN npm run build

# Stage 2: Build Go (with embedded frontend)
FROM golang:1.25-bookworm AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=ui-builder /ui/dist ./cmd/eazyclaw/ui-dist
RUN CGO_ENABLED=0 go build -o /eazyclaw ./cmd/eazyclaw/

# Stage 3: Runtime
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates curl git chromium \
    python3 python3-venv \
    && rm -rf /var/lib/apt/lists/*

# Install uv
RUN curl -LsSf https://astral.sh/uv/install.sh | sh
ENV PATH="/root/.local/bin:$PATH"

# Install Node.js
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
    && apt-get install -y nodejs \
    && rm -rf /var/lib/apt/lists/*

# Configure paths for mounted volume
ENV UV_CACHE_DIR=/data/uv/cache
ENV UV_PYTHON_INSTALL_DIR=/data/uv/python
ENV npm_config_prefix=/data/npm/global
ENV npm_config_cache=/data/npm/cache
ENV PATH="/data/npm/global/bin:/data/uv/bin:$PATH"

COPY --from=builder /eazyclaw /usr/local/bin/eazyclaw
COPY config.example.yaml /etc/eazyclaw/config.example.yaml
COPY AGENTS.md SOUL.md BOOTSTRAP.md IDENTITY.md USER.md HEARTBEAT.md MEMORY.md /defaults/
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 8080
VOLUME /data
ENTRYPOINT ["/entrypoint.sh"]
CMD ["eazyclaw", "serve"]
