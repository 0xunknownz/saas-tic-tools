#!/usr/bin/env sh
# Stop the single-host TEST environment. Data volumes are kept unless --volumes is passed.
# Usage: sh deploy/scripts/down.sh [--volumes]
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$ROOT"
docker compose --env-file deploy/.env -f deploy/compose/test.yaml down "$@"
