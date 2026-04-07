import { test, expect } from '@playwright/test';
import { readFileSync } from 'fs';
import { join } from 'path';

/**
 * Phase 3 / Plan 07 / Task 1: BUG #4 + BUG #10 / LAYT-01 regression test
 *
 * Asserts the sidebar has a responsive width backed by sidebarWidthSignal,
 * persists to localStorage['sidebar-width'], is draggable via a
 * [data-testid="sidebar-resize-handle"] handle, and clamps to [200, 480]
 * with default 280. Mobile keeps the existing w-72 (288px) overlay size
 * and hides the resize handle.
 *
 * Root cause (LOCKED per 03-CONTEXT.md): AppShell.js line 82 hardcodes
 * md:w-64 (256px) on the <aside>; no resize handle exists.
 *
 * Fix (LOCKED per 03-CONTEXT.md):
 *   - state.js: new sidebarWidthSignal exported, initialized from
 *     localStorage['sidebar-width'] clamped [200, 480], default 280
 *   - AppShell.js: remove md:w-64, use inline style="width: Npx" on
 *     the <aside> on md+; add <SidebarResizeHandle> sibling with
 *     data-testid="sidebar-resize-handle" hidden on mobile
 *   - drag: pointer events (not mouse*), clamp on move, persist on up
 *   - mobile: w-72 preserved, handle hidden
 *
 * TDD ORDER: committed in failing state in Task 1, flipped to green in Task 2.
 */

const STATE_PATH = join(
  __dirname, '..', '..', '..', 'internal', 'web', 'static', 'app', 'state.js',
);
const APPSHELL_PATH = join(
  __dirname, '..', '..', '..', 'internal', 'web', 'static', 'app', 'AppShell.js',
);

function readSrc(p: string): string {
  return readFileSync(p, 'utf-8');
}

async function initLocalStorage(
  page: import('@playwright/test').Page,
  entries: Record<string, string>,
): Promise<void> {
  await page.addInitScript((data) => {
    try {
      window.localStorage.clear();
      const obj = data as Record<string, string>;
      for (const k in obj) {
        window.localStorage.setItem(k, obj[k]);
      }
    } catch (_) {}
  }, entries);
}

async function asideWidth(page: import('@playwright/test').Page): Promise<number | null> {
  return page.evaluate(() => {
    const aside = document.querySelector('aside');
    if (!aside) return null;
    return aside.getBoundingClientRect().width;
  });
}

