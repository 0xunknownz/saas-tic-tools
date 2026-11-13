# Architecture

- `apps/api` — Go API. Backend capabilities are plugins described by `plugin.yaml`; `go tool saasctl generate`
  validates the set and writes static wiring to `internal/bootstrap/generated/` (never edit by hand).
- `apps/admin` — Vite + React Admin.
- `apps/web` — TanStack Start Product Web.
- `config/product.yaml` — product identity and Foundation module selection (schema-validated).

File ownership: `internal/bootstrap/generated/**` belongs to saasctl. Foundation modules are Go packages
in the pinned `github.com/0xunknownz/saas.starter/foundation/...` module; the Product does not carry their
source. Everything under `internal/business/**` is yours: add a directory with a `plugin.yaml` and run
`go tool saasctl generate`.
