# Private Compose deployment candidate

This is an M5 work in progress, not yet an installable release package. It uses preloaded API/worker/migration, Web, Admin edge and PostgreSQL images; `pull_policy: never` prevents Compose fetching images. The one-shot migration service mounts the verified release migration plan read-only, overriding any copy in the API image. Before deployment, verify all images, digests, SBOM, signature and platform against the release manifest and validate the public key via an independent trusted channel.

Provision an HTTPS reverse proxy on the same host pointing to `127.0.0.1:8080`. Only that trusted proxy may expose the edge; the supplied private nginx config tells upstreams the original scheme is HTTPS. Configure the proxy to reject untrusted hosts and overwrite forwarded headers. Supply a private S3-compatible service reachable by API and worker; no object storage server is included here. Disable providers requiring the Internet for an offline installation.

Set required Compose variables using a protected local environment or secret manager; `POSTGRES_PASSWORD_FILE` must point to a protected file. `DATABASE_URL` must use the same database credentials and point at the Compose service `postgres:5432`; URL-encode special characters. API configuration currently consumes `DATABASE_URL`, `SECRETS_KEY` and S3 credentials as environment variables, so access to Docker metadata and the environment must be restricted. `WEB_PUBLIC_URL` and `ADMIN_PUBLIC_URL` must be HTTPS URLs; the latter ends in `/admin`.

The migration container must finish successfully before the API or worker starts. Persistent PostgreSQL data is held in a named volume; the external object store has its own backup and retention obligations. Bootstrap administration uses `go tool saasctl admin bootstrap issue`. External TLS termination, protected configuration injection and object-store backups remain operator responsibilities. Verify and load the signed images before starting this stack; the customer host must not build or pull images.

## Database backup and restore

Stop API and worker, prevent all external writes and migrations, and export the same Compose configuration variables used for the deployment. Set `POSTGRES_USER` and `POSTGRES_DB` to the database owner and source database; ensure `release/manifest.json` is the manifest for the deployed release. Then run:

```sh
sh deploy/private/backup.sh /secure/backups/new-backup-directory
```

The new directory contains a PostgreSQL custom dump, the release manifest, ordered migration history and SHA-256 checksums. The operator must separately back up object storage, deployment configuration, encryption keys and public key material using their own secure procedures. Store the backup and keys in protected, independent locations. Checksums detect accidental changes; they do not authenticate an untrusted backup.

To restore, prepare a **new empty database** on a compatible PostgreSQL server, stop API and worker, block other writers, export its name as `POSTGRES_DB` and run:

```sh
sh deploy/private/restore.sh /secure/backups/new-backup-directory --confirm-empty-target
```

The script verifies checksums before restoring in one transaction and compares migration records afterward. Before starting the application, restore the matching object store and protected configuration, verify the backed-up manifest against the intended image and module versions, and run the documented application readiness checks. A failed post-restore comparison requires investigation; do not start the application. Rolling back application images does not undo database migrations. Use the compatible upgrade and restore procedure below.


## Environment and version checks before an upgrade

Keep the installed and candidate release bundles in separate directories. Independently verify the release public key, then run `go tool saasctl release verify --bundle TARGET --verification-key TRUSTED_KEY` and `go tool saasctl release load` for the target. Keep the installed images available for rollback.

Before stopping service, check:

- Docker Engine and Compose are installed and the daemon is reachable (`docker version`, `docker compose version`). The host CPU/OS and the loaded image platform must match `build.targetPlatform` in each manifest. Use `docker image inspect IMAGE` to check platform and image ID.
- PostgreSQL major version and the backup tools are compatible. This template uses PostgreSQL 17. Check available disk space for both the database and backup (`df -h`) and ensure backup files are readable only by authorized operators.
- The HTTPS reverse proxy, service names, ports, mounted plan, protected environment, encryption key and object-store configuration refer to the intended installation. Do not print secrets while inspecting Docker metadata.
- Record `product.id`, `product.version`, `product.gitCommit`, `foundation.version`, `composition.digest` and each `composition.plugins[].id/version/contractVersion` from the installed and candidate `release/manifest.json`. These are release and module versions; an image tag alone is insufficient evidence of provenance.
- Compare the applied migration rows to the installed plan. Run this with the same protected Compose environment used for deployment:

```sh
docker compose -f deploy/compose/private.yaml exec -T postgres \
  psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
  -c 'SELECT module_id, migration_version, checksum FROM saasctl_migrations ORDER BY module_id, migration_version'
```

Never edit previously applied SQL or its checksums to make a comparison pass. Inspect migration differences before starting:

```sh
go tool saasctl release upgrade-plan \
  --from-manifest INSTALLED/release/manifest.json \
  --from-plan INSTALLED/release/migrations/plan.json \
  --to-manifest TARGET/release/manifest.json \
  --to-plan TARGET/release/migrations/plan.json \
  --out upgrade-plan.json
```

The report checks product, version ordering, platform and migration changes; it does not prove business-level compatibility. A compatible result still requires review of the target's release notes and data changes.

## Compatible upgrade and failure rollback

1. Stop API and worker, block external writes and concurrent migration jobs. Run the backup script above with `RELEASE_MANIFEST` pointing to the **installed** manifest. Preserve the backup's `manifest.json`, `migrations.txt` and `SHA256SUMS` alongside the dump. Back up object data and protected settings separately.
2. Switch to the verified target bundle and its protected Compose environment. The migration service uses the bundle's `release/migrations/plan.json` read-only; keep the plan, manifest and image versions together.
3. Run `docker compose -f deploy/compose/private.yaml up -d --pull never`. The migration service must exit successfully before API/worker start. If it fails, do not bypass the dependency gate. Inspect migration logs and the applied history.
4. After an API container is recreated, restart Edge so Nginx refreshes its API upstream address: `docker compose -f deploy/compose/private.yaml restart edge`. Check `/readyz` through both API and Edge, then verify Web/Admin asset loading and a representative data read. Compare the migration rows with the target plan and record the actual running image IDs against the target manifest.
5. If migration or application validation fails, stop API/worker and prevent all writes. Preserve the failed database for investigation. Provision a **new empty database**; do not erase production data volumes or attempt ad hoc down migrations.
6. Point the protected Compose database setting at that empty target and run `restore.sh BACKUP --confirm-empty-target` as above. Restore matching object data and protected configuration. The script must verify backup checksums and exact migration-history equality before proceeding.
7. Select the previous verified bundle, its migration plan, image versions and preserved encryption keys. Start the previous stack with `--pull never`; check its API/Edge readiness, retained data, module versions and migration history. Only then restore external traffic.

If using extra Compose override files, first render the exact resolved configuration with `docker compose ... config > /secure/resolved-compose.yaml`, restrict its permissions (it contains secrets), and pass its absolute path through `PRIVATE_COMPOSE_FILE` to both backup and restore scripts. Keep this rendered file outside release inventories and source control.
