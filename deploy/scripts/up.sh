#!/usr/bin/env sh
# Deploy the Product to a single-host TEST environment (PRD AC-027, technical design 11.15 / 14.3).
#
#   1. go tool saasctl build         -> release/manifest.json + release/migrations/plan.json
#   2. PostgreSQL only
#   3. go tool saasctl db migrate    -> a failure exits non-zero and the API is never started
#   4. docker compose build  -> always rebuilds the stack images (layer-cached; #110)
#   5. the rest of the stack -> API/worker additionally wait for the one-shot "migrate" service
#
# Usage: sh deploy/scripts/up.sh    (from any directory; requires deploy/.env)
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$ROOT"
COMPOSE="deploy/compose/test.yaml"
ENV_FILE="deploy/.env"

if [ ! -f "$ENV_FILE" ]; then
    echo "error: $ENV_FILE is missing; copy deploy/.env.example to deploy/.env and edit it" >&2
    exit 1
fi
# go tool builds the CLI pinned in apps/api/go.mod, so Go replaces the separate saasctl install.
for tool in docker go; do
    if ! command -v "$tool" >/dev/null 2>&1; then
        echo "error: $tool is not on PATH" >&2
        exit 1
    fi
done
if ! docker compose version >/dev/null 2>&1; then
    echo "error: the docker compose plugin is not available" >&2
    exit 1
fi

set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a

case "${SECRETS_KEY:-}" in
    "" | REPLACE_* | *REPLACE_WITH_*)
        echo "error: set SECRETS_KEY in $ENV_FILE (openssl rand -base64 32)" >&2
        exit 1
        ;;
esac

# The image build fetches the private Foundation module (design 5.3). A build-time vendor tree needs
# no credential; otherwise the build reads FOUNDATION_READ_TOKEN through a BuildKit secret.
if [ ! -d apps/api/vendor ] && [ -z "${FOUNDATION_READ_TOKEN:-}" ]; then
    echo "error: set FOUNDATION_READ_TOKEN in $ENV_FILE (read-only token for github.com/0xunknownz/saas.starter) or vendor the module under apps/api/vendor" >&2
    exit 1
fi

# The web/edge images install the private Foundation npm packages; the token is mounted as a BuildKit
# secret for that build step (design 4.2/5.3).
if [ -z "${NODE_AUTH_TOKEN:-}" ]; then
    echo "error: set NODE_AUTH_TOKEN in $ENV_FILE (read-only GitHub Packages token with read:packages for @0xunknownz)" >&2
    exit 1
fi

echo "==> go tool saasctl build (validation, compile, Release Manifest, migration plan)"
go tool saasctl build --image-tag "${API_IMAGE:-}" --platform "${TARGET_PLATFORM:-linux/amd64}"

echo "==> starting PostgreSQL"
docker compose --env-file "$ENV_FILE" -f "$COMPOSE" up -d --wait postgres

echo "==> applying migrations with go tool saasctl db migrate, before any application container starts"
if ! DATABASE_URL="${MIGRATE_DATABASE_URL:?set MIGRATE_DATABASE_URL in deploy/.env}" go tool saasctl db migrate --yes; then
    echo "error: migrations failed; the API was not started. Fix the migration and retry." >&2
    exit 1
fi

# A composed licence module starts in Missing state, which blocks every write (D-011); the
# documented smoke flow needs a licence to create tenants with --modules all. Install a 24h
# signed TEST licence through the product importer unless the deployment configured its own
# verification keys (#109). Private deployments are never affected by this path.
if grep -q '"foundation.license"' apps/api/internal/bootstrap/generated/composition.json && [ -z "${LICENSE_VERIFICATION_KEYS:-}" ]; then
    echo "==> installing a 24h test licence (test environment only, #109)"
    sh deploy/scripts/test-license.sh
    set -a
    # shellcheck disable=SC1090
    . "$ENV_FILE"
    set +a
fi

# Always build: docker compose up only builds when a tag is missing, so an existing
# API_IMAGE/WEB_IMAGE/EDGE_IMAGE would otherwise run stale code (#110). BuildKit's layer cache
# makes an unchanged rebuild cheap, and this step fails fast on a broken Dockerfile instead of
# silently starting the old images.
echo "==> building the stack images"
docker compose --env-file "$ENV_FILE" -f "$COMPOSE" build

echo "==> starting the application"
# --force-recreate: compose up does not restart a running container when only the IMAGE behind
# the tag changed, so without it a rebuilt demo-api:test would still be served by the old
# container (#110). The one-shot migrate service re-runs as a no-op ("database is up to date").
docker compose --env-file "$ENV_FILE" -f "$COMPOSE" up -d --wait --force-recreate

export DATABASE_URL="${MIGRATE_DATABASE_URL:-}"
echo
echo "Test environment is up:"
echo "  Admin: ${ADMIN_PUBLIC_URL:-http://localhost:8080/admin}"
echo "  Web:   ${WEB_PUBLIC_URL:-http://localhost:8080}"
echo "  API:   http://localhost:${HTTP_PORT:-8080}/healthz"
echo
BOOTSTRAP_EMAIL="${BOOTSTRAP_EMAIL:-root@example.com}"
if bootstrap_output=$(go tool saasctl admin bootstrap issue --email "$BOOTSTRAP_EMAIL" --operator deploy-up --admin-url "${ADMIN_PUBLIC_URL:-http://localhost:8080}" 2>&1); then
    echo "Create the first platform administrator (single use):"
    printf '%s
' "$bootstrap_output"
else
    echo "Platform administrator already initialized; sign in at the Admin URL."
fi
