#!/bin/bash
set -e

# Ensure volume directories exist
mkdir -p /data/{workspace,sessions,skills,memory,uv/cache,npm/cache,npm/global,auth}

# Copy example config if none exists
if [ ! -f /data/config.yaml ]; then
  cp /etc/eazyclaw/config.example.yaml /data/config.yaml
  echo "Created default config at /data/config.yaml"
fi

exec "$@"
