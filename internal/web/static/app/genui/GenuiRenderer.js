// genui/GenuiRenderer.js -- The FIXED, TRUSTED renderer for the generative
// command center (v-genui-0).
//
// SECURITY KEYSTONE: this is a pure interpreter over a CLOSED switch of layout
// primitives + registered leaf widgets. It NEVER evaluates spec-supplied code:
// no eval, no dynamic-function construction, no dynamic import, no raw-markup
// injection of spec content. All text goes through Preact's escaping text
// nodes. An unknown type renders an inert placeholder. A spec is data; this
// code draws it.
//
// Data binds BY REFERENCE: a node's `bind`/`when`/`repeat.over` is a dotted
// identifier resolved against the secret-free `data` object (the fleet
// snapshot, projected into the binding shape by GenuiPane). The renderer never
// trusts literal data values from the spec for the rich widgets.
import { html } from 'htm/preact'

// ---- closed vocabularies (mirror the Go validator) ----------------------
const PRIMITIVES = new Set(['col', 'row', 'grid', 'stack', 'section', 'text', 'heading'])
const WIDGETS = new Set(['status-list', 'session-list', 'conductor-card', 'decision-list', 'stat'])
const GAP = { sm: '8px', md: '16px', lg: '28px' }
const TONE_CLASS = {
  ok: 'genui-tone-ok', warn: 'genui-tone-warn', danger: 'genui-tone-danger',
  info: 'genui-tone-info', neutral: 'genui-tone-neutral', '': 'genui-tone-neutral',
}
const STATUS_DOT = { running: '🟢', waiting: '🟡', idle: '⚪', error: '🔴', stopped: '⚫', absent: '⚫' }

// resolveRef walks a dotted identifier against the scope. Returns undefined on
// any miss. Pure property access only — no expression evaluation.
function resolveRef(ref, scope) {
  if (!ref) return undefined
  let cur = scope
  for (const part of String(ref).split('.')) {
    if (cur == null || typeof cur !== 'object') return undefined
    cur = cur[part]
  }
  return cur
}

// errorCard renders an inert rejection. Validation failures and unknown types
// land here; nothing partially-valid is ever drawn.
function errorCard(message) {
  return html`<div class="genui-error" data-testid="genui-error">⚠️ ${message}</div>`
}

// renderNode is the closed switch. `scope` is the binding context (the root
// data object, or a per-item object inside a repeat).
function renderNode(node, scope, depth, key) {
  if (depth > 14) return errorCard('max render depth exceeded')
  if (!node || typeof node !== 'object') return errorCard('invalid node')
  const type = node.type

  // `when`: a pre-computed boolean flag gate. Render only if truthy.
  if (node.when) {
    if (!resolveRef(node.when, scope)) return null
  }

  // `repeat`: render the template once per element of a list ref.
  if (node.repeat) {
    const list = resolveRef(node.repeat.over, scope)
    if (!Array.isArray(list)) return null
    const as = node.repeat.as || 'item'
    const capped = list.slice(0, 200)
    const items = capped.map((el, i) => {
      const childScope = Object.assign({}, scope, { [as]: el })
      return renderNode(node.repeat.template, childScope, depth + 1, `${key}-r${i}`)
    })
    // The repeat is hosted by the node's own container (type may be a layout).
    return renderContainer(node, items, scope, key)
  }

  if (PRIMITIVES.has(type)) {
    return renderPrimitive(node, scope, depth, key)
  }
  if (WIDGETS.has(type)) {
    return renderWidget(node, scope, key)
  }
  // Unknown type -> inert placeholder (the original keystone).
  return html`<div class="genui-unknown" data-testid="genui-unknown" key=${key}>unsupported widget: ${String(type)}</div>`
}

function renderChildren(node, scope, depth, key) {
  const kids = Array.isArray(node.children) ? node.children : []
  return kids.map((c, i) => renderNode(c, scope, depth + 1, `${key}-${i}`))
}

// renderContainer draws a layout primitive wrapping pre-rendered children
// (used by repeat, where children come from the template fan-out).
function renderContainer(node, children, scope, key) {
  const gap = GAP[node.gap] || GAP.md
  if (node.type === 'grid') {
    const cols = node.cols && node.cols >= 1 && node.cols <= 6 ? node.cols : 2
    return html`<div class="genui-grid" key=${key}
      style=${{ display: 'grid', gridTemplateColumns: `repeat(${cols}, minmax(0,1fr))`, gap }}>${children}</div>`
  }
  const dir = node.type === 'row' ? 'row' : 'column'
  return html`<div class="genui-stack" key=${key}
    style=${{ display: 'flex', flexDirection: dir, gap }}>${children}</div>`
}

