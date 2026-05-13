// e2e/optimistic-ui.spec.js -- B15 (REGRESSION v1.8 optimistic UI revert).
//
// SessionRow.handleMutation (SessionRow.js:47-60) flips the dot to a pending
// status synchronously, fires the mutation, and on error reverts the dot
// AND fires an error toast via addToast. The success path lets SSE deliver
// the real status; the optimistic override clears after 3s regardless.
//
// The interesting branch is the error path: it's the one the user sees when
// the server rejects (read-only mode, rate limit, host process death). v1.8
// shipped a regression where the optimistic dot stuck even when the server
// returned 500 — the row looked stopped forever until the user refreshed.
// This spec uses Playwright route interception to force a 500 and asserts
// the dot reverts AND a toast appears.

import { test, expect } from '@playwright/test'

test.beforeEach(({}, testInfo) => {
  test.skip(
    testInfo.project.name !== 'chromium-desktop',
    'desktop-only: sidebar overlay is closed by default at tablet/phone viewports',
  )
})

async function gotoFreshApp(page) {
  await page.goto('/healthz')
  await page.evaluate(() => {
    try { localStorage.clear() } catch (_) {}
  })
  await page.goto('/')
  await page.waitForFunction(() => window.__preactSessionListActive === true, {
    timeout: 5000,
  })
}

async function resetFixture(request) {
  const res = await request.post('/__fixture/reset')
  expect(res.status()).toBe(204)
}

function sidebarSession(page, sessionId) {
  return page.locator('aside').locator(`[data-session-id="${sessionId}"]`)
}

async function dotClass(page, sessionId) {
  return (await sidebarSession(page, sessionId)
    .locator('span.rounded-full')
    .first()
    .getAttribute('class')) || ''
}

function toolbarButton(row, label) {
  return row.getByRole('button', { name: new RegExp(label, 'i') })
}

test.describe('SessionRow optimistic UI (B15, REGRESSION v1.8)', () => {
  test.beforeEach(async ({ request }) => {
    await resetFixture(request)
  })

  test('stop on running session flips dot to stopped optimistically before server replies', async ({
    page,
  }) => {
    await gotoFreshApp(page)
    // sess-002 starts running per the fixture seed.
    const row = sidebarSession(page, 'sess-002')
    await expect(row).toBeVisible()
    expect(await dotClass(page, 'sess-002')).toMatch(/bg-tn-green/)

    // Delay the server's response so we can observe the optimistic dot
    // in its in-flight window. Without the delay, SSE typically delivers
    // the real `stopped` status before we read the DOM, making the test
    // unable to distinguish optimistic from confirmed.
    await page.route('**/api/sessions/sess-002/stop', async (route) => {
      await new Promise((r) => setTimeout(r, 600))
      await route.continue()
    })

    // Hover the row to reveal the toolbar; the Stop button appears for
    // running sessions only (SessionRow.js:144-155).
    await row.hover()
    await toolbarButton(row, 'Stop session').click()

    // While the mutation is in flight, the dot must already show the
    // pending `stopped` color (bg-tn-muted/50, no pulse) — that's the
    // optimistic override. Poll quickly to catch it before SSE confirms.
    await expect
      .poll(() => dotClass(page, 'sess-002'), { timeout: 2000, intervals: [50, 100, 200] })
      .toMatch(/bg-tn-muted\/50/)
  })

  test('on server 500, optimistic dot reverts and an error toast appears', async ({
    page,
  }) => {
    await gotoFreshApp(page)
    const row = sidebarSession(page, 'sess-002')
    await expect(row).toBeVisible()
    expect(await dotClass(page, 'sess-002')).toMatch(/bg-tn-green/)

    // Intercept the stop mutation and return 500 with a body the apiFetch
    // error-message extractor can surface (matches api.js error shape).
    await page.route('**/api/sessions/sess-002/stop', async (route) => {
      await route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({ error: { message: 'simulated stop failure' } }),
      })
    })

    await row.hover()
    await toolbarButton(row, 'Stop session').click()

    // The toast appears with the server-supplied message. role="alert" is
    // the assertive container; error toasts mount under it.
    const toast = page
      .locator('[role="alert"]')
      .filter({ hasText: 'simulated stop failure' })
    await expect(toast, 'error toast should surface server error.message').toBeVisible()

    // The optimistic override clears immediately on the catch branch
    // (SessionRow.js:52-54 setOptimisticStatus(null)), so the dot reverts
    // to the real status (still running, because the server didn't
    // actually transition). Poll because the apiFetch catch is async.
    await expect
      .poll(() => dotClass(page, 'sess-002'), { timeout: 3000 })
      .toMatch(/bg-tn-green/)
  })

  test('error toast persists until clicked (no auto-dismiss for type=error)', async ({
    page,
  }) => {
    await gotoFreshApp(page)
    await page.route('**/api/sessions/sess-002/stop', async (route) => {
      await route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({ error: { message: 'persistent error' } }),
      })
    })

    const row = sidebarSession(page, 'sess-002')
    await row.hover()
    await toolbarButton(row, 'Stop session').click()

    const toast = page
      .locator('[role="alert"]')
      .filter({ hasText: 'persistent error' })
    await expect(toast).toBeVisible()

    // info/success auto-dismiss after 5s; error must NOT — Toast.js:49-51.
    // Wait longer than that and re-assert.
    await page.waitForTimeout(5500)
    await expect(toast, 'error toast must NOT auto-dismiss').toBeVisible()
  })
})