test.describe('BUG #4 + BUG #10 / LAYT-01 — responsive sidebar width with drag resize handle', () => {
  // STRUCTURAL — always run, fail before fix.

  test('structural: state.js exports sidebarWidthSignal', () => {
    const src = readSrc(STATE_PATH);
    expect(
      /export const sidebarWidthSignal = signal/.test(src),
      'state.js must export sidebarWidthSignal = signal(...). LAYT-01 locks this signal name.',
    ).toBe(true);
  });

  test('structural: AppShell.js imports and uses sidebarWidthSignal', () => {
    const src = readSrc(APPSHELL_PATH);
    expect(
      /sidebarWidthSignal/.test(src),
      'AppShell.js must import sidebarWidthSignal from state.js and use it on the <aside> inline width.',
    ).toBe(true);
  });

  test('structural: AppShell.js has data-testid="sidebar-resize-handle"', () => {
    const src = readSrc(APPSHELL_PATH);
    expect(
      /data-testid="sidebar-resize-handle"/.test(src),
      'AppShell.js must render a resize handle with data-testid="sidebar-resize-handle" adjacent to the <aside>.',
    ).toBe(true);
  });

  test('structural: AppShell.js no longer has md:w-64 on the aside', () => {
    const src = readSrc(APPSHELL_PATH);
    const re = /<aside[\s\S]*?md:w-64/;
    expect(
      re.test(src),
      'AppShell.js <aside> still has the hardcoded md:w-64 class. LAYT-01 replaces this with an inline style bound to sidebarWidthSignal on md+.',
    ).toBe(false);
  });

  test('structural: AppShell.js preserves w-72 mobile fallback on the aside', () => {
    const src = readSrc(APPSHELL_PATH);
    const re = /<aside[\s\S]*?w-72/;
    expect(
      re.test(src),
      'AppShell.js <aside> must keep w-72 (mobile overlay width) — LAYT-01 only replaces md:w-64 with inline width.',
    ).toBe(true);
  });

  // RUNTIME — desktop width tests.

  test('runtime: desktop no-preference cold load → aside width === 280 (default)', async ({ page }) => {
    await initLocalStorage(page, {});
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto('/?t=test');
    await page.waitForSelector('header', { state: 'attached', timeout: 15000 });
    await page.waitForTimeout(200);

    const width = await asideWidth(page);
    expect(width, 'aside not found').not.toBeNull();
    expect(
      Math.round(width!),
      `default width must be 280; got ${width}`,
    ).toBe(280);
  });

  test("runtime: desktop localStorage['sidebar-width']='320' → aside width === 320", async ({ page }) => {
    await initLocalStorage(page, { 'sidebar-width': '320' });
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto('/?t=test');
    await page.waitForSelector('header', { state: 'attached', timeout: 15000 });
    await page.waitForTimeout(200);

    const width = await asideWidth(page);
    expect(width, 'aside not found').not.toBeNull();
    expect(
      Math.round(width!),
      `stored width 320 must restore; got ${width}`,
    ).toBe(320);
  });

  test("runtime: desktop localStorage['sidebar-width']='600' → aside width === 480 (clamp max)", async ({ page }) => {
    await initLocalStorage(page, { 'sidebar-width': '600' });
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto('/?t=test');
    await page.waitForSelector('header', { state: 'attached', timeout: 15000 });
    await page.waitForTimeout(200);

    const width = await asideWidth(page);
    expect(width, 'aside not found').not.toBeNull();
    expect(
      Math.round(width!),
      `clamped high value should be 480; got ${width}`,
    ).toBe(480);
  });

  test("runtime: desktop localStorage['sidebar-width']='50' → aside width === 200 (clamp min)", async ({ page }) => {
    await initLocalStorage(page, { 'sidebar-width': '50' });
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto('/?t=test');
    await page.waitForSelector('header', { state: 'attached', timeout: 15000 });
    await page.waitForTimeout(200);

    const width = await asideWidth(page);
    expect(width, 'aside not found').not.toBeNull();
    expect(
      Math.round(width!),
      `clamped low value should be 200; got ${width}`,
    ).toBe(200);
  });

  test('runtime: dragging the resize handle +60px increases aside width by ~60', async ({ page }) => {
    await initLocalStorage(page, {});
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto('/?t=test');
    await page.waitForSelector('header', { state: 'attached', timeout: 15000 });
    await page.waitForTimeout(200);

    const startWidth = await asideWidth(page);
    expect(startWidth, 'aside not found').not.toBeNull();

    const handle = page.locator('[data-testid="sidebar-resize-handle"]');
    const handleCount = await handle.count();
    expect(handleCount, 'resize handle not found').toBeGreaterThan(0);

    const box = await handle.boundingBox();
    expect(box, 'resize handle has no boundingBox').not.toBeNull();
    const startX = box!.x + box!.width / 2;
    const startY = box!.y + box!.height / 2;

    await page.mouse.move(startX, startY);
    await page.mouse.down();
    await page.mouse.move(startX + 60, startY, { steps: 10 });
    await page.mouse.up();
    await page.waitForTimeout(200);

    const newWidth = await asideWidth(page);
    expect(newWidth, 'aside width after drag').not.toBeNull();
    expect(
      Math.abs(newWidth! - (startWidth! + 60)),
      `drag of +60 should yield width ~${startWidth! + 60}, got ${newWidth}`,
    ).toBeLessThanOrEqual(5);
  });

  test('runtime: width persists to localStorage across reload after drag', async ({ page }) => {
    // Do NOT use addInitScript here — it would re-clear localStorage on
    // page.reload() and wipe the dragged-in width. Clear once manually
    // after the first navigation, then drag + reload.
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto('/?t=test');
    await page.evaluate(() => { try { window.localStorage.clear(); } catch (_) {} });
    await page.reload();
    await page.waitForSelector('header', { state: 'attached', timeout: 15000 });
    await page.waitForTimeout(200);

    const handle = page.locator('[data-testid="sidebar-resize-handle"]');
    const handleCount = await handle.count();
    expect(handleCount, 'resize handle not found').toBeGreaterThan(0);

    const box = await handle.boundingBox();
    expect(box, 'resize handle has no boundingBox').not.toBeNull();
    const startX = box!.x + box!.width / 2;
    const startY = box!.y + box!.height / 2;

    await page.mouse.move(startX, startY);
    await page.mouse.down();
    await page.mouse.move(startX + 60, startY, { steps: 10 });
    await page.mouse.up();
    await page.waitForTimeout(300);

    const widthBefore = await asideWidth(page);
    // Confirm localStorage was actually written by the pointerup handler.
    const stored = await page.evaluate(() => window.localStorage.getItem('sidebar-width'));
    expect(stored, 'pointerup must have persisted sidebar-width to localStorage').not.toBeNull();

    // Reload the page (no init script clears localStorage).
    await page.reload();
    await page.waitForSelector('header', { state: 'attached', timeout: 15000 });
    await page.waitForTimeout(200);

    const widthAfter = await asideWidth(page);
    expect(widthAfter, 'aside width after reload').not.toBeNull();
    expect(
      Math.abs(widthAfter! - widthBefore!),
      `width must persist across reload; before=${widthBefore} after=${widthAfter}`,
    ).toBeLessThanOrEqual(1);
  });

  test('runtime: mobile 375x812 → aside width === 288 (w-72 preserved) and handle hidden', async ({ page }) => {
    // Plan 06 made mobile sidebar default to closed; force it open so we can measure the aside.
    await initLocalStorage(page, { 'agentdeck.sidebarOpen': 'true' });
    await page.setViewportSize({ width: 375, height: 812 });
    await page.goto('/?t=test');
    await page.waitForSelector('header', { state: 'attached', timeout: 15000 });
    await page.waitForTimeout(300);

    const width = await asideWidth(page);
    expect(width, 'aside not found').not.toBeNull();
    expect(
      Math.round(width!),
      `mobile aside width must be 288 (w-72); got ${width}`,
    ).toBe(288);

    const handleVisible = await page.locator('[data-testid="sidebar-resize-handle"]').isVisible().catch(() => false);
    expect(
      handleVisible,
      'resize handle must be hidden on mobile (class hidden md:block)',
    ).toBe(false);
  });
});
