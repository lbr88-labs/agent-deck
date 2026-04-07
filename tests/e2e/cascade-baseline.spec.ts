import { test, expect } from '@playwright/test';
import { writeFileSync, mkdirSync, existsSync } from 'fs';
import { join, dirname } from 'path';

/**
 * Phase 1 / Plan 02: Cascade-order baseline capture (Pitfall #2 mitigation)
 *
 * This spec runs against the CURRENT Play CDN page state (vendor/tailwind.js
 * still loaded). It captures computed styles for cascade-sensitive selectors
 * and writes them to tests/e2e/baselines/cascade-order.json.
 *
 * Plan 03 will run an identical capture AFTER swapping to the compiled CSS,
 * diff against this baseline, and fail the build on any drift.
 *
 * DO NOT delete or regenerate this baseline without re-running plan 03.
 */

const SELECTORS = [
  'body',
  '.topbar',
  '.brand',
  '.menu-toggle',
  '.menu-panel',
  '.terminal-panel',
  '.terminal-shell',
  '.terminal-canvas',
  '.terminal-fallback',
  '.menu-filter',
  '.menu-list',
  '.menu-item.group',
  '.menu-item.session',
  '.status-dot',
  '.meta',
  '.costs-btn',
];

const CASCADE_PROPERTIES = [
  'font-family',
  'font-size',
  'color',
  'background-color',
  'padding-top',
  'padding-right',
  'padding-bottom',
  'padding-left',
  'margin-top',
  'margin-right',
  'margin-bottom',
  'margin-left',
  'width',
  'min-width',
  'flex-grow',
  'flex-shrink',
  'border-color',
  'border-width',
];

interface SnapshotEntry {
  selector: string;
  found: boolean;
  styles: Record<string, string>;
}

interface Snapshot {
  capturedAt: string;
  source: string;
  viewport: { width: number; height: number };
  entries: SnapshotEntry[];
}

async function captureSnapshot(
  page: import('@playwright/test').Page,
  viewport: { width: number; height: number },
  source: string,
): Promise<Snapshot> {
  await page.setViewportSize(viewport);
  await page.goto('/?t=test');
  // Wait for either Preact to mount OR the static fallback to render.
  await page.waitForSelector('body', { state: 'attached' });
  await page.waitForTimeout(500); // allow Tailwind Play CDN to inject runtime styles

  const entries: SnapshotEntry[] = await page.evaluate(
    ({ selectors, properties }) => {
      const out: SnapshotEntry[] = [];
      for (const sel of selectors) {
        const el = document.querySelector(sel);
        if (!el) {
          out.push({ selector: sel, found: false, styles: {} });
          continue;
        }
        const cs = window.getComputedStyle(el as Element);
        const styles: Record<string, string> = {};
        for (const prop of properties) {
          styles[prop] = cs.getPropertyValue(prop);
        }
        out.push({ selector: sel, found: true, styles });
      }
      return out;
    },
    { selectors: SELECTORS, properties: CASCADE_PROPERTIES },
  );

  return {
    capturedAt: new Date().toISOString(),
    source,
    viewport,
    entries,
  };
}

test.describe('cascade-order baseline (Phase 1 / Plan 02)', () => {
  test('captures computed styles at desktop and mobile while Play CDN is loaded', async ({ page }) => {
    const desktop = await captureSnapshot(page, { width: 1280, height: 800 }, 'play-cdn');
    const mobile = await captureSnapshot(page, { width: 375, height: 812 }, 'play-cdn');

    // Sanity: confirm we are still in the Play CDN era (vendor/tailwind.js loaded).
    // If this is false, the test was run AFTER plan 03 — abort to avoid clobbering the baseline.
    const playCdnPresent = await page.evaluate(() => {
      return Array.from(document.querySelectorAll('script')).some(
        (s) => (s as HTMLScriptElement).src.includes('/static/vendor/tailwind.js'),
      );
    });
    expect(playCdnPresent, 'cascade baseline must be captured BEFORE plan 03 swaps to compiled CSS').toBe(true);

    // At least body, .topbar, .menu-panel, .terminal-panel must exist on the rendered page.
    const requiredFound = desktop.entries.filter((e) =>
      ['body', '.topbar', '.menu-panel', '.terminal-panel'].includes(e.selector),
    );
    for (const r of requiredFound) {
      expect(r.found, `required selector ${r.selector} not found in DOM`).toBe(true);
    }

    const baseline = {
      schemaVersion: 1,
      capturedFor: 'phase 1 / plan 02 — pre-Tailwind-precompile baseline',
      captured_against: 'play-cdn-plus-static-styles-css',
      note: 'Dual-layer state: Play CDN runtime is dominant, /static/styles.css is the secondary hand-written layer. Plan 03 cascade-verify diffs against this snapshot AFTER swapping styles.css to compiled Tailwind output.',
      pitfallReference: 'PITFALLS.md Pitfall #2 (cascade order regressions)',
      doNotEdit: 'Plan 03 diffs against this file. Regenerate only with explicit phase 1 re-run.',
      desktop,
      mobile,
    };

    const out = join(__dirname, 'baselines', 'cascade-order.json');
    if (!existsSync(dirname(out))) mkdirSync(dirname(out), { recursive: true });
    writeFileSync(out, JSON.stringify(baseline, null, 2) + '\n', 'utf-8');
  });
});
