#!/bin/bash
set -euo pipefail

DATA_ROOT="${DATA_ROOT:-/data/eazyclaw}"
MEMORY_DIR="${DATA_ROOT}/memory"

# Railway sets PORT dynamically; map it to WEB_PORT so eazyclaw picks it up.
if [ -n "${PORT:-}" ] && [ -z "${WEB_PORT:-}" ]; then
  export WEB_PORT="$PORT"
fi

mkdir -p "${DATA_ROOT}"/{workspace,sessions,skills,memory,cron,uv/cache,uv/python,uv/tools,uv/bin,npm/cache,npm/global,auth,auth/gh}

# Ensure Python venv exists on the persistent volume.
if [ ! -f /data/venv/bin/python ]; then
  python3 -m venv /data/venv
  echo "Created Python venv at /data/venv"
fi

# Copy example config if none exists in the new root.
if [ ! -f "${DATA_ROOT}/config.yaml" ]; then
  cp /etc/eazyclaw/config.example.yaml "${DATA_ROOT}/config.yaml"
  echo "Created default config at ${DATA_ROOT}/config.yaml"
fi

# Seed default memory bootstrap files (all visible in Memory Explorer).
if [ ! -f "${MEMORY_DIR}/AGENTS.md" ]; then
  cp /defaults/AGENTS.md "${MEMORY_DIR}/AGENTS.md"
  echo "Created default AGENTS.md at ${MEMORY_DIR}/AGENTS.md"
fi

if [ ! -f "${MEMORY_DIR}/SOUL.md" ]; then
  cp /defaults/SOUL.md "${MEMORY_DIR}/SOUL.md"
  echo "Created default SOUL.md at ${MEMORY_DIR}/SOUL.md"
fi

if [ ! -f "${MEMORY_DIR}/IDENTITY.md" ]; then
  cp /defaults/IDENTITY.md "${MEMORY_DIR}/IDENTITY.md"
  echo "Created default IDENTITY.md at ${MEMORY_DIR}/IDENTITY.md"
fi

if [ ! -f "${MEMORY_DIR}/USER.md" ]; then
  cp /defaults/USER.md "${MEMORY_DIR}/USER.md"
  echo "Created default USER.md at ${MEMORY_DIR}/USER.md"
fi

if [ ! -f "${MEMORY_DIR}/HEARTBEAT.md" ]; then
  cp /defaults/HEARTBEAT.md "${MEMORY_DIR}/HEARTBEAT.md"
  echo "Created default HEARTBEAT.md at ${MEMORY_DIR}/HEARTBEAT.md"
fi

if [ ! -f "${MEMORY_DIR}/MEMORY.md" ]; then
  cp /defaults/MEMORY.md "${MEMORY_DIR}/MEMORY.md"
  echo "Created default MEMORY.md at ${MEMORY_DIR}/MEMORY.md"
fi

# Seed default skills if the skills directory is empty.
SKILLS_DIR="${DATA_ROOT}/skills"
if [ -d /defaults/skills ]; then
  for skill_dir in /defaults/skills/*/; do
    skill_name="$(basename "$skill_dir")"
    if [ ! -d "${SKILLS_DIR}/${skill_name}" ]; then
      cp -r "$skill_dir" "${SKILLS_DIR}/${skill_name}"
      echo "Installed default skill: ${skill_name}"
    fi
  done
fi

# Seed today's day-wise memory file if missing.
TODAY="$(date +%F)"
if [ ! -f "${MEMORY_DIR}/${TODAY}.md" ]; then
  cat > "${MEMORY_DIR}/${TODAY}.md" <<EOF
# Daily Memory (${TODAY})

Append concise day-wise updates here.
EOF
  echo "Created daily memory file at ${MEMORY_DIR}/${TODAY}.md"
fi

exec "$@"
