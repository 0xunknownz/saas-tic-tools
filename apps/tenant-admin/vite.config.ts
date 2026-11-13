import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// tenant-admin is served under /console/ on the same host as web (design 4.1): the tenant session
// cookie is __Host-tenant_session, so a separate host would not share it.
const api = `http://localhost:${process.env.API_PORT ?? 18080}`

export default defineConfig({
  base: '/console/',
  server: {
    port: Number(process.env.TENANT_ADMIN_PORT ?? 13001),
    strictPort: true,
    proxy: Object.fromEntries(['/api', '/healthz', '/readyz'].map((p) => [p, { target: api, changeOrigin: false }])),
  },
  plugins: [react(), tailwindcss()],
})
