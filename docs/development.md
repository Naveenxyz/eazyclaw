# Development

## Prerequisites

- Go 1.25+
- Node.js 22+
- Docker

## Local Build

### Backend

```sh
go vet ./...
go build -o eazyclaw ./cmd/eazyclaw/
```

### Frontend

```sh
cd ui && npm install && npm run build
```

### Full Docker Build

```sh
docker compose build && docker compose up -d
```

## Running Tests

```sh
go test ./...
```

## Docker Build Details

The Dockerfile uses a multi-stage build:

1. **ui-builder** — builds the React frontend
2. **go-builder** — compiles the Go binary
3. **runtime** — Debian-based final image

The runtime image includes the following tools alongside the EazyClaw binary:
`git`, `gh`, `rg`, `fd`, `python3`, `uv`, `node`, `npm`

## Data Volume

EazyClaw uses a `/data` Docker volume. All persistent data (memory, skills, sessions, config) lives under `/data/eazyclaw/` inside the container.

## Environment Setup

Copy `.env.example` to `.env` and fill in your API keys before starting:

```sh
cp .env.example .env
# Edit .env and set your LLM provider API keys
```
