#!/bin/sh
# Quiesced PostgreSQL backup. Run with sh; backup directory must not exist.
set -eu
umask 077
if [ "$#" -ne 1 ]; then echo "usage: sh deploy/private/backup.sh NEW_BACKUP_DIR" >&2; exit 2; fi
: "${POSTGRES_USER:?set POSTGRES_USER}" "${POSTGRES_DB:?set POSTGRES_DB}"
compose=${PRIVATE_COMPOSE_FILE:-deploy/compose/private.yaml}
manifest=${RELEASE_MANIFEST:-release/manifest.json}
[ -f "$manifest" ] || { echo "release manifest missing" >&2; exit 1; }
running=$(docker compose -f "$compose" ps --status running --services api worker) || { echo "cannot inspect application services" >&2; exit 1; }
[ -z "$running" ] || { echo "stop api and worker before backup" >&2; exit 1; }
mkdir -- "$1" || { echo "backup directory must be new" >&2; exit 1; }
backup_dir=$(CDPATH= cd -- "$1" && pwd)
cp "$manifest" "$backup_dir/manifest.json"
docker compose -f "$compose" exec -T postgres pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc > "$backup_dir/database.dump"
docker compose -f "$compose" exec -T postgres psql -X -q -A -t -F '|' -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c 'SELECT module_id, migration_version, checksum FROM saasctl_migrations ORDER BY module_id, migration_version' > "$backup_dir/migrations.txt"
(cd "$backup_dir"; if command -v sha256sum >/dev/null 2>&1; then sha256sum database.dump manifest.json migrations.txt > SHA256SUMS; else shasum -a 256 database.dump manifest.json migrations.txt > SHA256SUMS; fi)
printf '%s
' "Backup created: $backup_dir; back up the object store and configuration separately."
