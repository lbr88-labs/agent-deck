// Phase 9 / Plan 04: POL-6 light theme audit (axe-core powered).
//
// Runs @axe-core/playwright with `runOnly: ['color-contrast']` against every
// light-theme surface shipped in v1.5.0:
//   - Main shell (empty state)
//   - Sidebar with fixture sessions across all statuses
//   - Multi-profile dropdown open
//   - CostDashboard tab
//   - EmptyStateDashboard
//   - CreateSessionDialog open
//   - ConfirmDialog open
//   - SettingsPanel open
//   - KeyboardShortcutsOverlay open
//   - ToastHistoryDrawer open with fixture history
//   - Toast variants (info + success + error)
//
// Theme is forced to LIGHT via two mechanisms (see pw-p9-plan4.config.mjs):
//   1. colorScheme: 'light' context option
//   2. addInitScript setting localStorage['theme'] = 'light' before SPA boot
//
// Each test asserts `results.violations.length === 0` with the full
// violations array dumped in the failure message so the fix task has a
// complete target list.
import { test, expect } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';

const FIXTURE_MENU = {
  items: [
    { type: 'group', level: 0, group: { path: 'work', name: 'Work', expanded: true, sessionCount: 2 } },
    { type: 'session', level: 1, session: { id: 's1', title: 'Build pipeline', status: 'running', tool: 'claude', groupPath: 'work' } },
    { type: 'session', level: 1, session: { id: 's2', title: 'Research docs', status: 'waiting', tool: 'shell', groupPath: 'work' } },
    { type: 'group', level: 0, group: { path: 'personal', name: 'Personal', expanded: true, sessionCount: 2 } },
    { type: 'session', level: 1, session: { id: 's3', title: 'Blog drafts', status: 'idle', tool: 'claude', groupPath: 'personal' } },
    { type: 'session', level: 1, session: { id: 's4', title: 'Errored task', status: 'error', tool: 'shell', groupPath: 'personal' } },
  ],
};

const EMPTY_MENU = { items: [] };

const FIXTURE_COSTS_SUMMARY = {
  today_usd: 12.34, today_events: 5,
  week_usd: 67.89, week_events: 42,
  month_usd: 234.56, month_events: 200,
  projected_usd: 500.00,
};

const FIXTURE_COSTS_DAILY = [
  { date: '2026-04-03', cost_usd: 5.01 },
  { date: '2026-04-04', cost_usd: 7.12 },
  { date: '2026-04-05', cost_usd: 9.44 },
  { date: '2026-04-06', cost_usd: 3.33 },
  { date: '2026-04-07', cost_usd: 6.78 },
  { date: '2026-04-08', cost_usd: 8.01 },
  { date: '2026-04-09', cost_usd: 12.34 },
];

const FIXTURE_COSTS_MODELS = {
  'claude-opus-4': 120.5,
  'claude-sonnet-4': 84.2,
  'gpt-4o': 30.0,
};

const FIXTURE_PROFILES = {
  current: 'default',
  profiles: ['default', 'work', 'personal', 'research', 'client-a', 'client-b', 'archived', 'staging', '_test', '_dev'],
};

const FIXTURE_SETTINGS = { webMutations: true };

async function forceLight(page) {
  await page.addInitScript(() => {
    localStorage.setItem('theme', 'light');
  });
}

async function mockEndpoints(page, opts: { menu?: any } = {}) {
  const menu = opts.menu || FIXTURE_MENU;
  await page.route('**/api/menu*', r => r.fulfill({ json: menu }));
  await page.route('**/api/costs/summary*', r => r.fulfill({ json: FIXTURE_COSTS_SUMMARY }));
  await page.route('**/api/costs/daily*', r => r.fulfill({ json: FIXTURE_COSTS_DAILY }));
  await page.route('**/api/costs/models*', r => r.fulfill({ json: FIXTURE_COSTS_MODELS }));
  await page.route('**/api/profiles*', r => r.fulfill({ json: FIXTURE_PROFILES }));
  await page.route('**/api/settings*', r => r.fulfill({ json: FIXTURE_SETTINGS }));
  // SSE stream keeps the connection open indefinitely; abort it so
  // waitForLoadState('domcontentloaded') and our header/ready probes can
  // settle. The app handles `EventSource` error events gracefully.
  await page.route('**/events/menu*', r => r.abort());
}

async function waitForAppReady(page) {
  // Wait for Topbar to mount — this is the load-bearing signal that the
  // Preact SPA has bootstrapped and applied the theme class to <html>.
  await page.waitForSelector('header', { state: 'attached', timeout: 15000 });
  await page.waitForTimeout(150);
}

