// e2e/command-center.spec.js -- Command Center pane end-to-end coverage.
//
// The Command Center (internal/web/static/app/panes/CommandCenterPane.js) is the
// embedded, live, two-way fleet god-view. It renders the synthesized snapshot
// from GET /events/command-center (SSE), groups sessions per conductor, filters
// OUT error/stopped sessions (the noise the user explicitly rejected), surfaces
// honest-status substates, and routes typed instructions via POST
// /api/command-center/ask. See conductor/agent-deck/COMMAND-CENTER-DESIGN.md.
//
// Grounded in the fixture seed (tests/web/fixtures/cmd/web-fixture/main.go):
//   sess-001 "agent-deck"     status=idle    group=work           (IsConductor)
//   sess-002 "frontend"       status=running group=work
//   sess-003 "innotrade-api"  status=idle    group=work/innotrade
//   sess-004 "scratch"        status=idle    group=personal
// → active CHILD totals at cold load (the conductor row itself is excluded from
//   the per-child tally): running=1 (sess-002), waiting=0, idle=2 (sess-003/004).
import { test, expect } from '@playwright/test'

async function openCommandCenter(page) {
  await page.goto('/')
  // The top tab strip is hidden on phone-class viewports (≤720px), which use
  // the bottom MobileTabs bar instead. Click whichever control is visible so
  // the flagship view is reachable on every viewport.
  const viewport = page.viewportSize()
  if (viewport && viewport.width < 768) {
    await page.locator('[data-testid="mobile-tab-command-center"]').click()
  } else {
    await page.locator('.top-tab', { hasText: 'Command Center' }).click()
  }
  await expect(page.locator('[data-testid="command-center-pane"]')).toBeVisible({ timeout: 5000 })
}

test.describe('command center pane', () => {
  test.beforeEach(async ({ request }) => {
    await request.post('/__fixture/reset')
  })

  test('renders live with totals derived from the fixture seed', async ({ page }) => {
    await openCommandCenter(page)
    // The live indicator and totals hydrate from the first SSE snapshot.
    await expect(page.locator('[data-testid="cc-live"]')).toContainText('live', { timeout: 5000 })
    // Seed (child sessions only; the conductor row is tallied separately):
    // sess-002 running; sess-003/004 idle; nothing waiting. toContainText
    // retries through the initial empty render before the SSE snapshot lands.
    await expect(page.locator('[data-testid="cc-totals"]')).toContainText('1 running', { timeout: 5000 })
    await expect(page.locator('[data-testid="cc-totals"]')).toContainText('0 waiting')
    await expect(page.locator('[data-testid="cc-totals"]')).toContainText('2 idle')
  })

  test('shows conductor rows and the two-way input bar', async ({ page }) => {
    await openCommandCenter(page)
    // At least one conductor row renders (groups: work, work/innotrade, personal).
    await expect(page.locator('[data-testid="cc-conductor"]').first()).toBeVisible({ timeout: 5000 })
    // The two-way input bar and target picker are present.
    await expect(page.locator('[data-testid="cc-input"]')).toBeVisible()
    await expect(page.locator('[data-testid="cc-target"]')).toBeVisible()
    // Maestro is the default routing target.
    await expect(page.locator('[data-testid="cc-target"]')).toContainText('Maestro')
  })

  test('updates live via SSE without a reload', async ({ page, request }) => {
    await openCommandCenter(page)
    await expect(page.locator('[data-testid="cc-totals"]')).toContainText('1 running', { timeout: 5000 })

    // Drive a TUI-side transition: sess-002 running -> waiting. This must flow
    // through /events/command-center and re-render the panel WITHOUT a reload.
    await request.post('/__fixture/session/sess-002/status?to=waiting')

    await expect(page.locator('[data-testid="cc-totals"]')).toContainText('0 running', { timeout: 6000 })
    await expect(page.locator('[data-testid="cc-totals"]')).toContainText('1 waiting')
    // We never called page.goto/reload — the SSE feed alone drove the change.
  })

  test('filters OUT error/stopped sessions (the rejected noise)', async ({ page, request }) => {
    await openCommandCenter(page)
    await expect(page.locator('[data-testid="command-center-pane"]')).toBeVisible({ timeout: 5000 })

    // Force sess-002 to error. It must NOT appear in any conductor's session
    // list — error is filtered by construction.
    await request.post('/__fixture/session/sess-002/status?to=error')

    // Give the SSE feed a moment to push the change.
    await expect(page.locator('[data-testid="cc-totals"]')).toContainText('0 running', { timeout: 6000 })

    // Expand every conductor row, then assert no rendered session row is error/stopped.
    const heads = page.locator('.cc-cd-head')
    const n = await heads.count()
    for (let i = 0; i < n; i++) await heads.nth(i).click()

    const errorRows = page.locator('[data-testid="cc-session"][data-status="error"]')
    const stoppedRows = page.locator('[data-testid="cc-session"][data-status="stopped"]')
    await expect(errorRows).toHaveCount(0)
    await expect(stoppedRows).toHaveCount(0)
  })

  test('typing routes through the ask endpoint and reflects status', async ({ page }) => {
    await openCommandCenter(page)
    await expect(page.locator('[data-testid="cc-input"]')).toBeVisible({ timeout: 5000 })

    await page.locator('[data-testid="cc-input"] textarea').fill('approve #1361 — keep conductor.enabled')
    await page.locator('[data-testid="cc-send"]').click()

    // The status line reflects the routing outcome. In the fixture there is no
    // live agent behind the target, so the supported `session send` path can't
    // deliver and the handler returns an honest error — the point is that the
    // request went through the /ask endpoint and the UI reflected a result,
    // not a silent no-op. Either ✓ routed or ✗ deliver is a valid reflection.
    await expect(page.locator('[data-testid="cc-status"]')).not.toHaveText('ready', { timeout: 6000 })
  })
})

