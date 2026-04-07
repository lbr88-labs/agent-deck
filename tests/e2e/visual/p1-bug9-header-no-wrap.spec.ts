import { test, expect } from '@playwright/test';
import { readFileSync } from 'fs';
import { join } from 'path';

/**
 * Phase 3 / Plan 05 / Task 1: BUG #9 / LAYT-06 regression test
 *
 * Asserts the Topbar header remains a single row at tablet widths
 * (768-1023px) and on mobile (375) and desktop (1280). Measures header
 * bounding-box height and verifies the brand container and action
 * container share the same top Y position.
 *
 * Root cause (LOCKED per 03-CONTEXT.md): Topbar.js line 11 has flex-wrap
 * on the outer header, so at tablet widths the right action container
 * wraps below the brand container, doubling the header height.
 *
 * Fix (LOCKED per 03-CONTEXT.md):
 *   - remove flex-wrap from the header class
 *   - add min-w-0 to the brand container so it can shrink
 *   - add flex-shrink-0 to the action container so it keeps full width
 *   - add md:hidden lg:inline to the brand text span so tablet widths
 *     hide the "Agent Deck" text and keep the icon
 *
 * TDD ORDER: committed in failing state in Task 1, flipped to green in Task 2.
 */

const TOPBAR_PATH = join(
  __dirname, '..', '..', '..', 'internal', 'web', 'static', 'app', 'Topbar.js',
);

function readSrc(): string {
  return readFileSync(TOPBAR_PATH, 'utf-8');
}

test.describe('BUG #9 / LAYT-06 — Topbar header does not wrap on tablet widths', () => {
  // STRUCTURAL — always run, fail before fix.

  test('structural: Topbar.js header element has no flex-wrap class', () => {
    const src = readSrc();
    const re = /<header class="[^"]*flex-wrap/;
    expect(
      re.test(src),
      'Topbar.js header element still has flex-wrap — remove it so the right action container never wraps below the brand container.',
    ).toBe(false);
  });

  test('structural: Topbar.js has min-w-0 somewhere on the brand container', () => {
    const src = readSrc();
    expect(
      /min-w-0/.test(src),
      'Topbar.js must add min-w-0 to the brand container so it can shrink below its intrinsic width when the header gets narrow.',
    ).toBe(true);
  });

  test('structural: Topbar.js brand text span has md:hidden lg:inline visibility classes', () => {
    const src = readSrc();
    expect(
      /md:hidden lg:inline/.test(src),
      'Topbar.js must hide the "Agent Deck" brand text on tablet widths (md:hidden) and show it on desktop (lg:inline) so the icon + hamburger cluster fits in the left half.',
    ).toBe(true);
  });

  // RUNTIME — parameterized over tablet widths, asserts single-row header.

  const tabletWidths = [768, 820, 900, 1023];
  for (const width of tabletWidths) {
    test(`runtime: header is single row at ${width}x800`, async ({ page }) => {
      await page.setViewportSize({ width, height: 800 });
      await page.goto('/?t=test');
      await page.waitForSelector('header', { state: 'attached', timeout: 15000 });
      await page.waitForTimeout(200);

      const box = await page.locator('header').boundingBox();
      expect(box, 'header must have a bounding box').not.toBeNull();
      expect(
        box!.height,
        `header height at ${width}px must be <= 64 (single row), got ${box!.height}`,
      ).toBeLessThanOrEqual(64);
      expect(
        Math.round(box!.width),
        `header width at ${width}px must equal viewport width`,
      ).toBe(width);

      // Brand and action containers must share the same top Y within 1px.
      const tops = await page.evaluate(() => {
        const header = document.querySelector('header');
        if (!header) return null;
        const children = Array.from(header.children) as HTMLElement[];
        if (children.length < 2) return null;
        // First child: brand container. Last child: action container.
        const brand = children[0].getBoundingClientRect();
        const action = children[children.length - 1].getBoundingClientRect();
        return { brandTop: brand.top, actionTop: action.top };
      });

      expect(tops, 'could not measure brand/action container tops').not.toBeNull();
      expect(
        Math.abs(tops!.brandTop - tops!.actionTop),
        `brand and action containers must share the same row at ${width}px; brandTop=${tops!.brandTop} actionTop=${tops!.actionTop}`,
      ).toBeLessThanOrEqual(1);
    });
  }

  test('runtime: header is single row at 1280x800 (desktop sanity)', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto('/?t=test');
    await page.waitForSelector('header', { state: 'attached', timeout: 15000 });
    await page.waitForTimeout(200);

    const box = await page.locator('header').boundingBox();
    expect(box, 'header must have a bounding box').not.toBeNull();
    expect(
      box!.height,
      `header height at 1280px must be <= 64, got ${box!.height}`,
    ).toBeLessThanOrEqual(64);
  });

  test('runtime: header is single row at 375x812 (mobile sanity) with hamburger visible', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 812 });
    await page.goto('/?t=test');
    await page.waitForSelector('header', { state: 'attached', timeout: 15000 });
    await page.waitForTimeout(200);

    const box = await page.locator('header').boundingBox();
    expect(box, 'header must have a bounding box').not.toBeNull();
    expect(
      box!.height,
      `header height at 375px must be <= 64, got ${box!.height}`,
    ).toBeLessThanOrEqual(64);

    // Hamburger toggle button is the first button inside the brand container.
    const hamburgerCount = await page
      .locator('header button[aria-label="Open sidebar"], header button[aria-label="Close sidebar"]')
      .count();
    expect(
      hamburgerCount,
      'mobile header must show the hamburger toggle',
    ).toBeGreaterThan(0);
  });
});
