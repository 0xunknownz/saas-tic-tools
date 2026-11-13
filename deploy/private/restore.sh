#!/bin/sh
# Restore only to a freshly created, empty target database.
set -eu
umask 077
if [ "$#" -ne 2 ] || [ "$2" != "--confirm-empty-target" ]; then echo "usage: sh deploy/private/restore.sh BACKUP_DIR --confirm-empty-target" >&2; exit 2; fi
: "${POSTGRES_USER:?set POSTGRES_USER}" "${POSTGRES_DB:?set POSTGRES_DB}"
compose=${PRIVATE_COMPOSE_FILE:-deploy/compose/private.yaml}
backup_dir=$(CDPATH= cd -- "$1" && pwd)
running=$(docker compose -f "$compose" ps --status running --services api worker) || { echo "cannot inspect application services" >&2; exit 1; }
[ -z "$running" ] || { echo "stop api and worker before restore" >&2; exit 1; }
[ -f "$backup_dir/SHA256SUMS" ] || { echo "backup checksum list missing" >&2; exit 1; }
actual=$(mktemp) || exit 1
trap 'rm -f "$actual"' EXIT HUP INT TERM
(cd "$backup_dir"; if command -v sha256sum >/dev/null 2>&1; then sha256sum database.dump manifest.json migrations.txt; else shasum -a 256 database.dump manifest.json migrations.txt; fi) > "$actual" || { echo "cannot calculate backup checksums" >&2; exit 1; }
cmp "$actual" "$backup_dir/SHA256SUMS" || { echo "backup checksum mismatch" >&2; exit 1; }
objects=$(docker compose -f "$compose" exec -T postgres psql -X -q -A -t -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "SELECT (SELECT count(*) FROM pg_namespace WHERE nspname NOT LIKE 'pg_%' AND nspname NOT IN ('public','information_schema')) + (SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public') + (SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='public') + (SELECT count(*) FROM pg_type t JOIN pg_namespace n ON n.oid=t.typnamespace WHERE n.nspname='public' AND t.typtype IN ('e','d','c','r'))") || { echo "cannot inspect target database" >&2; exit 1; }
[ "$objects" = 0 ] || { echo "target database is not empty" >&2; exit 1; }
docker compose -f "$compose" exec -T postgres pg_restore -U "$POSTGRES_USER" -d "$POSTGRES_DB" --clean --if-exists --no-owner --no-privileges --single-transaction --exit-on-error < "$backup_dir/database.dump"
docker compose -f "$compose" exec -T postgres psql -X -q -A -t -F '|' -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c 'SELECT module_id, migration_version, checksum FROM saasctl_migrations ORDER BY module_id, migration_version' > "$actual"
cmp "$backup_dir/migrations.txt" "$actual" || { echo "restored migration records differ from backup; do not start application" >&2; exit 1; }
printf '%s
' "Database restored and migration records match. Verify manifest, object store and configuration before starting."