function renderPrimitive(node, scope, depth, key) {
  const type = node.type
  if (type === 'text') {
    // Plain text leaf — Preact escapes it; spec content can never become markup.
    return html`<div class="genui-text" key=${key}>${node.text || ''}</div>`
  }
  if (type === 'heading') {
    const lvl = node.level >= 1 && node.level <= 3 ? node.level : 2
    const cls = `genui-h genui-h${lvl}`
    if (lvl === 1) return html`<h1 class=${cls} key=${key}>${node.text || ''}</h1>`
    if (lvl === 3) return html`<h3 class=${cls} key=${key}>${node.text || ''}</h3>`
    return html`<h2 class=${cls} key=${key}>${node.text || ''}</h2>`
  }
  if (type === 'section') {
    const gap = GAP[node.gap] || GAP.md
    return html`<section class="genui-section" key=${key}>
      ${node.text && html`<h2 class="genui-section-title">${node.text}</h2>`}
      <div class="genui-stack" style=${{ display: 'flex', flexDirection: 'column', gap }}>
        ${renderChildren(node, scope, depth, key)}
      </div>
    </section>`
  }
  // col / row / grid / stack
  return renderContainer(node, renderChildren(node, scope, depth, key), scope, key)
}

// ---- registered leaf widgets (vetted developer components) ---------------
function renderWidget(node, scope, key) {
  const type = node.type
  if (type === 'stat') {
    const v = resolveRef(node.bind, scope)
    const val = v == null ? '—' : v
    return html`<div class=${`genui-stat ${TONE_CLASS[node.tone] || TONE_CLASS['']}`}
      data-testid="genui-stat" key=${key}>
      <div class="genui-stat-val">${val}</div>
      <div class="genui-stat-lbl">${node.label || ''}</div>
    </div>`
  }
  if (type === 'conductor-card') {
    const cd = resolveRef(node.bind, scope) || {}
    const dot = cd.name === 'maestro' && cd.status === 'running' ? '🔵' : (STATUS_DOT[cd.status] || '⚪')
    return html`<div class="genui-card" data-testid="genui-conductor-card" data-name=${cd.name || ''} key=${key}>
      <span class="genui-card-dot">${dot}</span>
      <span class="genui-card-name">${cd.name || '—'}</span>
      <span class="genui-card-work">${cd.currentlyWorkingOn || cd.status || ''}</span>
    </div>`
  }
  if (type === 'status-list') {
    const list = resolveRef(node.bind, scope)
    const rows = Array.isArray(list) ? list : []
    return html`<div class="genui-list" data-testid="genui-status-list" key=${key}>
      ${rows.length ? rows.map((cd, i) => {
        const dot = cd.name === 'maestro' && cd.status === 'running' ? '🔵' : (STATUS_DOT[cd.status] || '⚪')
        const counts = cd.counts || {}
        const live = ['running', 'waiting', 'idle'].filter(k => counts[k]).map(k => `${counts[k]} ${k}`).join(' · ')
        return html`<div class="genui-row" data-testid="genui-status-row" data-name=${cd.name} key=${'s' + i}>
          <span class="genui-row-dot">${dot}</span>
          <span class="genui-row-name">${cd.name}</span>
          <span class="genui-row-work">${cd.currentlyWorkingOn || cd.status || ''}</span>
          <span class="genui-row-live">${live}</span>
        </div>`
      }) : html`<div class="genui-empty">no conductors</div>`}
    </div>`
  }
  if (type === 'session-list') {
    const list = resolveRef(node.bind, scope)
    const rows = Array.isArray(list) ? list : []
    return html`<div class="genui-list" data-testid="genui-session-list" key=${key}>
      ${rows.length ? rows.map((s, i) => html`
        <div class="genui-srow" data-testid="genui-session-row" data-status=${s.status} key=${'x' + i}>
          <span class="genui-row-dot">${STATUS_DOT[s.status] || '⚪'}</span>
          <span class="genui-row-name">${s.title}</span>
          <span class="genui-row-work">${s.workingOn || ''}</span>
        </div>`) : html`<div class="genui-empty">no active sessions</div>`}
    </div>`
  }
  if (type === 'decision-list') {
    const list = resolveRef(node.bind, scope)
    const rows = Array.isArray(list) ? list : []
    return html`<div class="genui-list" data-testid="genui-decision-list" key=${key}>
      ${rows.length ? rows.map((d, i) => html`
        <div class="genui-drow" data-testid="genui-decision-row" key=${'d' + i}>
          ${d.id && html`<span class="genui-drow-id">${d.id}</span>`}
          <span class="genui-drow-q">${d.question}</span>
        </div>`) : html`<div class="genui-empty">nothing waiting on you 🎉</div>`}
    </div>`
  }
  return errorCard('unhandled widget ' + type)
}

// renderSpec is the public entry. The SERVER already strictly validated the
// spec; this draws the tree. `data` is the secret-free binding object.
export function renderSpec(spec, data) {
  if (!spec || typeof spec !== 'object' || !spec.root) {
    return errorCard('no spec')
  }
  const scope = data || {}
  return html`<div class="genui-root" data-testid="genui-root" data-spec-id=${spec.specId || ''}>
    ${renderNode(spec.root, scope, 1, 'root')}
  </div>`
}
