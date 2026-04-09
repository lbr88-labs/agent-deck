// Phase 9 / Plan 04: POL-6 targeted luminance contrast checks.
//
// This spec is the REGRESSION-RESISTANT layer for POL-6. Instead of relying
// on axe-core's color-contrast rule (which can change its heuristics across
// versions), it reads computed styles directly via page.evaluate, walks the
// DOM ancestry to find a non-transparent background, and computes the WCAG
// 2.1 contrast ratio with the standard (L1 + 0.05) / (L2 + 0.05) formula.
//
// Each test asserts ratio >= 4.5 (WCAG AA for normal text). For 14pt bold
// or 18pt regular text the threshold is 3.0, but we apply the stricter 4.5
// for simplicity and because Tailwind text-xs / text-sm in our app are
// well below 18pt.
//
// Targets are the elements flagged by earlier plans as "POL-6 territory":
//   - session row tool label (06-03 deferred item #5)
//   - session row cost badge (06-03 deferred item #5)
//   - group row count chip (06-03 deferred item #5)
//   - profile dropdown inactive option (06-01 / 09-02 handoff note)
//   - cost summary card subtitle (06-04 handoff note)
//   - toast history drawer timestamp (06-04 deferred item #8)
//   - empty state dashboard body text (06-04 deferred)
import { test, expect } from '@playwright/test';

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

const FIXTURE_PROFILES = {
  current: 'default',
  profiles: ['default', 'work', 'personal', 'research', 'client-a', 'client-b', 'archived', 'staging', '_test'],
};

const FIXTURE_SETTINGS = { webMutations: true };

// Runs inside page.evaluate — duplicated here (can't import) because page.evaluate
// serializes the function body only. The helper returns { fg, bg, ratio } for
// the given selector. Walks ancestors if background is transparent.
const computeContrastInPage = `
(function(selector) {
  function parseRgb(s) {
    const m = s.match(/rgba?\\((\\d+),\\s*(\\d+),\\s*(\\d+)(?:,\\s*([\\d.]+))?/);
    if (!m) return null;
    return [parseInt(m[1], 10), parseInt(m[2], 10), parseInt(m[3], 10), m[4] != null ? parseFloat(m[4]) : 1];
  }
  function toLinear(c) {
    const s = c / 255;
    return s <= 0.03928 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4);
  }
  function relativeLuminance(r, g, b) {
    return 0.2126 * toLinear(r) + 0.7152 * toLinear(g) + 0.0722 * toLinear(b);
  }
  function compositeOverWhite(rgba) {
    // Approximate background blending against the page's bg-white (255,255,255)
    const [r, g, b, a] = rgba;
    if (a >= 1) return [r, g, b];
    return [
      Math.round(r * a + 255 * (1 - a)),
      Math.round(g * a + 255 * (1 - a)),
      Math.round(b * a + 255 * (1 - a)),
    ];
  }
  function findOpaqueBg(el) {
    let node = el;
    while (node && node !== document.body) {
      const cs = getComputedStyle(node);
      const bg = cs.backgroundColor;
      const rgba = parseRgb(bg);
      if (rgba && rgba[3] > 0) {
        return compositeOverWhite(rgba);
      }
      node = node.parentElement;
    }
    return [255, 255, 255];
  }
  const el = document.querySelector(selector);
  if (!el) return { error: 'element not found: ' + selector };
  const cs = getComputedStyle(el);
  const fgRaw = parseRgb(cs.color);
  if (!fgRaw) return { error: 'cannot parse fg color: ' + cs.color };
  const fg = [fgRaw[0], fgRaw[1], fgRaw[2]];
  const bg = findOpaqueBg(el);
  const L1 = relativeLuminance(fg[0], fg[1], fg[2]);
  const L2 = relativeLuminance(bg[0], bg[1], bg[2]);
  const hi = Math.max(L1, L2);
  const lo = Math.min(L1, L2);
  const ratio = (hi + 0.05) / (lo + 0.05);
  return {
    fg: 'rgb(' + fg.join(',') + ')',
    bg: 'rgb(' + bg.join(',') + ')',
    ratio: Math.round(ratio * 100) / 100,
    fontSize: cs.fontSize,
    fontWeight: cs.fontWeight,
  };
})
`;

async function forceLight(page) {
  await page.addInitScript(() => {
    localStorage.setItem('theme', 'light');
  });
}

