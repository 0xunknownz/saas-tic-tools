#!/bin/sh
# Entrypoint of the Go application image (API, worker and one-shot migrate service).
#
# MIGRATE_ON_START=true applies the database migrations before the requested process starts and
# exits non-zero when they fail, so the process is never started on a half-migrated database.
# The compose template runs migrations in its own one-shot "migrate" service and leaves this off.
set -eu

if [ "${MIGRATE_ON_START:-false}" = "true" ]; then
    echo "entrypoint: applying database migrations before starting: ${*:-/app/api}" >&2
    /app/migrate -plan "${PLAN_PATH:-/app/migrations/plan.json}"
fi

exec "$@"
