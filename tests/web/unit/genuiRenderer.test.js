// unit/genuiRenderer.test.js -- The FIXED genui renderer is the security
// keystone of the generative command center (v-genui-0). These tests prove:
//   (1) a valid spec renders the expected DOM (spec -> widgets),
//   (2) RESHAPE: applying a different spec re-renders the whole UI to match it,
//   (3) a script-laden text field is ESCAPED, not executed (no code-eval),
//   (4) an unknown/forbidden widget type renders an inert placeholder.
import { describe, it, expect, afterEach } from 'vitest'
import { render } from 'preact'
import { renderSpec } from '../../../internal/web/static/app/genui/GenuiRenderer.js'

let host
function mount(spec, data) {
  host = document.createElement('div')
  document.body.appendChild(host)
  render(renderSpec(spec, data), host)
  return host
}

afterEach(() => {
  if (host) { render(null, host); host.remove(); host = null }
})

const fleetData = {
  totals: { running: 3, waiting: 9, idle: 8 },
  conductors: [
    { name: 'agent-deck', status: 'running', currentlyWorkingOn: 'release wave', counts: { running: 1 } },
    { name: 'innotrade', status: 'waiting', currentlyWorkingOn: '', counts: { waiting: 2 } },
  ],
  sessions: [{ title: 'fix-1431', status: 'running', workingOn: 'spawn race' }],
  decisionsWaiting: [{ id: '#1361', question: 'is conductor.enabled necessary?' }],
}

describe('genui renderer — spec to DOM', () => {
  it('renders a stat widget bound by reference to live data', () => {
    const spec = { schema: 1, specId: 'v', title: 't',
      root: { type: 'col', children: [{ type: 'stat', label: 'Running', tone: 'ok', bind: 'totals.running' }] } }
    const el = mount(spec, fleetData)
    const stat = el.querySelector('[data-testid="genui-stat"]')
    expect(stat).toBeTruthy()
    expect(stat.textContent).toContain('3')
    expect(stat.textContent).toContain('Running')
  })

  it('renders a status-list with one row per conductor', () => {
    const spec = { schema: 1, specId: 'v', title: 't', root: { type: 'status-list', bind: 'conductors' } }
    const el = mount(spec, fleetData)
    const rows = el.querySelectorAll('[data-testid="genui-status-row"]')
    expect(rows.length).toBe(2)
    expect(rows[0].getAttribute('data-name')).toBe('agent-deck')
  })

  it('renders a repeat as one card per item with per-item bind', () => {
    const spec = { schema: 1, specId: 'v', title: 't',
      root: { type: 'grid', cols: 2, repeat: { over: 'conductors', as: 'item',
        template: { type: 'conductor-card', bind: 'item' } } } }
    const el = mount(spec, fleetData)
    const cards = el.querySelectorAll('[data-testid="genui-conductor-card"]')
    expect(cards.length).toBe(2)
    expect(cards[1].getAttribute('data-name')).toBe('innotrade')
  })

  it('renders a decision-list bound by reference', () => {
    const spec = { schema: 1, specId: 'v', title: 't', root: { type: 'decision-list', bind: 'decisionsWaiting' } }
    const el = mount(spec, fleetData)
    const rows = el.querySelectorAll('[data-testid="genui-decision-row"]')
    expect(rows.length).toBe(1)
    expect(rows[0].textContent).toContain('#1361')
  })
})

describe('genui renderer — RESHAPE (same data, different spec)', () => {
  it('re-renders the whole UI to match a different spec', () => {
    const specA = { schema: 1, specId: 'A', title: 'A', root: { type: 'status-list', bind: 'conductors' } }
    const specB = { schema: 1, specId: 'B', title: 'B', root: { type: 'decision-list', bind: 'decisionsWaiting' } }
    host = document.createElement('div'); document.body.appendChild(host)
    render(renderSpec(specA, fleetData), host)
    expect(host.querySelector('[data-spec-id="A"]')).toBeTruthy()
    expect(host.querySelectorAll('[data-testid="genui-status-row"]').length).toBe(2)
    expect(host.querySelectorAll('[data-testid="genui-decision-row"]').length).toBe(0)
    render(renderSpec(specB, fleetData), host)
    expect(host.querySelector('[data-spec-id="B"]')).toBeTruthy()
    expect(host.querySelectorAll('[data-testid="genui-status-row"]').length).toBe(0)
    expect(host.querySelectorAll('[data-testid="genui-decision-row"]').length).toBe(1)
  })
})

describe('genui renderer — SECURITY keystone', () => {
  it('escapes a script-laden text field instead of executing it', () => {
    globalThis.__XSS_FIRED__ = false
    const evil = '<' + 'script>globalThis.__XSS_FIRED__=true</' + 'script><img src=x onerror=globalThis.__XSS_FIRED__=true>'
    const spec = { schema: 1, specId: 'evil', title: 't', root: { type: 'text', text: evil } }
    const el = mount(spec, {})
    expect(el.querySelectorAll('script').length).toBe(0)
    expect(el.querySelectorAll('img').length).toBe(0)
    expect(globalThis.__XSS_FIRED__).toBe(false)
    expect(el.textContent).toContain('script')
  })

  it('renders an inert placeholder for an unknown/forbidden widget type', () => {
    const spec = { schema: 1, specId: 'x', title: 't', root: { type: 'col', children: [{ type: 'iframe' }] } }
    const el = mount(spec, {})
    const ph = el.querySelector('[data-testid="genui-unknown"]')
    expect(ph).toBeTruthy()
    expect(ph.textContent).toContain('iframe')
  })

  it('contains no dynamic code-exec primitives (source audit)', async () => {
    const fs = await import('node:fs')
    const url = await import('node:url')
    const path = await import('node:path')
    const here = path.dirname(url.fileURLToPath(import.meta.url))
    const src = fs.readFileSync(
      path.resolve(here, '../../../internal/web/static/app/genui/GenuiRenderer.js'), 'utf8')
    const code = src.split('\n').map(l => l.replace(/\/\/.*$/, '')).join('\n')
    const fn = 'Func' + 'tion'
    const ev = 'ev' + 'al'
    const inner = 'inner' + 'HTML'
    const danger = 'dangerously' + 'SetInnerHTML'
    expect(code).not.toMatch(new RegExp('\\b' + ev + '\\s*\\('))
    expect(code).not.toMatch(new RegExp('new\\s+' + fn + '\\s*\\('))
    expect(code).not.toMatch(new RegExp(inner + '\\s*='))
    expect(code).not.toContain(danger)
  })
})
