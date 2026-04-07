// Standalone Playwright config for cascade-baseline.spec.ts.
// Used by Phase 1 / Plan 02 to capture the pre-Tailwind-precompile baseline.
// Connects to a manually-started agent-deck web server on port 18420 so the
// test does not race the default playwright.config.ts webServer (port 19999).
import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: '.',
  testMatch: 'cascade-baseline.spec.ts',
  timeout: 60000,
  retries: 0,
  use: {
    baseURL: 'http://127.0.0.1:18420',
    headless: true,
    viewport: { width: 1280, height: 800 },
    extraHTTPHeaders: {
      // Token for any future authenticated routes; the page itself uses ?t=test on goto().
      Authorization: 'Bearer test',
    },
  },
  projects: [
    { name: 'chromium', use: { browserName: 'chromium' } },
  ],
  // No webServer block — server is started manually before this spec runs.
})
