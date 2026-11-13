# Kubernetes deployment chart (M6 draft)

This chart is a configurable starting point for a generated Product. It does not establish a supported customer Kubernetes version, storage class, network policy, resource sizing or air-gap inventory. Those values must come from the customer baseline and be checked by the shared Preflight before installation. This chart is not an accepted M6 deployment.

The chart scaffold contains API, worker, Web, edge and a single PostgreSQL StatefulSet. API and worker init containers wait for PostgreSQL readiness and then run the release migration plan before starting; the migration runner's database advisory lock serializes concurrent pod starts. The Chart supports initial installation by default; Helm upgrades are refused unless upgrade.allow=true. Enabling it requires the operator to drain API/worker, take and verify a database backup, run the upgrade, and restore the backup on failure; per-pod init containers do not make unattended rolling schema upgrades safe. Generated Charts pass Helm 3.17.3 strict lint and template rendering with synthetic inputs. A local kind v1.32.2/arm64 rehearsal exercised migration failure gating, recovery, admin MFA/login, tenant management and database PVC retention. This does not establish support for a customer environment or replace the shared Preflight and customer installation acceptance. Production database HA, backup/restore, ingress/TLS, NetworkPolicy, resource sizing, security-context hardening and storage-specific behavior also require review against the frozen customer environment.

## Required values

Set API, Web, Edge and PostgreSQL image references to immutable digests from the release manifest; Worker defaults to the API image reference. Provide pre-created Kubernetes Secret names:

- secrets.app must contain DATABASE_URL, SECRETS_KEY, and required S3 credentials.
- secrets.postgres must contain POSTGRES_PASSWORD.

Never put secret values in values files, source control or rendered reports. Supply public URLs, S3 endpoint/region/bucket and notification settings in a protected environment-specific values file. Configure the PostgreSQL storage class and capacity from the approved customer baseline.

Only documented credential keys are read from the app Secret; extra keys such as APP_ENV, HTTP_ADDR or public URLs cannot override chart-owned configuration. Migration plans are content-addressed immutable ConfigMaps retained by Helm; remove an old plan only after no pods refer to it. Use one Product release per namespace. PostgreSQL inherits imagePullSecrets and global scheduling settings.

Set release.manifest to the JSON content of the signed Release Manifest so operations can read the deployed Product/Foundation versions and migration pin from the release-manifest ConfigMap. The manifest contains no secret values.

This chart does not generate an offline bundle or perform Preflight. Do not mark #79 or #81 accepted based on chart presence or rendering alone.
