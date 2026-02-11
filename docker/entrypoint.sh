#!/usr/bin/env bash
set -euo pipefail

# ─── Contextify All-in-One Entrypoint ───
# Starts PostgreSQL, Ollama, and Contextify server in a single container.
# Handles graceful shutdown via SIGTERM.

# ─── Config ───
POSTGRES_USER="${POSTGRES_USER:-contextify}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-contextify_local}"
POSTGRES_DB="${POSTGRES_DB:-contextify}"
CONTEXTIFY_PORT="${CONTEXTIFY_PORT:-8420}"
EMBEDDING_MODEL="${EMBEDDING_MODEL:-nomic-embed-text}"
PGDATA="${PGDATA:-/var/lib/postgresql/data}"

# PIDs for graceful shutdown
CONTEXTIFY_PID=""
OLLAMA_PID=""

cleanup() {
    echo "[entrypoint] shutting down..."

    if [ -n "$CONTEXTIFY_PID" ] && kill -0 "$CONTEXTIFY_PID" 2>/dev/null; then
        echo "[entrypoint] stopping contextify..."
        kill -TERM "$CONTEXTIFY_PID" 2>/dev/null || true
        wait "$CONTEXTIFY_PID" 2>/dev/null || true
    fi

    if [ -n "$OLLAMA_PID" ] && kill -0 "$OLLAMA_PID" 2>/dev/null; then
        echo "[entrypoint] stopping ollama..."
        kill -TERM "$OLLAMA_PID" 2>/dev/null || true
        wait "$OLLAMA_PID" 2>/dev/null || true
    fi

    echo "[entrypoint] stopping postgresql..."
    gosu postgres pg_ctl -D "$PGDATA" -m fast stop 2>/dev/null || true

    echo "[entrypoint] shutdown complete."
    exit 0
}

trap cleanup SIGTERM SIGINT SIGQUIT

# ─── 1. PostgreSQL ───
echo "[entrypoint] starting postgresql..."

# Initialize if PGDATA is empty
if [ ! -s "$PGDATA/PG_VERSION" ]; then
    echo "[entrypoint] initializing postgresql data directory..."
    gosu postgres initdb -D "$PGDATA" --auth-local=trust --auth-host=md5

    # Configure: listen only on localhost (internal to container)
    echo "listen_addresses = '127.0.0.1'" >> "$PGDATA/postgresql.conf"
    echo "port = 5432" >> "$PGDATA/postgresql.conf"
    echo "shared_buffers = 128MB" >> "$PGDATA/postgresql.conf"
    echo "work_mem = 8MB" >> "$PGDATA/postgresql.conf"
    echo "maintenance_work_mem = 64MB" >> "$PGDATA/postgresql.conf"
    echo "max_connections = 50" >> "$PGDATA/postgresql.conf"

    # HBA: allow password auth from localhost
    cat > "$PGDATA/pg_hba.conf" <<EOF
# TYPE  DATABASE        USER            ADDRESS                 METHOD
local   all             all                                     trust
host    all             all             127.0.0.1/32            md5
host    all             all             ::1/128                 md5
EOF
fi

# Start PostgreSQL (TCP for app + Unix socket for admin setup)
gosu postgres pg_ctl -D "$PGDATA" -o "-c listen_addresses=127.0.0.1 -c unix_socket_directories=/var/run/postgresql" -w start

# Wait for PostgreSQL to be ready
echo "[entrypoint] waiting for postgresql..."
for i in $(seq 1 30); do
    if gosu postgres pg_isready -q -h 127.0.0.1; then
        echo "[entrypoint] postgresql is ready."
        break
    fi
    if [ "$i" -eq 30 ]; then
        echo "[entrypoint] ERROR: postgresql did not become ready in 30s"
        exit 1
    fi
    sleep 1
done

# Create user, database, and extensions (idempotent)
# Use local socket (trust auth) for superuser setup
gosu postgres psql -v ON_ERROR_STOP=0 <<SQL
DO \$\$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '${POSTGRES_USER}') THEN
        CREATE ROLE ${POSTGRES_USER} WITH LOGIN PASSWORD '${POSTGRES_PASSWORD}';
    END IF;
END
\$\$;

SELECT 'CREATE DATABASE ${POSTGRES_DB} OWNER ${POSTGRES_USER}'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '${POSTGRES_DB}')
\gexec

\connect ${POSTGRES_DB}
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
GRANT ALL PRIVILEGES ON DATABASE ${POSTGRES_DB} TO ${POSTGRES_USER};
GRANT ALL ON SCHEMA public TO ${POSTGRES_USER};
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO ${POSTGRES_USER};
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO ${POSTGRES_USER};
SQL

echo "[entrypoint] postgresql initialized."

# ─── 2. Ollama ───
echo "[entrypoint] starting ollama..."
OLLAMA_HOST="127.0.0.1:11434" ollama serve &
OLLAMA_PID=$!

# Wait for Ollama to be ready
echo "[entrypoint] waiting for ollama..."
for i in $(seq 1 30); do
    if curl -sf http://127.0.0.1:11434/api/tags >/dev/null 2>&1; then
        echo "[entrypoint] ollama is ready."
        break
    fi
    if [ "$i" -eq 30 ]; then
        echo "[entrypoint] ERROR: ollama did not become ready in 30s"
        exit 1
    fi
    sleep 1
done

# ─── 3. Contextify Server ───
echo "[entrypoint] starting contextify server..."

export DATABASE_URL="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@127.0.0.1:5432/${POSTGRES_DB}?sslmode=disable"
export OLLAMA_URL="http://127.0.0.1:11434"
export SERVER_PORT="${CONTEXTIFY_PORT}"
export EMBEDDING_MODEL="${EMBEDDING_MODEL}"

/usr/local/bin/contextify &
CONTEXTIFY_PID=$!

echo "[entrypoint] all services started. contextify available at :${CONTEXTIFY_PORT}"

# Wait for the main process
wait "$CONTEXTIFY_PID"
