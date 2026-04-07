import { test, expect } from '@playwright/test';

/**
 * Phase 2 / Plan 01 / Task 1: BUG #2 / CRIT-02 regression test
 *
 * Asserts that a cold load of `/` records zero `pageerror` events at both
 * desktop (1280x800) and mobile (375x812) viewports, and that no error
 * message contains the substring `onShowLinkUnderline`.
 *
 * Root cause (LOCKED per 02-CONTEXT.md): internal/web/static/index.html line 61
 * loads `/static/vendor/addon-canvas.js` as a UMD <script>. That addon was
 * built against an older xterm.js API and references
 * `_linkifier2.onShowLinkUnderline`, but the bundled xterm is v6 which
 * removed the canvas addon API entirely. The addon throws synchronously at
 * page load, before Preact mounts, and the error cascades into BUG #1.
 *
 * Fix (LOCKED per 02-CONTEXT.md): delete the <script> tag from index.html.
 * The dead `typeof window.CanvasAddon !== 'undefined'` guards in
 * TerminalPanel.js become permanently false, xterm v6 DOM renderer is the
 * safe fallback. Do NOT delete vendor/addon-canvas.js itself, PERF-02 owns
 * deletion in a later phase.
 *
 * TDD ORDER: this spec is committed in FAILING state in Task 1, then the
 * fix lands in Task 2, flipping the spec to green.
 */

interface CapturedError {
  message: string;
  name: string;
  stack: string;
}

async function collectPageErrors(
  page: import('@playwright/test').Page,
  viewport: { width: number; height: number },
): Promise<CapturedError[]> {
  const errors: CapturedError[] = [];
  page.on('pageerror', (err) => {
    errors.push({
      message: err.message || String(err),
      name: err.name || 'Error',
      stack: err.stack || '',
    });
  });
  await page.setViewportSize(viewport);
  await page.goto('/?t=test');
  // Wait for Preact AppShell to mount, the same gate cascade-verify.spec.ts uses.
  await page.waitForSelector('header', { state: 'attached', timeout: 15000 });
  // Additional 500 ms to catch any deferred script errors that fire after
  // the initial render tick.
  await page.waitForTimeout(500);
  return errors;
}

function formatErrors(errors: CapturedError[]): string {
  if (errors.length === 0) return '(none)';
  return errors
    .map((e, i) => `  [${i}] ${e.name}: ${e.message}`)
    .join('\n');
}

test.describe('BUG #2 / CRIT-02 — no JS errors on cold load', () => {
  test('desktop 1280x800: zero pageerror events on cold load', async ({ page }) => {
    const errors = await collectPageErrors(page, { width: 1280, height: 800 });
    expect(
      errors,
      `cold load must produce zero pageerror events; got:\n${formatErrors(errors)}`,
    ).toEqual([]);
  });

  test('desktop 1280x800: no error message mentions onShowLinkUnderline', async ({ page }) => {
    const errors = await collectPageErrors(page, { width: 1280, height: 800 });
    const culprit = errors.find((e) => /onShowLinkUnderline/i.test(e.message));
    expect(
      culprit,
      `an onShowLinkUnderline error fired, this is the addon-canvas.js xterm v6 incompatibility (BUG #2 / CRIT-02). Fix: delete the <script src="/static/vendor/addon-canvas.js"> tag from internal/web/static/index.html.\n${formatErrors(errors)}`,
    ).toBeUndefined();
  });

  test('mobile 375x812: zero pageerror events on cold load', async ({ page }) => {
    const errors = await collectPageErrors(page, { width: 375, height: 812 });
    expect(
      errors,
      `mobile cold load must produce zero pageerror events; got:\n${formatErrors(errors)}`,
    ).toEqual([]);
  });

  test('structural gate: addon-canvas.js script tag must not be present', async ({ page }) => {
    // Structural fallback: even if the page.on('pageerror') listener misses a
    // UMD-global synchronous throw, the presence of the <script> tag alone is
    // sufficient to fail the test. This keeps the regression gate honest
    // across browser/runtime variations where the error might be swallowed.
    await page.goto('/?t=test');
    await page.waitForSelector('header', { state: 'attached', timeout: 15000 });
    const hasBrokenScript = await page.evaluate(
      () => !!document.querySelector('script[src*="addon-canvas.js"]'),
    );
    expect(
      hasBrokenScript,
      'index.html must not load /static/vendor/addon-canvas.js — this UMD addon throws at load under xterm v6 (BUG #2 / CRIT-02).',
    ).toBe(false);
  });
});
