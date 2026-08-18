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
  psql -h "$HOST" -p "$PORT" -U "$USER" -d "$DB" -v ON_ERROR_STOP=1 -f "$f" -q
done
echo "migrations done"
