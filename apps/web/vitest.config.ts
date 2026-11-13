import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

// The web tests run against the pages and headless hooks only: no router and no server are started.
// jsdom supplies the DOM; the fetch client is replaced by an in-memory mock in every test.
export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    include: ['tests/**/*.test.ts', 'tests/**/*.test.tsx'],
    setupFiles: ['tests/setup.ts'],
  },
})
