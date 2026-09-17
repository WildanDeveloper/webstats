#!/bin/sh
# Apply all migrations in order. Usage: ./migrate.sh
set -e
DIR="$(cd "$(dirname "$0")" && pwd)"
HOST="${PGHOST:-localhost}"
PORT="${PGPORT:-5432}"
USER="${PGUSER:-webstats}"
DB="${PGDATABASE:-webstats}"
export PGPASSWORD="${PGPASSWORD:-webstats}"

for f in "$DIR"/migrations/*.sql; do
  echo "applying $(basename "$f")"
  if [ -n "${DATABASE_URL:-}" ]; then
    psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f "$f" -q
  else
    psql -h "$HOST" -p "$PORT" -U "$USER" -d "$DB" -v ON_ERROR_STOP=1 -f "$f" -q
  fi
done
echo "migrations done"
