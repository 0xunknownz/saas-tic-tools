# syntax=docker/dockerfile:1
# Test-environment edge: the built Admin SPA under /admin/ plus a reverse proxy for the API and Web.
FROM node:22-bookworm-slim AS build
RUN corepack enable && corepack prepare pnpm@10.34.9 --activate
WORKDIR /src
COPY package.json pnpm-workspace.yaml ./
COPY apps/admin/package.json apps/admin/package.json
COPY apps/web/package.json apps/web/package.json
COPY apps/tenant-admin/package.json apps/tenant-admin/package.json
COPY packages ./packages
COPY apps/admin ./apps/admin
COPY apps/tenant-admin ./apps/tenant-admin
# foundation-client's postinstall regenerates its typed client from the OpenAPI contracts, so the
# generator script and the contract JSON must exist in the image build context.
COPY scripts ./scripts
COPY apps/api/openapi ./apps/api/openapi
COPY .saasctl/openapi ./apps/api/openapi
RUN --mount=type=secret,id=npm_token,required=false \
    if [ -s /run/secrets/npm_token ]; then printf '@0xunknownz:registry=https://npm.pkg.github.com\n//npm.pkg.github.com/:_authToken=%s\n' "$(cat /run/secrets/npm_token)" > /root/.npmrc; fi; \
    pnpm install --no-frozen-lockfile; status=$?; rm -f /root/.npmrc; exit $status
# Serve the SPA below /admin/ so ADMIN_PUBLIC_URL has the documented shape.
RUN cd apps/admin && pnpm exec vite build --base=/admin/
# tenant-admin shares the web host and lives under /console/ (design 4.1); the __Host- cookie forbids
# a separate hostname.
RUN cd apps/tenant-admin && pnpm exec vite build --base=/console/

FROM nginx:1.27-alpine
COPY deploy/docker/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /src/apps/admin/dist /usr/share/nginx/html/admin
COPY --from=build /src/apps/tenant-admin/dist /usr/share/nginx/html/console
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=5 CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null || exit 1
