import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import adminConfig from './admin.config.ts'
import { assertValidAdminApp } from '@0xunknownz/foundation-platform-ui'

// Admin Module Contract v1 is validated when the build or dev server starts: duplicate ids, a menu that
// points at a missing page, a Product module using Foundation namespaces, an incompatible Shell range or
// an invalid icon/page reference fails the build instead of shipping a half-registered console
// (PRD 5.4, 5.10; technical design 7.4). The test suite exercises the same validator.
assertValidAdminApp(adminConfig)

// The Admin SPA is served at the site root in development and may be mounted under a path
// (for example /admin/) in a deployment: build with `vite build --base=/admin/`. Asset URLs follow
// the base automatically; the router keeps the full /admin/... paths declared by the contract, so no
// source change is needed for a sub-path deployment.

// Ports come from `go tool saasctl dev up` (config/development.yaml); the defaults match that file.
const api = `http://localhost:${process.env.API_PORT ?? 18080}`

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: Number(process.env.ADMIN_PORT ?? 15173),
    strictPort: true,
    // changeOrigin stays false: the API compares the browser's Origin with the Host it receives (CSRF, 11.3),
    // and the dev proxy must not rewrite Host to the API's address.
    proxy: Object.fromEntries(['/api', '/healthz', '/readyz'].map((p) => [p, { target: api, changeOrigin: false }])),
  },
})