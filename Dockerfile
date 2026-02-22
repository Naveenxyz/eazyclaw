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
    supervisor nginx \
    ca-certificates curl wget git gh jq tmux tree zip unzip \
    ripgrep fd-find \
    python3 python3-venv \
    && ln -sf /usr/bin/fdfind /usr/local/bin/fd \
    && rm -rf /var/lib/apt/lists/*

# Install uv
RUN curl -LsSf https://astral.sh/uv/install.sh | sh
ENV PATH="/root/.local/bin:$PATH"

# Install Node.js
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
    && apt-get install -y nodejs \
    && rm -rf /var/lib/apt/lists/*

# Create a default virtualenv so uv/pip don't hit the externally-managed error
RUN python3 -m venv /data/venv
ENV VIRTUAL_ENV=/data/venv
ENV PATH="/data/venv/bin:$PATH"

# Configure paths for mounted volume
ENV DATA_ROOT=/data/eazyclaw
ENV UV_CACHE_DIR=/data/eazyclaw/uv/cache
ENV UV_PYTHON_INSTALL_DIR=/data/eazyclaw/uv/python
ENV UV_TOOL_DIR=/data/eazyclaw/uv/tools
ENV UV_TOOL_BIN_DIR=/data/eazyclaw/uv/bin
ENV GH_CONFIG_DIR=/data/eazyclaw/auth/gh
ENV npm_config_prefix=/data/eazyclaw/npm/global
ENV npm_config_cache=/data/eazyclaw/npm/cache
ENV PATH="/data/eazyclaw/npm/global/bin:/data/eazyclaw/uv/bin:$PATH"

COPY --from=builder /eazyclaw /usr/local/bin/eazyclaw
COPY config.example.yaml /etc/eazyclaw/config.example.yaml
COPY AGENTS.md SOUL.md BOOTSTRAP.md IDENTITY.md USER.md HEARTBEAT.md MEMORY.md /defaults/
COPY skills/ /defaults/skills/
COPY supervisord.conf /etc/supervisor/conf.d/eazyclaw.conf
COPY nginx.conf /etc/nginx/conf.d/default.conf
RUN rm -f /etc/nginx/sites-enabled/default

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 80
EXPOSE 8080
ENTRYPOINT ["/entrypoint.sh"]
CMD ["supervisord", "-n"]
