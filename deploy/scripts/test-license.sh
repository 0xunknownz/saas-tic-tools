#!/usr/bin/env sh
# Install a 24-hour signed TEST licence into the composed test environment (#109).
#
#   sh deploy/scripts/test-license.sh      (run while PostgreSQL is up and migrated)
#
# Why: a composed licence module starts in Missing state, which blocks every write
# (D-011), and deploy/.env carries no verification keys, so the documented smoke flow
# cannot create tenants with --modules all. This mirrors the Starter's CI fixture
# (scripts/m5-security-license-fixture.sh): generate an ephemeral Ed25519 key, sign a
# 24h licence for THIS product and instance, install it through the product's own
# licence importer, and record the public key in a managed block of deploy/.env.
# The private key never leaves the process and is never written to disk.
#
# Commercial boundaries stay intact:
#   - refuses unless APP_ENV=development (only the test compose file sets it);
#   - refuses when LICENSE_VERIFICATION_KEYS is already configured: the deployment's
#     own licence flow always wins;
#   - never touches the private deployment templates or a real signed licence;
#   - after the 24h expiry, writes are blocked again; reset the test volume
#     (docker compose ... down -v) and re-run up.sh to install a fresh one.
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$ROOT"
ENV_FILE="deploy/.env"
COMPOSE="deploy/compose/test.yaml"

if [ ! -f "$ENV_FILE" ]; then
    echo "error: $ENV_FILE is missing; copy deploy/.env.example to deploy/.env and edit it" >&2
    exit 1
fi
set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a

if [ "$(printf '%s' "${APP_ENV:-development}")" != "development" ]; then
    echo "error: test-license only runs with APP_ENV=development (the test environment)" >&2
    exit 1
fi
if [ -n "${LICENSE_VERIFICATION_KEYS:-}" ]; then
    echo "test-license: LICENSE_VERIFICATION_KEYS is already configured; keeping the deployment's own licence flow."
    exit 0
fi
if ! grep -q '"foundation.license"' apps/api/internal/bootstrap/generated/composition.json; then
    echo "test-license: this composition has no licence module; nothing to do."
    exit 0
fi

: "${POSTGRES_USER:?set POSTGRES_USER in deploy/.env}"
: "${POSTGRES_PASSWORD:?set POSTGRES_PASSWORD in deploy/.env}"
: "${POSTGRES_DB:?set POSTGRES_DB in deploy/.env}"
PSQL="docker compose --env-file $ENV_FILE -f $COMPOSE exec -T postgres psql -U $POSTGRES_USER -d $POSTGRES_DB -Atc"
installed=$($PSQL "SELECT count(*) FROM license_state WHERE singleton = true AND installed_claims IS NOT NULL" 2>/dev/null || echo "")
if [ "$installed" = "" ]; then
    echo "error: cannot read license_state; is PostgreSQL up and migrated? Run deploy/scripts/up.sh." >&2
    exit 1
fi
if [ "$installed" != "0" ]; then
    expires=$($PSQL "SELECT coalesce(expires_at::text,'-') FROM license_state WHERE singleton = true")
    echo "test-license: a licence is already installed (expires $expires); leaving it untouched."
    echo "test-license: to replace it, reset the test volume (docker compose ... down -v) and re-run up.sh."
    exit 0
fi

