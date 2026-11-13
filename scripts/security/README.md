# Security test suite fixture

The Foundation security test suite (technical design 17.10) generates its cases from
`apps/api/openapi/*.json` plus the Foundation contracts embedded in the CLI and runs them against a real
API. `apps/api/openapi/` holds the Product's own `product-*.json`; the Foundation contracts come from the
pinned module. This directory is the Product's
half of the configuration.

## Split of responsibilities

- **Foundation** builds the identities and one-time credentials: the platform super
  administrator through the bootstrap flow, a platform account without a role, two tenants
  with their administrators, member and extra accounts, an expired session per realm, and the
  restricted sessions. `go tool saasctl security suite` needs `DATABASE_URL` and `SECRETS_KEY` for
  that, exactly like the API.
- **The Product** provides seed data and the legal request for each operation. A typical
  entry is two to four lines:

  ```json
  "getDoc": { "method": "GET", "path": "/api/tenant/v1/docs/${docId}" }
  ```

  `seeds` create what the requests refer to (a file, an invitation, a tenant) and capture
  response fields into `${variables}`. Foundation also exposes `${platformEmail}`,
  `${tenantAId}`, `${tenantAExtra0AccountId}`, `${run}` (a per-run suffix),
  `${totp:platformTOTP}` and the other subjects it built.

## What the suite checks per operation

It runs the legal request first. Only when it succeeds does it run the negative matrix:
unauthenticated, wrong realm, missing permission, expired session, cross-tenant access for
tenant-owned resources (including file downloads) and, for cookie-authenticated writes, a
cross-origin request that must be refused. A failed legal request is reported `INVALID` and no
negative case runs, so a wrong fixture can never look like "access denied". An operation
without `x-permission`/`x-tenant-scoped` (or without `x-entry` when `security: []`) fails.

## Add a Product business API without changing the suite

1. Publish every Product operation in `apps/api/openapi/*.json` with a globally unique
   `operationId`, explicit `security` (`[]` only for public entry), boolean
   `x-tenant-scoped`, and `x-permission` for protected operations. Use exactly one
   `adminSession` or `tenantSession` requirement; tenant-owned resources use
   `tenantSession` plus `x-tenant-scoped: true`. A missing/null declaration,
   ambiguous realms, or a duplicate operation ID fails the suite.
2. Add an `operations` entry under the same ID here, with a legal request against
   a *real* Product resource. For list, detail and edit, the shape is:

   ```json
   "listProductTestItems": { "method": "GET", "path": "/api/product-admin/v1/test-items" },
   "getProductTestItem": { "method": "GET", "path": "/api/product-admin/v1/test-items/ti-001" },
   "updateProductTestItem": {
     "method": "PUT",
     "path": "/api/product-admin/v1/test-items/ti-004",
     "json": {"name": "Edited", "category": "Environmental", "method": "Gravimetric",
              "description": "Suite edit", "expectedRevision": 1}
   }
   ```

   The example dictionary is a platform-wide catalog whose migration seeds `ti-001`
   through `ti-004`. Its edit request uses `ti-004` with the seeded `revision: 1`, so
   repeated runs keep a stable legal baseline; incrementing `revision` on each write
   means a fixture should target its own row rather than a shared one. A Product with
   its own edit operation adds the OpenAPI declaration and a legal body with every
   required field. Create dynamic IDs with a seed and `capture` from `body.id` first,
   and avoid reusing a destructive baseline resource when later cases need it.
3. For a tenant-owned detail/edit resource, seed it with `tenantA`, capture its ID,
   and use that same ID with `tenantB` for the suite cross-tenant probe. For a list,
   add an addressable tenant path if the API accepts one; otherwise the collection
   is scoped by the session and cross-tenant is `NOT_APPLICABLE` with a reason.
   Platform-wide catalog operations use `x-tenant-scoped: false` and have no
   cross-tenant case. All protected operations still run unauthenticated, wrong-realm,
   missing-permission and expired-session cases; writes also run CSRF.
4. Run `sh scripts/security/run.sh` against the Product API and inspect each new
   operation in `scripts/security/report.json`. Any non-2xx legal baseline is `INVALID`
   and prevents negative probes, including when the fixture points at a missing ID.
   The script exits nonzero on `FAIL` or `INVALID`; only a passing report may be
   fed to `go tool saasctl build --security-result scripts/security/report.json`. The
   resulting `release/manifest.json` stores the suite result and OpenAPI contract
   digest in `security`. CI must run this in the actual Product against its own API.

## Cases that do not apply

`notApplicable` and `skip` record a reason; the report keeps them visible and the run still
passes. Never delete a case silently: AC-026 asks for the reason.

## Module set

The fixture covers the Foundation modules this Product enables (tenant, storage, notification,
jobs). If you disable a module, its endpoints are not mounted: mark them `skip` with that
reason, or keep the profile the fixture was written for.

## Running it

```bash
export DATABASE_URL=... SECRETS_KEY=...
go tool saasctl db migrate --yes
(cd apps/api && go run ./cmd/api &)
sh scripts/security/run.sh
```

The report lands in `scripts/security/report.json`. Pass it to the Release Manifest with
`go tool saasctl build --security-result scripts/security/report.json`.