async function assertLightTheme(page) {
  const isDark = await page.evaluate(() => document.documentElement.classList.contains('dark'));
  expect(isDark, 'expected document.documentElement to NOT have the dark class').toBe(false);
}

function summarizeViolations(violations: any[]) {
  return JSON.stringify(
    violations.map(v => ({
      id: v.id,
      impact: v.impact,
      help: v.help,
      nodes: v.nodes.map((n: any) => ({
        target: n.target,
        failureSummary: n.failureSummary,
      })),
    })),
    null,
    2,
  );
}

test.describe('POL-6 light theme audit (axe-core color-contrast)', () => {
  test.beforeEach(async ({ page }) => {
    await forceLight(page);
    await mockEndpoints(page);
  });

  test('T1 main shell empty state — zero color-contrast violations', async ({ page }) => {
    await page.route('**/api/menu*', r => r.fulfill({ json: EMPTY_MENU }));
    await page.goto('/?token=test');
    await waitForAppReady(page);
    await assertLightTheme(page);
    const results = await new AxeBuilder({ page }).options({ runOnly: ['color-contrast'] }).analyze();
    expect(results.violations, summarizeViolations(results.violations)).toHaveLength(0);
  });

  test('T2 sidebar with fixture sessions — zero color-contrast violations', async ({ page }) => {
    await page.goto('/?token=test');
    await waitForAppReady(page);
    await page.waitForSelector('#preact-session-list', { state: 'attached' });
    await assertLightTheme(page);
    const results = await new AxeBuilder({ page })
      .include('#preact-session-list')
      .options({ runOnly: ['color-contrast'] })
      .analyze();
    expect(results.violations, summarizeViolations(results.violations)).toHaveLength(0);
  });

  test('T3 multi-profile dropdown open — zero color-contrast violations', async ({ page }) => {
    await page.goto('/?token=test');
    await waitForAppReady(page);
    // Click the profile button to open the listbox
    const profileBtn = page.locator('[data-testid="profile-indicator"] button[aria-haspopup="listbox"]');
    await profileBtn.waitFor({ state: 'visible' });
    await profileBtn.click();
    await page.locator('[role="listbox"][aria-label="Available profiles (read-only)"]').waitFor({ state: 'visible' });
    await assertLightTheme(page);
    const results = await new AxeBuilder({ page })
      .include('[role="listbox"][aria-label="Available profiles (read-only)"]')
      .options({ runOnly: ['color-contrast'] })
      .analyze();
    expect(results.violations, summarizeViolations(results.violations)).toHaveLength(0);
  });

  test('T4 CostDashboard tab — zero color-contrast violations', async ({ page }) => {
    await page.goto('/?token=test');
    await waitForAppReady(page);
    // Switch to Costs tab via activeTabSignal
    await page.evaluate(async () => {
      const state: any = await import('/static/app/state.js');
      state.activeTabSignal.value = 'costs';
    });
    // Wait for summary cards to render
    await page.waitForSelector('text=Today', { state: 'visible', timeout: 5000 });
    await assertLightTheme(page);
    const results = await new AxeBuilder({ page })
      .include('main')
      .options({ runOnly: ['color-contrast'] })
      .analyze();
    expect(results.violations, summarizeViolations(results.violations)).toHaveLength(0);
  });

  test('T5 EmptyStateDashboard — zero color-contrast violations', async ({ page }) => {
    await page.route('**/api/menu*', r => r.fulfill({ json: EMPTY_MENU }));
    await page.goto('/?token=test');
    await waitForAppReady(page);
    await page.waitForSelector('[data-testid="empty-state-dashboard"]', { state: 'visible' });
    await assertLightTheme(page);
    const results = await new AxeBuilder({ page })
      .include('[data-testid="empty-state-dashboard"]')
      .options({ runOnly: ['color-contrast'] })
      .analyze();
    expect(results.violations, summarizeViolations(results.violations)).toHaveLength(0);
  });

  test('T6 CreateSessionDialog open — zero color-contrast violations', async ({ page }) => {
    await page.goto('/?token=test');
    await waitForAppReady(page);
    await page.evaluate(async () => {
      const state: any = await import('/static/app/state.js');
      state.createSessionDialogSignal.value = true;
    });
    await page.waitForSelector('dialog, [role="dialog"]', { state: 'visible', timeout: 5000 });
    await assertLightTheme(page);
    const results = await new AxeBuilder({ page })
      .include('[role="dialog"], dialog')
      .options({ runOnly: ['color-contrast'] })
      .analyze();
    expect(results.violations, summarizeViolations(results.violations)).toHaveLength(0);
  });

  test('T7 ConfirmDialog open — zero color-contrast violations', async ({ page }) => {
    await page.goto('/?token=test');
    await waitForAppReady(page);
    await page.evaluate(async () => {
      const state: any = await import('/static/app/state.js');
      state.confirmDialogSignal.value = {
        message: 'Delete session "Build pipeline"? This cannot be undone.',
        onConfirm: () => {},
      };
    });
    await page.waitForSelector('[role="dialog"], dialog', { state: 'visible', timeout: 5000 });
    await assertLightTheme(page);
    const results = await new AxeBuilder({ page })
      .include('[role="dialog"], dialog')
      .options({ runOnly: ['color-contrast'] })
      .analyze();
    expect(results.violations, summarizeViolations(results.violations)).toHaveLength(0);
  });

  test('T8 GroupNameDialog open — zero color-contrast violations', async ({ page }) => {
    await page.goto('/?token=test');
    await waitForAppReady(page);
    await page.evaluate(async () => {
      const state: any = await import('/static/app/state.js');
      state.groupNameDialogSignal.value = {
        mode: 'rename',
        groupPath: 'work',
        currentName: 'Work',
        onSubmit: null,
      };
    });
    await page.waitForSelector('[role="dialog"], dialog', { state: 'visible', timeout: 5000 });
    await assertLightTheme(page);
    const results = await new AxeBuilder({ page })
      .include('[role="dialog"], dialog')
      .options({ runOnly: ['color-contrast'] })
      .analyze();
    expect(results.violations, summarizeViolations(results.violations)).toHaveLength(0);
  });

  test('T9 KeyboardShortcutsOverlay — zero color-contrast violations', async ({ page }) => {
    await page.goto('/?token=test');
    await waitForAppReady(page);
    // Open via `?` key — the standard hotkey
    await page.keyboard.press('?');
    // Fall back to direct signal manipulation if the key doesn't bind
    await page.evaluate(async () => {
      const state: any = await import('/static/app/state.js');
      if (state.shortcutsOverlaySignal) {
        state.shortcutsOverlaySignal.value = true;
      }
    });
    await page.waitForTimeout(250);
    await assertLightTheme(page);
    // Scope to any overlay / dialog
    const results = await new AxeBuilder({ page })
      .options({ runOnly: ['color-contrast'] })
      .analyze();
    expect(results.violations, summarizeViolations(results.violations)).toHaveLength(0);
  });

  test('T10 ToastHistoryDrawer open — zero color-contrast violations', async ({ page }) => {
    await page.goto('/?token=test');
    await waitForAppReady(page);
    await page.evaluate(async () => {
      const state: any = await import('/static/app/state.js');
      state.toastHistorySignal.value = [
        { id: 1, message: 'old info message', type: 'info', createdAt: Date.now() - 60000 },
        { id: 2, message: 'old error message', type: 'error', createdAt: Date.now() - 30000 },
        { id: 3, message: 'old success message', type: 'success', createdAt: Date.now() - 10000 },
      ];
      state.toastHistoryOpenSignal.value = true;
    });
    await page.waitForSelector('[role="dialog"][aria-label="Toast history"]', { state: 'visible' });
    await assertLightTheme(page);
    const results = await new AxeBuilder({ page })
      .include('[role="dialog"][aria-label="Toast history"]')
      .options({ runOnly: ['color-contrast'] })
      .analyze();
    expect(results.violations, summarizeViolations(results.violations)).toHaveLength(0);
  });

  test('T11 toast variants (info + success + error) — zero color-contrast violations', async ({ page }) => {
    await page.goto('/?token=test');
    await waitForAppReady(page);
    await page.evaluate(async () => {
      const mod: any = await import('/static/app/Toast.js');
      const state: any = await import('/static/app/state.js');
      state.toastsSignal.value = [];
      mod.addToast('info variant message', 'info');
      mod.addToast('success variant message', 'success');
      mod.addToast('error variant message', 'error');
    });
    await page.waitForTimeout(250);
    await assertLightTheme(page);
    const results = await new AxeBuilder({ page })
      .include('[role="alert"][aria-live="assertive"]')
      .include('[role="status"][aria-live="polite"]')
      .options({ runOnly: ['color-contrast'] })
      .analyze();
    expect(results.violations, summarizeViolations(results.violations)).toHaveLength(0);
  });
});
