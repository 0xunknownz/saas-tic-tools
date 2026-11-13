# Development

Requirements: Go 1.24+, Node.js 20+, pnpm, Docker.

```bash
go tool saasctl doctor      # verify the toolchain
go tool saasctl dev up      # starts PostgreSQL (Docker), Go API, Admin and Web; Ctrl-C stops everything
```

Default addresses (edit `config/development.yaml`):

| Service | URL |
|---|---|
| API health | http://localhost:18080/healthz, readiness http://localhost:18080/readyz |
| Admin | http://localhost:15173 |
| Web | http://localhost:13000 |
| Tenant admin | http://localhost:13001/console/ |
| PostgreSQL | localhost:15432 (password in `.saasctl/dev.env`, generated on first run) |

## License in development

When the composition includes the `license` module, `dev up` installs a locally signed development License before starting the API (valid for 30 days, renewed by the next `dev up`) and prints its expiry. Without it the fresh database stays in the `missing` state and D-011 blocks writes such as creating a tenant (`403 LICENSE_RESTRICTED`). The development signing key is generated once into `.saasctl/dev-license-key.pem` (0600, git-ignored) and is not customer signing material. If you manage licensing yourself, set `LICENSE_VERIFICATION_KEYS`, `LICENSE_INSTANCE_ID` and `LICENSE_PRODUCT_VERSION`; `dev up` then leaves the license state alone.

## Adding a backend plugin

1. Create `apps/api/internal/business/<name>/` with `plugin.yaml` (see `business/example`), a `Config`, `Deps`,
   `NewService` and optionally `RegisterHTTP` / `NewWorker`.
2. Run `go tool saasctl generate`. Errors name the plugin, file and field. Run `go tool saasctl generate --check` (also part of `go tool saasctl doctor`) to detect drift.

## Tests

```bash
cd apps/api && go test ./...
pnpm install && pnpm build && pnpm typecheck   # build first: it writes the Web route tree
```
