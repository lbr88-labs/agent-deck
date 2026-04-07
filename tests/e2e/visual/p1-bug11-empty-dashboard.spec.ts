import { test, expect } from '@playwright/test';
import { readFileSync } from 'fs';
import { join } from 'path';

/**
 * Phase 3 / Plan 04 / Task 1: BUG #11 / LAYT-07 regression test
 *
 * Asserts that the empty-state dashboard renders when sessions exist but
 * no session is selected. Exercises two locked fixes:
 *
 *   1. App.js popstate handler explicitly clears selectedIdSignal when the
 *      user navigates back to /.
 *   2. EmptyStateDashboard.js renders a "Recently active" list when
 *      sessions.length > 0, with clickable buttons that set selectedIdSignal.
 *
 * Root cause (LOCKED per 03-CONTEXT.md): the current popstate handler's
 * comment says "Don't clear selection on popstate to root: user may still
 * want it" — so hitting back on the browser leaves selectedIdSignal stale,
 * and TerminalPanel keeps trying to render a dead terminal instead of the
 * dashboard. Also, EmptyStateDashboard was designed for "no sessions yet"
 * and shows nothing useful when users with existing sessions just want to
 * see an overview.
 *
 * Fix (LOCKED per 03-CONTEXT.md): clear the signal on root-path popstate,
 * and add a Recently active list to the dashboard.
 *
 * TDD ORDER: committed in failing state in Task 1, flipped to green in Task 2.
 *
 * STRUCTURAL FALLBACK: three file-read tests always run and fail before fix.
 */

const APP_PATH = join(
  __dirname, '..', '..', '..', 'internal', 'web', 'static', 'app', 'App.js',
);
const DASHBOARD_PATH = join(
  __dirname, '..', '..', '..', 'internal', 'web', 'static', 'app', 'EmptyStateDashboard.js',
);

function readSrc(p: string): string {
  return readFileSync(p, 'utf-8');
}

test.describe('BUG #11 / LAYT-07 — empty-state dashboard when sessions exist but no selection', () => {
  // STRUCTURAL — always run, fail before fix.

  test("structural: App.js popstate handler clears selectedIdSignal on root path", () => {
    const src = readSrc(APP_PATH);
    // Look for a branch where path === '/' (or the default case) explicitly
    // sets selectedIdSignal.value = null. The current handler has a catch
    // branch that also sets null, so we need the specific root-path context.
    const re = /(path === '\/'|if \(path === "\/"\))[\s\S]{0,120}selectedIdSignal\.value = null/;
    expect(
      re.test(src),
      'App.js popstate handler must explicitly clear selectedIdSignal when the path is / (root). LAYT-07 fix: change the "Don\'t clear" branch into an explicit clear so the dashboard re-renders on browser back navigation.',
    ).toBe(true);
  });

  test("structural: EmptyStateDashboard.js contains 'Recently active' text", () => {
    const src = readSrc(DASHBOARD_PATH);
    expect(
      /Recently active/.test(src),
      'EmptyStateDashboard.js must contain a "Recently active" section that renders when sessions.length > 0.',
    ).toBe(true);
  });

  test('structural: EmptyStateDashboard.js imports selectedIdSignal from state.js', () => {
    const src = readSrc(DASHBOARD_PATH);
    const re = /import[\s\S]*?selectedIdSignal[\s\S]*?from.*state\.js/;
    expect(
      re.test(src),
      'EmptyStateDashboard.js must import selectedIdSignal from state.js so Recently active buttons can set selection.',
    ).toBe(true);
  });

  test('structural: EmptyStateDashboard.js renders data-session-id buttons', () => {
    const src = readSrc(DASHBOARD_PATH);
    expect(
      /data-session-id/.test(src),
      'EmptyStateDashboard.js must render buttons with data-session-id for each recently-active entry.',
    ).toBe(true);
  });

  // RUNTIME — best-effort, skip without fixtures.

  test('runtime: dashboard is visible on cold load when no session is selected', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto('/?t=test');
    await page.waitForSelector('header', { state: 'attached', timeout: 15000 });
    await page.waitForTimeout(500);

    // The dashboard has an "Agent Deck" heading in the main content area.
    // Look inside <main> rather than the top brand span.
    const mainHeadingCount = await page.locator('main').locator('text=Agent Deck').count();
    expect(
      mainHeadingCount,
      'EmptyStateDashboard must render in <main> on cold load (no session selected)',
    ).toBeGreaterThan(0);
  });

  test('runtime: popstate to / re-renders dashboard after a session is selected', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto('/?t=test');
    await page.waitForSelector('header', { state: 'attached', timeout: 15000 });

    const count = await page.locator('button[data-session-id]').count();
    test.skip(count === 0, 'no fixture sessions — cannot exercise popstate');

    await page.locator('button[data-session-id]').first().click();
    // Wait for URL to become /s/{id}
    await page.waitForFunction(() => window.location.pathname.startsWith('/s/'), null, { timeout: 5000 });

    await page.goBack();
    await page.waitForFunction(() => window.location.pathname === '/', null, { timeout: 5000 });
    await page.waitForTimeout(300);

    // After popstate to /, the dashboard must re-render.
    const mainHeadingCount = await page.locator('main').locator('text=Agent Deck').count();
    expect(
      mainHeadingCount,
      'after popstate to /, EmptyStateDashboard must render in <main> (selectedIdSignal cleared)',
    ).toBeGreaterThan(0);

    // xterm must NOT be present (we're on the dashboard, not a terminal).
    const xtermCount = await page.locator('main .xterm').count();
    expect(xtermCount, 'xterm should not be mounted on the dashboard').toBe(0);
  });

  test('runtime: Recently active list renders with data-session-id buttons when sessions exist', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto('/?t=test');
    await page.waitForSelector('header', { state: 'attached', timeout: 15000 });

    const sidebarSessionCount = await page.locator('#preact-session-list button[data-session-id]').count();
    test.skip(sidebarSessionCount === 0, 'no fixture sessions — Recently active list requires at least 1 session');

    await page.waitForTimeout(500);

    // The dashboard should mention "Recently active" when sessions exist.
    const recentCount = await page.locator('main').locator('text=Recently active').count();
    expect(
      recentCount,
      'EmptyStateDashboard must render the "Recently active" section when sessions.length > 0',
    ).toBeGreaterThan(0);

    // And at least one data-session-id button inside <main> (dashboard area).
    const recentButtonCount = await page.locator('main button[data-session-id]').count();
    expect(
      recentButtonCount,
      'Recently active list must contain at least one button[data-session-id] inside <main>',
    ).toBeGreaterThan(0);
  });

  test('runtime: clicking a Recently active button navigates to /s/{id}', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto('/?t=test');
    await page.waitForSelector('header', { state: 'attached', timeout: 15000 });

    const sidebarCount = await page.locator('#preact-session-list button[data-session-id]').count();
    test.skip(sidebarCount === 0, 'no fixture sessions');

    await page.waitForTimeout(500);
    const recentButtons = page.locator('main button[data-session-id]');
    const recentCount = await recentButtons.count();
    test.skip(recentCount === 0, 'Recently active list empty — fix not yet applied or no fixture');

    const firstId = await recentButtons.first().getAttribute('data-session-id');
    await recentButtons.first().click();

    await page.waitForFunction(
      (id) => window.location.pathname === '/s/' + encodeURIComponent(id || ''),
      firstId,
      { timeout: 5000 },
    );

    const finalPath = await page.evaluate(() => window.location.pathname);
    expect(
      finalPath,
      'clicking a Recently active button must navigate to /s/{id}',
    ).toBeTruthy();
  });
});
