// Standalone Playwright config for cascade-verify.spec.ts.
// Used by Phase 1 / Plan 03 to diff post-swap computed styles against the
// pre-swap baseline captured by plan 02.
//
// Connects to a manually-started agent-deck web server on port 18420 so the
// test does not race the default playwright.config.ts webServer (port 19999)
// or the plan 02 baseline-capture config.
import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: '.',
  testMatch: 'cascade-verify.spec.ts',
  timeout: 60000,
  retries: 0,
  use: {
    baseURL: 'http://127.0.0.1:18420',
    headless: true,
    viewport: { width: 1280, height: 800 },
    extraHTTPHeaders: {
      Authorization: 'Bearer test',
    },
  },
  projects: [
    { name: 'chromium', use: { browserName: 'chromium' } },
  ],
  // No webServer block — server is started manually before this spec runs.
})
