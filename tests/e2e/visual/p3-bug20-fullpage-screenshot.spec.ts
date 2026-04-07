import { test, expect } from '@playwright/test';
import { readFileSync } from 'fs';
import { join } from 'path';
import {
  expandForFullPageScreenshot,
  collapseAfterFullPageScreenshot,
} from './screenshot-helpers.js';

/**
 * Phase 4 / Plan 02 / Task 1: BUG #20 / COSM-02 regression test
 *
 * Asserts that the screenshot-helpers.ts module exists and exposes
 * expandForFullPageScreenshot / collapseAfterFullPageScreenshot, that the
 * helpers correctly inject and remove an overflow override, that a
 * full-page screenshot taken inside the helper window has a decoded PNG
 * height greater than the viewport (800), and that production CSS
 * (styles.src.css) still sets body { overflow: hidden }.
 *
 * Root cause (LOCKED per 04-CONTEXT.md): styles.src.css line 150 has
 * body { overflow: hidden } which is intentional production behavior
 * for the chat-style fixed-height app, but Playwright's
 * toHaveScreenshot({ fullPage: true }) walks the document scrollable
 * region and clips to viewport when overflow is hidden. Visual
 * baselines therefore capture only the viewport rectangle, not the
 * full document.
 *
 * Fix (LOCKED per 04-CONTEXT.md, TEST-ONLY): create
 * tests/e2e/visual/screenshot-helpers.ts that exports a pair of
 * helpers using page.addStyleTag to inject and later remove an
 * overflow: visible !important override. NO production CSS changes.
 *
 * TDD ORDER: this spec is committed in FAILING state in Task 1 (the
 * import resolves to a non-existent module). Task 2 creates the helper
 * module and the spec flips to green.
 */

function decodePngHeight(buf: Buffer): number {
  // PNG file format: 8-byte signature, then IHDR chunk starting at byte 8.
  // IHDR layout: 4 bytes length, 4 bytes type (0x49484452 "IHDR"),
  // 4 bytes width, 4 bytes height, 1 byte bit depth, ...
  // Width is at byte offset 16, height at byte offset 20 (big-endian uint32).
  if (buf.length < 24) {
    throw new Error(`PNG buffer too short: ${buf.length} bytes`);
  }
  return buf.readUInt32BE(20);
}

test.describe('BUG #20 / COSM-02 — full-page screenshot helper', () => {
  test('full-page screenshot with helper has height > viewport', async ({ page }) => {
    await page.goto('/?t=test');
    await page.waitForSelector('header', { state: 'attached', timeout: 15000 });

    await expandForFullPageScreenshot(page);

    const buf = await page.screenshot({ fullPage: true });
    const height = decodePngHeight(buf);

    await collapseAfterFullPageScreenshot(page);

    expect(
      height,
      `full-page screenshot height (${height}) must exceed viewport height (800) — proves expandForFullPageScreenshot bypassed body { overflow: hidden } and Playwright captured the full document. If this fails to 800, the helper did not inject the overflow override.`,
    ).toBeGreaterThan(800);
  });

  test('after collapse, getComputedStyle(body).overflow returns hidden', async ({ page }) => {
    await page.goto('/?t=test');
    await page.waitForSelector('header', { state: 'attached', timeout: 15000 });

    await expandForFullPageScreenshot(page);
    await collapseAfterFullPageScreenshot(page);

    const overflow = await page.evaluate(() => getComputedStyle(document.body).overflow);
    expect(
      overflow,
      'after collapseAfterFullPageScreenshot, body computed overflow must be "hidden" — proves the helper cleanly restored production cascade',
    ).toBe('hidden');
  });

  test('production CSS guard: styles.src.css still sets body { overflow: hidden }', () => {
    const p = join(__dirname, '..', '..', '..', 'internal', 'web', 'static', 'styles.src.css');
    const src = readFileSync(p, 'utf-8');
    // Locate a body { ... overflow: hidden ... } block. The check is whitespace-tolerant.
    const bodyOverflowRe = /body\s*\{[^}]*overflow:\s*hidden/m;
    expect(
      bodyOverflowRe.test(src),
      'styles.src.css must still contain a body block with overflow: hidden — the COSM-02 fix is test-only and MUST NOT touch production CSS',
    ).toBe(true);
  });

  test('helper module exports both expand and collapse functions', () => {
    // Import-time check: if the module is missing, the spec file fails to
    // load and ALL tests fail. This is the load-bearing TDD signal.
    expect(typeof expandForFullPageScreenshot, 'expandForFullPageScreenshot must be a function').toBe('function');
    expect(typeof collapseAfterFullPageScreenshot, 'collapseAfterFullPageScreenshot must be a function').toBe('function');
  });
});