# The importer lives inside the Product module, so the fixture compiles against it.
# It runs in a golang container attached to the compose network (module cache in a
# named volume); the temporary package exists only during the run.
module=$(sed -n 's/^module //p' apps/api/go.mod)
tmpdir=$(mktemp -d "$ROOT/apps/api/cmd/testlicense.XXXXXX")
cleanup() { rm -rf "$tmpdir"; rm -f "$ROOT/deploy/.test-license.env"; }
trap cleanup EXIT
cat > "$tmpdir/main.go" <<'GOEOF'
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	"__MODULE__/internal/bootstrap/generated"
	"github.com/0xunknownz/saas.starter/foundation/license"
	"github.com/0xunknownz/saas.starter/foundation/audit"
	"github.com/0xunknownz/saas.starter/foundation/db"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	database, err := db.NewService(db.Config{URL: os.Getenv("DATABASE_URL")}, db.Deps{})
	check(err)
	check(database.Start(ctx))
	defer database.Stop(context.Background())
	writer, err := audit.NewService(audit.Config{}, audit.Deps{Db: database})
	check(err)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	check(err)
	const keyID = "test-license"
	instance := envOr("LICENSE_INSTANCE_ID", "test-instance")
	version := envOr("LICENSE_PRODUCT_VERSION", "1.0.0")
	config := license.ImportConfig{Keys: map[string]ed25519.PublicKey{keyID: public},
		Product: generated.Product.ID, Instance: instance, Version: version}
	usage, err := license.NewInstanceUsage(config)
	check(err)
	importer, err := license.NewImporter(database, writer, usage, config)
	check(err)
	now := time.Now().UTC().Truncate(time.Second)
	candidate := license.Candidate{ProductID: generated.Product.ID, InstanceID: instance,
		CustomerID: "test-environment", MinVersion: version, MaxVersion: version,
		IssuedAt: now.Add(-time.Minute), NotBefore: now.Add(-time.Minute),
		ExpiresAt: now.Add(24 * time.Hour), Features: []string{"advanced"},
		Limits: map[string]int64{"tenants": 1000, "users": 10000}}
	payload, err := json.Marshal(candidate)
	check(err)
	envelope, err := json.Marshal(map[string]string{"keyId": keyID,
		"payload":   base64.StdEncoding.EncodeToString(payload),
		"signature": base64.StdEncoding.EncodeToString(ed25519.Sign(private, payload))})
	check(err)
	preview, err := importer.CreatePreview(ctx, envelope)
	check(err)
	_, err = importer.Apply(ctx, preview.ID, "test-license-"+preview.ID, "deploy:test-license", true)
	check(err)
	encoded, err := x509.MarshalPKIXPublicKey(public)
	check(err)
	keys, err := json.Marshal(map[string]string{keyID: string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: encoded}))})
	check(err)
	// The JSON holds a PEM public key only; it contains no shell apostrophes, so the
	// single-quoted line below is safe for both '.' sourcing and compose env files.
	env := fmt.Sprintf("LICENSE_VERIFICATION_KEYS='%s'\nLICENSE_INSTANCE_ID=%s\nLICENSE_PRODUCT_VERSION=%s\n", keys, instance, version)
	check(os.WriteFile(os.Args[1], []byte(env), 0o600))
	fmt.Printf("TEST_LICENSE_OK expires=%s\n", now.Add(24*time.Hour).Format(time.RFC3339))
}
GOEOF
sed "s|__MODULE__|$module|g" "$tmpdir/main.go" > "$tmpdir/main.go.new" && mv "$tmpdir/main.go.new" "$tmpdir/main.go"
rel=$(printf '%s' "${tmpdir#$ROOT/apps/api/}")

docker run --rm \
  --network "${COMPOSE_PROJECT_NAME:-saas-tic-tools-test}_default" \
  -v "$ROOT":/src -w /src/apps/api \
  -v saas-testlicense-gomod:/go/pkg/mod \
  -e DATABASE_URL="postgres://$POSTGRES_USER:$POSTGRES_PASSWORD@postgres:5432/$POSTGRES_DB?sslmode=disable" \
  golang:1.26-alpine go run "./$rel" /src/deploy/.test-license.env

{
  echo "# --- test-license managed block (24h test licence, issue #109). Remove to revoke. ---"
  cat "$ROOT/deploy/.test-license.env"
  echo "# --- end test-license block ---"
} >> "$ENV_FILE"
echo "test-license: installed through the product importer; verification keys recorded in $ENV_FILE."