// ---- Command Center v2 (Phase 1) ---------------------------------------------
// Per-project detail pages (live aggregation), comment-on-anything with context,
// keyboard nav, acknowledgements. The fixture seeds a conductor artifact tree
// (web-fixture/main.go seedConductorArtifacts): the "work" conductor row carries
// a status.json headline + an outputs/summary.md doc + a live "frontend" session.
test.describe('command center v2 — detail pages', () => {
  test.beforeEach(async ({ request }) => {
    await request.post('/__fixture/reset')
  })

  test('opens a per-project detail page that aggregates docs + sessions live', async ({ page }) => {
    await openCommandCenter(page)
    // Each conductor row exposes an "open →" affordance into its detail page.
    const workRow = page.locator('[data-testid="cc-conductor"][data-name="work"]')
    await expect(workRow).toBeVisible({ timeout: 5000 })
    await workRow.locator('[data-testid="cc-open-detail"]').click()

    // The detail page mounts and aggregates the four feeds.
    const detail = page.locator('[data-testid="command-center-detail-pane"]')
    await expect(detail).toBeVisible({ timeout: 5000 })
    // (a) status/activity headline from status.json
    await expect(page.locator('[data-testid="ccd-headline"]')).toContainText('work conductor', { timeout: 5000 })
    // (b) produced doc rendered inline (markdown → HTML)
    const doc = page.locator('[data-testid="ccd-doc"]').first()
    await expect(doc).toBeVisible({ timeout: 5000 })
    await expect(doc.locator('.ccd-doc-body.md strong')).toContainText('inline')
    // (c) live session list (the frontend child of group work)
    await expect(page.locator('[data-testid="ccd-session"]').first()).toBeVisible()
    // The input box is pre-scoped to this conductor — no target picker.
    await expect(page.locator('[data-testid="ccd-input"] .ccd-scope')).toContainText('conductor-work')
  })

  test('Esc and the back button return to the god-view list', async ({ page }) => {
    await openCommandCenter(page)
    await page.locator('[data-testid="cc-conductor"][data-name="work"] [data-testid="cc-open-detail"]').click()
    await expect(page.locator('[data-testid="command-center-detail-pane"]')).toBeVisible({ timeout: 5000 })

    // Esc backs out to the list.
    await page.keyboard.press('Escape')
    await expect(page.locator('[data-testid="command-center-pane"]')).toBeVisible({ timeout: 5000 })
    await expect(page.locator('[data-testid="command-center-detail-pane"]')).toHaveCount(0)

    // Re-open, then use the explicit back button.
    await page.locator('[data-testid="cc-conductor"][data-name="work"] [data-testid="cc-open-detail"]').click()
    await expect(page.locator('[data-testid="command-center-detail-pane"]')).toBeVisible({ timeout: 5000 })
    await page.locator('[data-testid="ccd-back"]').click()
    await expect(page.locator('[data-testid="command-center-pane"]')).toBeVisible({ timeout: 5000 })
  })

  test('digit-jump opens the matching conductor detail page', async ({ page }) => {
    await openCommandCenter(page)
    await expect(page.locator('[data-testid="cc-conductor"]').first()).toBeVisible({ timeout: 5000 })
    // Pressing "1" jumps to + opens the first conductor row's detail page.
    await page.keyboard.press('1')
    await expect(page.locator('[data-testid="command-center-detail-pane"]')).toBeVisible({ timeout: 5000 })
  })

  test('commenting on a decision prefills the input with its context', async ({ page }) => {
    await openCommandCenter(page)
    // The fixture's OPEN-ITEMS §D seeds one decision (#777). Comment on it.
    const decision = page.locator('[data-testid="cc-decision"]').first()
    await expect(decision).toBeVisible({ timeout: 5000 })
    await decision.locator('.cc-cmt').click()
    // The input is prefilled with a context-scoped "re <id>:" reference.
    await expect(page.locator('[data-testid="cc-input"] textarea')).toHaveValue(/^re /)
  })

  test('asking shows an acknowledgement (never silence)', async ({ page }) => {
    await openCommandCenter(page)
    await page.locator('[data-testid="cc-input"] textarea').fill('kick off the release')
    await page.locator('[data-testid="cc-send"]').click()
    // An ack appears immediately with the "got it → routed" progression
    // (Addendum 4), independent of whether a live agent is behind the target.
    await expect(page.locator('[data-testid="cc-acks"]')).toBeVisible({ timeout: 6000 })
    await expect(page.locator('[data-testid="cc-ack"]').first()).toContainText('kick off the release')
  })
})
