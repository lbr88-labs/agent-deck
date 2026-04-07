import { test, expect } from '@playwright/test';
import { readFileSync } from 'fs';
import { join } from 'path';

/**
 * Phase 3 / Plan 01 / Task 1: BUG #5 / LAYT-02 regression test
 *
 * Asserts that every visible SessionRow title span and every visible GroupRow
 * name span has a non-empty native `title` attribute, and that the tooltip
 * text equals the visible text. Also asserts no custom tooltip library was
 * introduced (native `title` only, per 03-CONTEXT.md).
 *
 * Root cause (LOCKED per 03-CONTEXT.md): SessionRow.js line 99 and
 * GroupRow.js line 55 render `<span class="flex-1 truncate min-w-0">...</span>`
 * with no `title=` attribute. Truncated names silently hide their full text;
 * hovering shows nothing.
 *
 * Fix (LOCKED per 03-CONTEXT.md):
 *   - SessionRow.js: add `title=${session.title || session.id}` to the title span.
 *   - GroupRow.js: add `title=${group.name || group.path}` to the name span.
 *   Always on, NOT conditional on actual truncation. Never introduce a custom
 *   tooltip library (anti-feature per project north star).
 *
 * TDD ORDER: this spec is committed in FAILING state in Task 1, then the fix
 * lands in Task 2, flipping the spec to green.
 *
 * STRUCTURAL FALLBACK: if no fixture sessions render, the runtime DOM walks
 * skip. In that case the two file-read structural tests ALWAYS run and
 * provide the failing-before-fix guarantee.
 */

interface MissingTitleHit {
  role: 'session' | 'group';
  identifier: string;
  title: string | null;
  text: string;
}

async function gotoAndWait(
  page: import('@playwright/test').Page,
  viewport: { width: number; height: number },
): Promise<void> {
  await page.setViewportSize(viewport);
  await page.goto('/?t=test');
  await page.waitForSelector('header', { state: 'attached', timeout: 15000 });
  await page
    .waitForSelector('#preact-session-list', { state: 'attached', timeout: 15000 })
    .catch(() => {
      // Empty state — structural tests still cover the TDD gate.
    });
  await page.waitForTimeout(500);
}

async function findMissingSessionTooltips(
  page: import('@playwright/test').Page,
): Promise<MissingTitleHit[]> {
  return page.evaluate(() => {
    const out: Array<{ role: 'session'; identifier: string; title: string | null; text: string }> = [];
    const buttons = document.querySelectorAll('button[data-session-id]');
    for (let i = 0; i < buttons.length; i++) {
      const btn = buttons[i] as HTMLElement;
      const id = btn.getAttribute('data-session-id') || '(unknown)';
      const titleSpan = btn.querySelector('span.truncate') as HTMLElement | null;
      if (!titleSpan) {
        out.push({ role: 'session', identifier: id, title: null, text: '(no .truncate span)' });
        continue;
      }
      const title = titleSpan.getAttribute('title');
      if (!title || title.trim() === '') {
        out.push({
          role: 'session',
          identifier: id,
          title,
          text: (titleSpan.textContent || '').trim().slice(0, 80),
        });
      }
    }
    return out;
  });
}

async function findMissingGroupTooltips(
  page: import('@playwright/test').Page,
): Promise<MissingTitleHit[]> {
  return page.evaluate(() => {
    const out: Array<{ role: 'group'; identifier: string; title: string | null; text: string }> = [];
    const buttons = document.querySelectorAll('button[aria-expanded]');
    for (let i = 0; i < buttons.length; i++) {
      const btn = buttons[i] as HTMLElement;
      const label = (btn.textContent || '(unknown)').trim().slice(0, 40);
      const titleSpan = btn.querySelector('span.truncate') as HTMLElement | null;
      // Only GroupRow buttons have a .truncate child span. Skip other
      // aria-expanded buttons (profile dropdown, info popover, etc.).
      if (!titleSpan) continue;
      const title = titleSpan.getAttribute('title');
      if (!title || title.trim() === '') {
        out.push({
          role: 'group',
          identifier: label,
          title,
          text: (titleSpan.textContent || '').trim().slice(0, 80),
        });
      }
    }
    return out;
  });
}

