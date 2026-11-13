# syntax=docker/dockerfile:1
# Private edge: serves the Admin SPA under /admin/ and proxies API and Web.
# Built from the repository root so the Admin workspace dependencies resolve.
FROM node:22-alpine AS admin
WORKDIR /app
RUN corepack enable
COPY package.json pnpm-workspace.yaml ./
COPY apps/admin ./apps/admin
COPY apps/tenant-admin ./apps/tenant-admin
COPY apps/api/openapi ./apps/api/openapi
COPY .saasctl/openapi ./apps/api/openapi
COPY scripts ./scripts
COPY packages ./packages
RUN --mount=type=secret,id=npm_token,required=false \
    if [ -s /run/secrets/npm_token ]; then printf '@0xunknownz:registry=https://npm.pkg.github.com\n//npm.pkg.github.com/:_authToken=%s\n' "$(cat /run/secrets/npm_token)" > /root/.npmrc; fi; \
    pnpm install --no-frozen-lockfile; status=$?; rm -f /root/.npmrc; exit $status
RUN pnpm --filter ./apps/admin exec vite build --base=/admin/
RUN pnpm --filter ./apps/tenant-admin exec vite build --base=/console/

FROM nginx:1.27-alpine
COPY --from=admin /app/apps/admin/dist/ /usr/share/nginx/html/admin/
COPY --from=admin /app/apps/tenant-admin/dist/ /usr/share/nginx/html/console/
COPY deploy/private/nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 8080