async function mockEndpoints(page, opts: { menu?: any } = {}) {
  const menu = opts.menu || FIXTURE_MENU;
  await page.route('**/api/menu*', r => r.fulfill({ json: menu }));
  await page.route('**/api/costs/summary*', r => r.fulfill({ json: FIXTURE_COSTS_SUMMARY }));
  await page.route('**/api/costs/daily*', r => r.fulfill({ json: [] }));
  await page.route('**/api/costs/models*', r => r.fulfill({ json: {} }));
  await page.route('**/api/profiles*', r => r.fulfill({ json: FIXTURE_PROFILES }));
  await page.route('**/api/settings*', r => r.fulfill({ json: FIXTURE_SETTINGS }));
  await page.route('**/events/menu*', r => r.abort());
}

async function waitForAppReady(page) {
  await page.waitForSelector('header', { state: 'attached', timeout: 15000 });
  await page.waitForTimeout(150);
}

test.describe('POL-6 targeted luminance contrast checks', () => {
  test.beforeEach(async ({ page }) => {
    await forceLight(page);
    await mockEndpoints(page);
  });

  test('L1 session row tool label has contrast >= 4.5', async ({ page }) => {
    await page.goto('/?token=test');
    await waitForAppReady(page);
    await page.waitForSelector('#preact-session-list button[data-session-id="s1"]');
    // The tool label is the span containing "claude" inside the row
    const result: any = await page.evaluate(`(${computeContrastInPage})('button[data-session-id=\"s1\"] > span.text-xs:not(.font-mono)')`);
    expect(result.error, JSON.stringify(result)).toBeUndefined();
    expect(result.ratio, `fg=${result.fg} bg=${result.bg} ratio=${result.ratio} fontSize=${result.fontSize}`).toBeGreaterThanOrEqual(4.5);
  });

  test('L2 session row cost badge has contrast >= 4.5', async ({ page }) => {
    await page.goto('/?token=test');
    await waitForAppReady(page);
    // Seed a cost for s1 via sessionCostsSignal
    await page.evaluate(async () => {
      const state: any = await import('/static/app/state.js');
      state.sessionCostsSignal.value = { s1: 1.23 };
    });
    await page.waitForTimeout(150);
    const result: any = await page.evaluate(`(${computeContrastInPage})('button[data-session-id=\"s1\"] span.font-mono')`);
    expect(result.error, JSON.stringify(result)).toBeUndefined();
    expect(result.ratio, `fg=${result.fg} bg=${result.bg} ratio=${result.ratio}`).toBeGreaterThanOrEqual(4.5);
  });

  test('L3 group row count chip has contrast >= 4.5', async ({ page }) => {
    await page.goto('/?token=test');
    await waitForAppReady(page);
    await page.waitForSelector('#preact-session-list button[aria-expanded]');
    // The count chip is the span matching (N) inside the group button
    const result: any = await page.evaluate(`(function() {
      const buttons = Array.from(document.querySelectorAll('#preact-session-list button[aria-expanded]'));
      if (!buttons.length) return { error: 'no group buttons' };
      const btn = buttons[0];
      // The count span is the third child span (arrow, title, count, action-cluster)
      const countSpan = Array.from(btn.querySelectorAll('span')).find(s => /^\\(\\d+\\)$/.test((s.textContent || '').trim()));
      if (!countSpan) return { error: 'count span not found' };
      countSpan.setAttribute('data-pol6-target', 'count-chip');
      return { found: true };
    })()`);
    expect((result as any).error, JSON.stringify(result)).toBeUndefined();
    const contrast: any = await page.evaluate(`(${computeContrastInPage})('[data-pol6-target="count-chip"]')`);
    expect(contrast.error, JSON.stringify(contrast)).toBeUndefined();
    expect(contrast.ratio, `fg=${contrast.fg} bg=${contrast.bg} ratio=${contrast.ratio}`).toBeGreaterThanOrEqual(4.5);
  });

  test('L4 profile dropdown inactive option has contrast >= 4.5', async ({ page }) => {
    await page.goto('/?token=test');
    await waitForAppReady(page);
    const profileBtn = page.locator('[data-testid="profile-indicator"] button[aria-haspopup="listbox"]');
    await profileBtn.waitFor({ state: 'visible' });
    await profileBtn.click();
    await page.waitForSelector('[role="listbox"] [role="option"]');
    // Find a non-selected option (aria-selected="false")
    const result: any = await page.evaluate(`(function() {
      const opts = Array.from(document.querySelectorAll('[role="listbox"] [role="option"][aria-selected="false"]'));
      if (!opts.length) return { error: 'no inactive options' };
      opts[0].setAttribute('data-pol6-target', 'inactive-option');
      return { found: true };
    })()`);
    expect((result as any).error, JSON.stringify(result)).toBeUndefined();
    const contrast: any = await page.evaluate(`(${computeContrastInPage})('[data-pol6-target="inactive-option"]')`);
    expect(contrast.error, JSON.stringify(contrast)).toBeUndefined();
    expect(contrast.ratio, `fg=${contrast.fg} bg=${contrast.bg} ratio=${contrast.ratio}`).toBeGreaterThanOrEqual(4.5);
  });

  test('L5 cost summary card subtitle has contrast >= 4.5', async ({ page }) => {
    await page.goto('/?token=test');
    await waitForAppReady(page);
    await page.evaluate(async () => {
      const state: any = await import('/static/app/state.js');
      state.activeTabSignal.value = 'costs';
    });
    await page.waitForSelector('text=Today', { state: 'visible' });
    // The "events" subtitle line under each card (e.g. "5 events")
    const result: any = await page.evaluate(`(function() {
      const divs = Array.from(document.querySelectorAll('main .grid > div'));
      if (!divs.length) return { error: 'no cost cards' };
      const card = divs[0];
      const subtitle = Array.from(card.querySelectorAll('div')).find(d => /events|avg/.test((d.textContent || '').trim()) && d.querySelectorAll('div').length === 0);
      if (!subtitle) return { error: 'subtitle not found' };
      subtitle.setAttribute('data-pol6-target', 'cost-subtitle');
      return { found: true };
    })()`);
    expect((result as any).error, JSON.stringify(result)).toBeUndefined();
    const contrast: any = await page.evaluate(`(${computeContrastInPage})('[data-pol6-target="cost-subtitle"]')`);
    expect(contrast.error, JSON.stringify(contrast)).toBeUndefined();
    expect(contrast.ratio, `fg=${contrast.fg} bg=${contrast.bg} ratio=${contrast.ratio}`).toBeGreaterThanOrEqual(4.5);
  });

  test('L6 toast history drawer timestamp has contrast >= 4.5', async ({ page }) => {
    await page.goto('/?token=test');
    await waitForAppReady(page);
    await page.evaluate(async () => {
      const state: any = await import('/static/app/state.js');
      state.toastHistorySignal.value = [
        { id: 1, message: 'info message', type: 'info', createdAt: Date.now() - 60000 },
        { id: 2, message: 'another info', type: 'info', createdAt: Date.now() - 30000 },
      ];
      state.toastHistoryOpenSignal.value = true;
    });
    await page.waitForSelector('[role="dialog"][aria-label="Toast history"] ul li');
    // The timestamp is the first font-mono span inside a non-error history row
    const result: any = await page.evaluate(`(function() {
      const lis = Array.from(document.querySelectorAll('[role="dialog"][aria-label="Toast history"] ul li'));
      if (!lis.length) return { error: 'no history rows' };
      const nonError = lis.find(li => !li.className.includes('bg-red-50'));
      if (!nonError) return { error: 'no non-error row' };
      const ts = nonError.querySelector('span.font-mono');
      if (!ts) return { error: 'no timestamp' };
      ts.setAttribute('data-pol6-target', 'drawer-ts');
      return { found: true };
    })()`);
    expect((result as any).error, JSON.stringify(result)).toBeUndefined();
    const contrast: any = await page.evaluate(`(${computeContrastInPage})('[data-pol6-target="drawer-ts"]')`);
    expect(contrast.error, JSON.stringify(contrast)).toBeUndefined();
    expect(contrast.ratio, `fg=${contrast.fg} bg=${contrast.bg} ratio=${contrast.ratio}`).toBeGreaterThanOrEqual(4.5);
  });

  test('L7 empty state dashboard body text has contrast >= 4.5', async ({ page }) => {
    await page.route('**/api/menu*', r => r.fulfill({ json: EMPTY_MENU }));
    await page.goto('/?token=test');
    await waitForAppReady(page);
    await page.waitForSelector('[data-testid="empty-state-dashboard"]');
    // The keyboard-hint paragraph with `<kbd>` elements, and the "No sessions yet" line
    const result: any = await page.evaluate(`(function() {
      const root = document.querySelector('[data-testid="empty-state-dashboard"]');
      if (!root) return { error: 'no empty state dashboard' };
      // Find the paragraph containing "No sessions yet"
      const ps = Array.from(root.querySelectorAll('p'));
      const target = ps.find(p => /No sessions yet/i.test((p.textContent || '')));
      if (!target) return { error: 'no empty-state paragraph' };
      target.setAttribute('data-pol6-target', 'empty-body');
      return { found: true };
    })()`);
    expect((result as any).error, JSON.stringify(result)).toBeUndefined();
    const contrast: any = await page.evaluate(`(${computeContrastInPage})('[data-pol6-target="empty-body"]')`);
    expect(contrast.error, JSON.stringify(contrast)).toBeUndefined();
    expect(contrast.ratio, `fg=${contrast.fg} bg=${contrast.bg} ratio=${contrast.ratio}`).toBeGreaterThanOrEqual(4.5);
  });
});
