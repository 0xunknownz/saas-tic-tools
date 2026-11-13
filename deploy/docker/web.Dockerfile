# syntax=docker/dockerfile:1
# Test-environment Web (TanStack Start SSR). A single stage keeps pnpm symlinks intact; this image
# is only used for the pilot test environment, not for production packaging.
FROM node:22-bookworm-slim
RUN corepack enable && corepack prepare pnpm@10.34.9 --activate
ENV NODE_ENV=production \
    PORT=3000 \
    HOST=0.0.0.0
WORKDIR /src
COPY package.json pnpm-workspace.yaml ./
COPY apps/web/package.json apps/web/package.json
COPY apps/admin/package.json apps/admin/package.json
COPY packages ./packages
COPY apps/web ./apps/web
# foundation-client's postinstall regenerates its typed client from the OpenAPI contracts, so the
# generator script and the contract JSON must exist in the image build context.
COPY scripts ./scripts
COPY apps/api/openapi ./apps/api/openapi
COPY .saasctl/openapi ./apps/api/openapi
RUN --mount=type=secret,id=npm_token,required=false \
    if [ -s /run/secrets/npm_token ]; then printf '@0xunknownz:registry=https://npm.pkg.github.com\n//npm.pkg.github.com/:_authToken=%s\n' "$(cat /run/secrets/npm_token)" > /root/.npmrc; fi; \
    pnpm install --no-frozen-lockfile && pnpm --filter ./apps/web build; status=$?; rm -f /root/.npmrc; exit $status
WORKDIR /src/apps/web
EXPOSE 3000
HEALTHCHECK --interval=15s --timeout=5s --start-period=20s --retries=5 CMD node -e "fetch('http://127.0.0.1:3000/').then(r=>process.exit(r.ok?0:1)).catch(()=>process.exit(1))"
CMD ["pnpm", "exec", "vite", "preview", "--host", "0.0.0.0", "--port", "3000"]
