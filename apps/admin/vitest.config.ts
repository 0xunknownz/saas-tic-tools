import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

// Foundation component and shell tests live in @0xunknownz/foundation-platform-ui; this app carries
// the entry and Product modules, which get their own tests when they grow.
export default defineConfig({
  plugins: [react()],
  test: { environment: 'jsdom', include: ['src/**/*.test.{ts,tsx}'] },
})
