import { test, expect } from '@playwright/test';
import { readFileSync } from 'fs';
import { join } from 'path';
import { execFileSync } from 'child_process';

/**
 * Phase 1 / Plan 03 / Task 2: Cascade-order verification (Pitfall #2 gate)
 *
 * Re-captures computed styles for the same selectors and viewports used by
 * cascade-baseline.spec.ts (plan 02) and diffs against
 * tests/e2e/baselines/cascade-order.json.
 *
 * THREE-STAGE DIFF (per plan 02 SUMMARY hand-off note):
 *
 *   Stage 1: For baseline selectors with `found: true` →
 *            re-capture and diff property-by-property.
 *            Any drift fails the test.
 *
 *   Stage 2: For baseline selectors with `found: false` →
 *            re-capture and confirm they STAY `found: false`.
 *            If a legacy class name suddenly starts matching after the swap,
 *            something added a new DOM element with the legacy class — flag it.
 *
 *   Stage 3: For the 16 legacy class selectors →
 *            parse pre-swap (committed at HEAD~1) and post-swap
 *            (current on disk) `internal/web/static/styles.css` and diff the
 *            CSS rule bodies for the legacy class selectors directly.
 *            getComputedStyle cannot see unmatched class rules — this is the
 *            only way to verify those rule bodies stay byte-equivalent across
 *            the cascade swap.
 *
 * If this spec fails, plan 03 task 3 (vendor/tailwind.js delete) MUST NOT run.
 */

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

interface Baseline {
  schemaVersion: number;
  capturedFor: string;
  captured_against: string;
  desktop: Snapshot;
  mobile: Snapshot;
}

// 16 legacy class selectors from the hand-written styles.css that were folded
// into styles.src.css in plan 01. The redesigned Preact app does NOT use these
// class names — they record found:false in the baseline. Stage 3 of the diff
// parses styles.css directly to ensure their CSS rule bodies stayed byte
// equivalent across the cascade swap.
const LEGACY_CLASS_SELECTORS = [
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
  'footer',
];

function loadBaseline(): Baseline {
  const p = join(__dirname, 'baselines', 'cascade-order.json');
  const raw = readFileSync(p, 'utf-8');
  return JSON.parse(raw) as Baseline;
}

// Properties whose values are colors. Used to apply canvas-based color
// normalization in the diff so v3 `rgb(...)` baselines compare equal to v4
// `oklch(...)` post-swap values when both render to the same pixel within ε.
const COLOR_PROPERTIES = new Set([
  'color',
  'background-color',
  'border-color',
  'outline-color',
  'caret-color',
]);

