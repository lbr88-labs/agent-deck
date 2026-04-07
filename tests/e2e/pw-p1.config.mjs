// Shared Playwright config for ALL Phase 3 P1 layout-bug regression specs
// (visual/p1-bug*.spec.ts).
//
// The default playwright.config.ts auto-spawns its own webServer via
// `go run ../../cmd/agent-deck --web --port 19999`, which (a) uses a flag
// the binary doesn't accept and (b) falls through to the TUI launch path
// that fails inside an agent-deck nested tmux session. This config points
// at a manually-managed test server (cmd/agent-deck-test-server) on port
// 18420 and has no webServer block, mirroring the Phase 2 standalone
// configs (pw-bug2.config.mjs / pw-p0-bug3.config.mjs / pw-p0-bug1.config.mjs).
//
// Start the server with:
//   nohup ./build/agent-deck-test-server -listen 127.0.0.1:18420 -profile _test \
//     > /tmp/web-p3.log 2>&1 &
//
// Rebuild the server after editing any internal/web/static/app/* JS file so
// the new go:embed bundle is served:
//   pkill -f agent-deck-test-server
//   go build -o build/agent-deck-test-server ./cmd/agent-deck-test-server
//   nohup ./build/agent-deck-test-server -listen 127.0.0.1:18420 -profile _test \
//     > /tmp/web-p3.log 2>&1 &

import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './visual',
  testMatch: 'p1-bug*.spec.ts',
  timeout: 60000,
  retries: 0,
  use: {
    baseURL: 'http://127.0.0.1:18420',
    headless: true,
    viewport: { width: 1280, height: 800 },
  },
  projects: [
    { name: 'chromium', use: { browserName: 'chromium' } },
  ],
  // No webServer block — the server is started manually before specs run.
})
