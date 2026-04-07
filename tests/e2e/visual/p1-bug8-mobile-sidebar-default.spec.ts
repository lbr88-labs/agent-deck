import { test, expect } from '@playwright/test';
import { readFileSync } from 'fs';
import { join } from 'path';

/**
 * Phase 3 / Plan 06 / Task 1: BUG #8 / LAYT-05 regression test
 *
 * Asserts the mobile sidebar defaults to CLOSED on cold load with no
 * localStorage entry; the desktop sidebar stays OPEN; explicit user
 * preference in localStorage overrides the viewport-based default in
 * both directions.
 *
 * Root cause (LOCKED per 03-CONTEXT.md): state.js line 30 initializes
 * sidebarOpenSignal with `localStorage.getItem(...) !== 'false'`, which
 * evaluates to `null !== 'false'` === `true` on a fresh phone — so the
 * sidebar starts OPEN and the overlay covers the entire 375px screen
 * blocking the terminal.
 *
 * Fix (LOCKED per 03-CONTEXT.md): replace the naive initializer with a
 * function that respects explicit localStorage values first, then falls
 * back to `window.innerWidth >= 768` (open on tablet/desktop, closed on
 * phone).
 *
 * TDD ORDER: committed in failing state in Task 1, flipped to green in Task 2.
 */

const STATE_PATH = join(
  __dirname, '..', '..', '..', 'internal', 'web', 'static', 'app', 'state.js',
);

function readSrc(): string {
  return readFileSync(STATE_PATH, 'utf-8');
}

async function clearLocalStorage(page: import('@playwright/test').Page): Promise<void> {
  await page.addInitScript(() => {
    try { window.localStorage.clear(); } catch (_) {}
  });
}

async function setLocalStorageSidebar(
  page: import('@playwright/test').Page,
  value: 'true' | 'false',
): Promise<void> {
  await page.addInitScript((v) => {
    try {
      window.localStorage.clear();
      window.localStorage.setItem('agentdeck.sidebarOpen', v as string);
    } catch (_) {}
  }, value);
}

async function readAsideState(page: import('@playwright/test').Page): Promise<{
  x: number;
  width: number;
  className: string;
  hasBackdrop: boolean;
} | null> {
  return page.evaluate(() => {
    const aside = document.querySelector('aside');
    if (!aside) return null;
    const rect = aside.getBoundingClientRect();
    return {
      x: rect.x,
      width: rect.width,
      className: aside.className || '',
      hasBackdrop: !!document.querySelector('.bg-black\\/50'),
    };
  });
}

test.describe('BUG #8 / LAYT-05 — mobile sidebar defaults to closed', () => {
  // STRUCTURAL — always run, fail before fix.

  test('structural: state.js has an initialSidebarOpen helper or viewport-based default', () => {
    const src = readSrc();
    const re = /initialSidebarOpen|window\.innerWidth >= 768/;
    expect(
      re.test(src),
      'state.js must replace the naive `localStorage.getItem(...) !== \'false\'` initializer with a function that branches on window.innerWidth for the no-preference case (LAYT-05).',
    ).toBe(true);
  });

  test("structural: state.js no longer uses the naive `!== 'false'` initializer", () => {
    const src = readSrc();
    // Look for the old pattern: signal(\n localStorage...getItem(...) !== 'false'\n )
    const re = /signal\(\s*localStorage\.getItem\('agentdeck\.sidebarOpen'\) !== 'false'\s*\)/;
    expect(
      re.test(src),
      'state.js still uses the naive `signal(localStorage.getItem(\'agentdeck.sidebarOpen\') !== \'false\')` initializer. LAYT-05 requires a helper function that branches on viewport width when no preference exists.',
    ).toBe(false);
  });

  // RUNTIME — four scenarios.

  test('runtime: mobile 375x812 no-preference cold load → sidebar CLOSED + no backdrop', async ({ page }) => {
    await clearLocalStorage(page);
    await page.setViewportSize({ width: 375, height: 812 });
    await page.goto('/?t=test');
    await page.waitForSelector('header', { state: 'attached', timeout: 15000 });
    await page.waitForTimeout(300);

    const state = await readAsideState(page);
    expect(state, 'aside element not found').not.toBeNull();
    const closed = state!.x < 0 || state!.className.includes('-translate-x-full');
    expect(
      closed,
      `mobile cold load must start with sidebar closed; got x=${state!.x}, className=${state!.className}`,
    ).toBe(true);
    expect(
      state!.hasBackdrop,
      'mobile cold load must not show the overlay backdrop when sidebar is closed',
    ).toBe(false);
  });

  test('runtime: desktop 1280x800 no-preference cold load → sidebar OPEN (no regression)', async ({ page }) => {
    await clearLocalStorage(page);
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto('/?t=test');
    await page.waitForSelector('header', { state: 'attached', timeout: 15000 });
    await page.waitForTimeout(300);

    const state = await readAsideState(page);
    expect(state, 'aside element not found').not.toBeNull();
    // Desktop sidebar is laid out statically via md:relative md:w-64, so x should be >= 0.
    expect(
      state!.x,
      `desktop cold load must start with sidebar visible (x >= 0); got x=${state!.x}, className=${state!.className}`,
    ).toBeGreaterThanOrEqual(0);
  });

  test("runtime: mobile 375x812 localStorage='true' → sidebar OPEN (user preference overrides)", async ({ page }) => {
    await setLocalStorageSidebar(page, 'true');
    await page.setViewportSize({ width: 375, height: 812 });
    await page.goto('/?t=test');
    await page.waitForSelector('header', { state: 'attached', timeout: 15000 });
    await page.waitForTimeout(300);

    const state = await readAsideState(page);
    expect(state, 'aside element not found').not.toBeNull();
    const open = state!.x >= 0 && state!.className.includes('translate-x-0');
    expect(
      open,
      `mobile with localStorage='true' must start open; got x=${state!.x}, className=${state!.className}`,
    ).toBe(true);
  });

  test("runtime: mobile 375x812 localStorage='false' → sidebar CLOSED (user preference respected)", async ({ page }) => {
    await setLocalStorageSidebar(page, 'false');
    await page.setViewportSize({ width: 375, height: 812 });
    await page.goto('/?t=test');
    await page.waitForSelector('header', { state: 'attached', timeout: 15000 });
    await page.waitForTimeout(300);

    const state = await readAsideState(page);
    expect(state, 'aside element not found').not.toBeNull();
    const closed = state!.x < 0 || state!.className.includes('-translate-x-full');
    expect(
      closed,
      `mobile with localStorage='false' must start closed; got x=${state!.x}, className=${state!.className}`,
    ).toBe(true);
  });
});