async function findTitleMismatches(
  page: import('@playwright/test').Page,
): Promise<Array<{ kind: string; title: string; text: string }>> {
  return page.evaluate(() => {
    const hits: Array<{ kind: string; title: string; text: string }> = [];
    const spans = document.querySelectorAll(
      'button[data-session-id] span.truncate, button[aria-expanded] span.truncate',
    );
    for (let i = 0; i < spans.length; i++) {
      const el = spans[i] as HTMLElement;
      const title = (el.getAttribute('title') || '').trim();
      const text = (el.textContent || '').trim();
      if (!title) continue;
      // Allow title to equal the visible text (normal case) or a fallback id/path
      // (when title || id pattern — the visible text IS the fallback and matches).
      if (title !== text) {
        hits.push({
          kind: el.closest('button[data-session-id]') ? 'session' : 'group',
          title,
          text,
        });
      }
    }
    return hits;
  });
}

function formatHits(hits: MissingTitleHit[]): string {
  if (hits.length === 0) return '(none)';
  return hits
    .map((h) => `  ${h.role}[${h.identifier}] title=${JSON.stringify(h.title)} text=${JSON.stringify(h.text)}`)
    .join('\n');
}

test.describe('BUG #5 / LAYT-02 — native title tooltip on truncated session/group names', () => {
  test('desktop 1280x800: every session title span has a non-empty title attribute', async ({ page }) => {
    await gotoAndWait(page, { width: 1280, height: 800 });
    const count = await page.locator('button[data-session-id]').count();
    test.skip(count === 0, 'no fixture sessions — relying on structural test + Phase 8 fixtures');
    const missing = await findMissingSessionTooltips(page);
    expect(
      missing,
      `desktop: ${missing.length} session title spans have no title attribute — BUG #5.\n${formatHits(missing)}`,
    ).toEqual([]);
  });

  test('desktop 1280x800: every group title span has a non-empty title attribute', async ({ page }) => {
    await gotoAndWait(page, { width: 1280, height: 800 });
    const count = await page.locator('button[aria-expanded]').count();
    test.skip(count === 0, 'no fixture groups — relying on structural test + Phase 8 fixtures');
    const missing = await findMissingGroupTooltips(page);
    expect(
      missing,
      `desktop: ${missing.length} group title spans have no title attribute — BUG #5.\n${formatHits(missing)}`,
    ).toEqual([]);
  });

  test('desktop 1280x800: title attribute equals visible text on every span that has one', async ({ page }) => {
    await gotoAndWait(page, { width: 1280, height: 800 });
    const rowCount = await page.locator('button[data-session-id], button[aria-expanded]').count();
    test.skip(rowCount === 0, 'no fixture rows — deferred to Phase 8 fixtures');
    const mismatches = await findTitleMismatches(page);
    expect(
      mismatches,
      `title attribute must equal visible text; got ${mismatches.length} mismatches:\n${JSON.stringify(mismatches, null, 2)}`,
    ).toEqual([]);
  });

  test('structural: no custom tooltip library introduced (no role=tooltip, no data-tooltip)', async ({ page }) => {
    await gotoAndWait(page, { width: 1280, height: 800 });
    const count = await page.evaluate(() => {
      return (
        document.querySelectorAll('[role="tooltip"], [data-tooltip]').length
      );
    });
    expect(
      count,
      'custom tooltip element found in DOM — LAYT-02 locks native title only, NO custom tooltip library',
    ).toBe(0);
  });

  // STRUCTURAL FALLBACK — always runs, fails before fix, passes after.
  test('structural: SessionRow.js truncate span has title attribute', () => {
    const p = join(__dirname, '..', '..', '..', 'internal', 'web', 'static', 'app', 'SessionRow.js');
    const src = readFileSync(p, 'utf-8');
    const titleRe = /title=\$\{session\.title \|\| session\.id\}/;
    expect(
      titleRe.test(src),
      'SessionRow.js title span is missing title=${session.title || session.id} — BUG #5. Add it to the <span class="flex-1 truncate min-w-0"> element.',
    ).toBe(true);
  });

  test('structural: GroupRow.js truncate span has title attribute', () => {
    const p = join(__dirname, '..', '..', '..', 'internal', 'web', 'static', 'app', 'GroupRow.js');
    const src = readFileSync(p, 'utf-8');
    const titleRe = /title=\$\{group\.name \|\| group\.path\}/;
    expect(
      titleRe.test(src),
      'GroupRow.js title span is missing title=${group.name || group.path} — BUG #5. Add it to the <span class="flex-1 truncate min-w-0 text-left"> element.',
    ).toBe(true);
  });
});
