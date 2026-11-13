import { defineConfig } from 'vite'
import { tanstackStart } from '@tanstack/react-start/plugin/vite'
import viteReact from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// Ports come from `go tool saasctl dev up` (config/development.yaml); the defaults match that file.
const api = `http://localhost:${process.env.API_PORT ?? 18080}`
const tenantAdmin = `http://localhost:${process.env.TENANT_ADMIN_PORT ?? 13001}`
const platformAdmin = `http://localhost:${process.env.ADMIN_PORT ?? 15173}`

export default defineConfig({
  server: {
    port: Number(process.env.WEB_PORT ?? 13000),
    strictPort: true,
    // changeOrigin stays false so the API sees the browser's Host when it checks Origin (CSRF, 11.3).
    proxy: {
      ...Object.fromEntries(['/api', '/healthz', '/readyz'].map((p) => [p, { target: api, changeOrigin: false }])),
      '/console': { target: tenantAdmin, changeOrigin: false, ws: true },
    },
  },
  plugins: [
    {
      name: 'tic-dev-platform-entry',
      // Platform dev assets use the root base, so open its existing server directly.
      // Deployment continues to use the edge's /admin/ route on the same host.
      configureServer(server) {
        server.middlewares.use((request, response, next) => {
          if (/^\/admin(?:\/|\?|$)/.test(request.url ?? '')) {
            response.writeHead(307, { Location: platformAdmin + request.url })
            response.end()
            return
          }
          next()
        })
      },
    },
    tanstackStart(), viteReact(), tailwindcss(),
  ],
})