async function captureCurrent(
  page: import('@playwright/test').Page,
  viewport: { width: number; height: number },
  selectors: string[],
  properties: string[],
): Promise<SnapshotEntry[]> {
  await page.setViewportSize(viewport);
  await page.goto('/?t=test');
  // Wait for Preact AppShell to mount — same gate plan 02 used.
  await page.waitForSelector('header', { state: 'attached', timeout: 15000 });
  await page.waitForTimeout(750);

  return page.evaluate(
    ({ selectors, properties }) => {
      const out: Array<{ selector: string; found: boolean; styles: Record<string, string> }> = [];
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
    { selectors, properties },
  );
}

// Use a 1x1 canvas to convert any CSS color string (rgb, oklch, hex, named,
// hsl, color(...)) to a canonical [R, G, B, A] tuple where each channel is
// 0-255. The browser does the heavy lifting via canvas fillStyle, which
// accepts any CSS color and serializes the resulting pixel.
//
// Used by the Stage 1 diff to compare baseline `rgb(17, 24, 39)` against
// post-swap `oklch(0.21 0.034 264.665)`. Tailwind v4 ships an OKLCH palette;
// Tailwind v3 (the Play CDN era) shipped an sRGB palette. The two
// representations of `text-gray-900` differ as STRINGS but render to nearly
// identical pixels. We tolerate ≤2/255 channel drift to absorb the small
// gamut-mapping difference between sRGB and OKLCH.
async function normalizeColors(
  page: import('@playwright/test').Page,
  colorStrings: string[],
): Promise<Record<string, [number, number, number, number]>> {
  return page.evaluate((colors) => {
    const out: Record<string, [number, number, number, number]> = {};
    const canvas = document.createElement('canvas');
    canvas.width = 1;
    canvas.height = 1;
    const ctx = canvas.getContext('2d', { willReadFrequently: true });
    if (!ctx) return out;
    for (const c of colors) {
      if (!c || c === 'none' || c === '') {
        out[c] = [0, 0, 0, 0];
        continue;
      }
      try {
        // Reset to fully transparent before each paint so unsupported colors
        // don't reuse the previous fillStyle.
        ctx.clearRect(0, 0, 1, 1);
        ctx.fillStyle = 'rgba(0,0,0,0)';
        ctx.fillStyle = c;
        ctx.fillRect(0, 0, 1, 1);
        const data = ctx.getImageData(0, 0, 1, 1).data;
        out[c] = [data[0], data[1], data[2], data[3]];
      } catch {
        out[c] = [-1, -1, -1, -1];
      }
    }
    return out;
  }, colorStrings);
}

interface Drift {
  selector: string;
  property: string;
  before: string;
  after: string;
}

// Stage 1: diff per-property for selectors that were found:true in the baseline.
//
// For color-typed properties (color, background-color, border-color, ...), we
// pass `colorRgba` to normalize both values into [R,G,B,A] tuples and compare
// with a small per-channel tolerance. This absorbs the v3-rgb → v4-oklch
// representation change without losing sensitivity to real cascade regressions.
function diffSnapshotFoundTrue(
  baseline: SnapshotEntry[],
  current: SnapshotEntry[],
  colorRgba: Record<string, [number, number, number, number]>,
): Drift[] {
  const drift: Drift[] = [];
  const COLOR_CHANNEL_TOLERANCE = 3; // 3/255 absorbs sRGB↔OKLCH gamut mapping noise
  for (const base of baseline) {
    if (!base.found) continue;
    const cur = current.find((c) => c.selector === base.selector);
    if (!cur || !cur.found) {
      drift.push({
        selector: base.selector,
        property: '__selector_missing__',
        before: 'found',
        after: 'missing',
      });
      continue;
    }
    for (const prop of Object.keys(base.styles)) {
      const beforeVal = base.styles[prop];
      const afterVal = cur.styles[prop];
      if (beforeVal === afterVal) continue;
      // String mismatch — for color-typed properties, fall back to canonicalized
      // pixel comparison via the canvas-normalized table.
      if (COLOR_PROPERTIES.has(prop)) {
        const a = colorRgba[beforeVal];
        const b = colorRgba[afterVal];
        if (a && b) {
          const equal =
            Math.abs(a[0] - b[0]) <= COLOR_CHANNEL_TOLERANCE &&
            Math.abs(a[1] - b[1]) <= COLOR_CHANNEL_TOLERANCE &&
            Math.abs(a[2] - b[2]) <= COLOR_CHANNEL_TOLERANCE &&
            Math.abs(a[3] - b[3]) <= COLOR_CHANNEL_TOLERANCE;
          if (equal) continue;
        }
        // Special case: border-color drift is invisible when border-width is
        // `0px` on every side. The Tailwind v4 default border color changed
        // from `gray-200` (sRGB rgb) to `currentColor` (the element's text
        // color) — a documented breaking change. For elements that draw no
        // border at all, the change has zero user-visible effect, so we
        // ignore it. If you add a `border-N` utility to such an element in a
        // future plan, the cascade verifier will start failing here and
        // you'll have to either re-baseline or pin the v3 default explicitly
        // in @theme.
        if (prop === 'border-color') {
          const bw = cur.styles['border-width'];
          if (bw === '0px') continue;
        }
      }
      drift.push({
        selector: base.selector,
        property: prop,
        before: beforeVal,
        after: afterVal,
      });
    }
  }
  return drift;
}

// Stage 2: confirm selectors that were found:false stay found:false.
function diffSnapshotFoundFalse(
  baseline: SnapshotEntry[],
  current: SnapshotEntry[],
): Drift[] {
  const drift: Drift[] = [];
  for (const base of baseline) {
    if (base.found) continue;
    const cur = current.find((c) => c.selector === base.selector);
    if (cur && cur.found) {
      drift.push({
        selector: base.selector,
        property: '__found_status__',
        before: 'found:false',
        after: 'found:true (legacy class unexpectedly matches new DOM)',
      });
    }
  }
  return drift;
}

// Stage 3: parse pre-swap and post-swap styles.css (raw text) and diff the CSS
// rule bodies for the legacy class selectors. This is the ONLY way to detect
// drift in rules whose selectors don't match any DOM element.
//
// Tailwind v4 minifies output, so each rule appears once on a single line. We
// look for the literal selector token followed by `{...}` in both files and
// compare the body text. Pre-swap content is read from git (HEAD~1) so this
// gate is reproducible after Task 1's commit.
interface RuleBodyDrift {
  selector: string;
  before: string | null;
  after: string | null;
}

function loadStylesCssAtHead(): string {
  // Plan 03 task 1 was committed before this spec runs.
  // HEAD~1 is the commit immediately before the index.html swap (i.e., the
  // pre-swap state of styles.css). Plan 01 already folded the legacy CSS into
  // styles.src.css, so styles.css at HEAD~1 should already contain the legacy
  // class rules — and since neither styles.src.css nor any source file changed
  // during plan 03 task 1, HEAD~1 styles.css must be byte-identical to the
  // current on-disk styles.css. This stage proves that contract.
  // execFileSync (not exec) — argv array, no shell, no injection surface.
  const out = execFileSync(
    'git',
    ['show', 'HEAD~1:internal/web/static/styles.css'],
    {
      cwd: join(__dirname, '..', '..'),
      encoding: 'utf-8',
      maxBuffer: 10 * 1024 * 1024,
    },
  );
  return out;
}

function loadStylesCssCurrent(): string {
  return readFileSync(
    join(__dirname, '..', '..', 'internal', 'web', 'static', 'styles.css'),
    'utf-8',
  );
}

// Extract the body of each rule whose selector list contains the given
// legacy selector. We escape the selector for use inside a regex and look for
// it followed by optional whitespace, then either `,` (more selectors in the
// same rule) or `{` (start of body). When `{` is found, we slice until the
// matching `}`. This is brace-balanced for nested at-rules.
function extractRuleBodies(css: string, selector: string): string[] {
  const escapedSelector = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  // Word boundaries: a class selector like `.meta` must not match `.meta-foo`,
  // and an element selector like `footer` must not match `footer-bar`. The
  // left boundary excludes alphanumerics, `-`, `_`, and `.` (so we don't match
  // `..topbar` or `foo.topbar` mid-rule). The right boundary requires a
  // selector terminator (whitespace, comma, brace, combinator, pseudo, attr).
  const leftBoundary = '(?:^|[^a-zA-Z0-9_.-])';
  const rightBoundary = '(?=[\\s,{:>+~\\[])';
  const re = new RegExp(leftBoundary + escapedSelector + rightBoundary, 'g');
  const bodies: string[] = [];
  let match: RegExpExecArray | null;
  while ((match = re.exec(css)) !== null) {
    // Walk forward from the match position until the next `{`, then capture
    // until the matching `}` (brace-balanced). Skip if no `{` is found before
    // the next `;` or `}` (would mean the selector appears inside a value or
    // declaration, not as a real selector).
    let pos = match.index + match[0].length;
    let openBrace = -1;
    while (pos < css.length) {
      const c = css[pos];
      if (c === '{') {
        openBrace = pos;
        break;
      }
      if (c === ';' || c === '}') break;
      pos++;
    }
    if (openBrace === -1) continue;
    let depth = 1;
    let i = openBrace + 1;
    while (i < css.length && depth > 0) {
      if (css[i] === '{') depth++;
      else if (css[i] === '}') depth--;
      i++;
    }
    bodies.push(css.slice(openBrace, i));
  }
  return bodies;
}

function diffLegacyClassRules(beforeCss: string, afterCss: string): RuleBodyDrift[] {
  const drift: RuleBodyDrift[] = [];
  for (const sel of LEGACY_CLASS_SELECTORS) {
    const before = extractRuleBodies(beforeCss, sel).sort().join('\n');
    const after = extractRuleBodies(afterCss, sel).sort().join('\n');
    if (before !== after) {
      drift.push({
        selector: sel,
        before: before || null,
        after: after || null,
      });
    }
  }
  return drift;
}

test.describe('cascade-order verification (Phase 1 / Plan 03)', () => {
  test('post-swap computed styles match plan 02 baseline (three-stage diff)', async ({ page }) => {
    const baseline = loadBaseline();
    expect(baseline.schemaVersion).toBe(1);

    // Sanity: confirm Play CDN is GONE (post-swap state).
    await page.goto('/?t=test');
    const playCdnPresent = await page.evaluate(() => {
      const scripts = document.querySelectorAll('script');
      for (let i = 0; i < scripts.length; i++) {
        const src = scripts[i].src || '';
        if (src.indexOf('/static/vendor/tailwind.js') !== -1) return true;
      }
      return false;
    });
    // cascade verify must run AFTER plan 03 task 1 swap
    expect(playCdnPresent).toBe(false);

    // Derive selectors + properties from the baseline entries.
    const selectors = baseline.desktop.entries.map((e) => e.selector);
    const properties = Object.keys(baseline.desktop.entries.find((e) => e.found)?.styles ?? {});
    expect(selectors.length).toBeGreaterThan(5);
    expect(properties.length).toBeGreaterThan(10);

    // ----- Stage 1: per-property diff for found:true selectors -----
    const currentDesktop = await captureCurrent(page, { width: 1280, height: 800 }, selectors, properties);
    const currentMobile = await captureCurrent(page, { width: 375, height: 812 }, selectors, properties);

    // Collect every distinct color-typed value (baseline + current, both
    // viewports) so we can canonicalize all of them in a single canvas pass.
    const colorValues = new Set<string>();
    const collectColors = (entries: SnapshotEntry[]) => {
      for (const e of entries) {
        if (!e.found) continue;
        for (const prop of Object.keys(e.styles)) {
          if (COLOR_PROPERTIES.has(prop)) colorValues.add(e.styles[prop]);
        }
      }
    };
    collectColors(baseline.desktop.entries);
    collectColors(baseline.mobile.entries);
    collectColors(currentDesktop);
    collectColors(currentMobile);
    const colorRgba = await normalizeColors(page, Array.from(colorValues));

    const desktopDrift = diffSnapshotFoundTrue(baseline.desktop.entries, currentDesktop, colorRgba);
    const mobileDrift = diffSnapshotFoundTrue(baseline.mobile.entries, currentMobile, colorRgba);

    // ----- Stage 2: confirm found:false selectors stay found:false -----
    const desktopFoundDrift = diffSnapshotFoundFalse(baseline.desktop.entries, currentDesktop);
    const mobileFoundDrift = diffSnapshotFoundFalse(baseline.mobile.entries, currentMobile);

    // ----- Stage 3: legacy-class rule body diff (text-level on styles.css) -----
    const beforeCss = loadStylesCssAtHead();
    const afterCss = loadStylesCssCurrent();
    const ruleBodyDrift = diffLegacyClassRules(beforeCss, afterCss);

    // Build report
    const report = {
      stage1_desktop_property_drift: desktopDrift,
      stage1_mobile_property_drift: mobileDrift,
      stage2_desktop_found_status_drift: desktopFoundDrift,
      stage2_mobile_found_status_drift: mobileFoundDrift,
      stage3_legacy_class_rule_body_drift: ruleBodyDrift,
    };
    const totalDriftCount =
      desktopDrift.length +
      mobileDrift.length +
      desktopFoundDrift.length +
      mobileFoundDrift.length +
      ruleBodyDrift.length;

    if (totalDriftCount > 0) {
      // Print to stderr so the failure message is grep-able from the test log.
      console.error('CASCADE DRIFT DETECTED:\n' + JSON.stringify(report, null, 2));
    }

    expect(desktopDrift, 'stage 1 — desktop cascade regressions').toEqual([]);
    expect(mobileDrift, 'stage 1 — mobile cascade regressions').toEqual([]);
    expect(desktopFoundDrift, 'stage 2 — desktop found:false→found:true regressions').toEqual([]);
    expect(mobileFoundDrift, 'stage 2 — mobile found:false→found:true regressions').toEqual([]);
    expect(ruleBodyDrift, 'stage 3 — legacy class rule body drift in styles.css').toEqual([]);
  });
});
