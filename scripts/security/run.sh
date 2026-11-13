#!/usr/bin/env sh
# Run the Foundation security test suite against a running API (technical design 17.10,
# PRD M-006 / AC-026). Needs the API reachable at API_BASE and the same DATABASE_URL and
# SECRETS_KEY the API runs with: the suite builds the sessions and reads the one-time
# activation and invitation links from the notification outbox.
set -eu
exec go tool saasctl security suite \
  --api "${API_BASE:-http://127.0.0.1:18080}" \
  --openapi apps/api/openapi \
  --fixture scripts/security/fixture.json \
  --out scripts/security/report.json "$@"
