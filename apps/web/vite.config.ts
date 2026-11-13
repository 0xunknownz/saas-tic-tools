import { defineConfig } from 'vite'
import { tanstackStart } from '@tanstack/react-start/plugin/vite'
import viteReact from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// Ports come from `go tool saasctl dev up` (config/development.yaml); the defaults match that file.
const api = `http://localhost:${process.env.API_PORT ?? 18080}`

export default defineConfig({
  server: {
    port: Number(process.env.WEB_PORT ?? 13000),
    strictPort: true,
    // changeOrigin stays false so the API sees the browser's Host when it checks Origin (CSRF, 11.3).
    proxy: Object.fromEntries(['/api', '/healthz', '/readyz'].map((p) => [p, { target: api, changeOrigin: false }])),
  },
  plugins: [tanstackStart(), viteReact(), tailwindcss()],
})
