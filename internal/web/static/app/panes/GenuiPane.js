// panes/GenuiPane.js -- The generative command center (v-genui-0).
//
// THE MAGIC: the SAME live fleet data, rendered by 3 different hand-authored
// whole-UI SPECS. Switching specs reshapes the ENTIRE view live — no rebuild,
// no new code. The fixed engine (GenuiRenderer + the Go validator) is what
// makes that safe: specs are inert DATA the renderer interprets.
//
// Data flows in by REFERENCE: this pane projects the live commandCenterSignal
// snapshot (delivered over the existing /events/command-center SSE) into the
// secret-free binding object the spec refs resolve against. No LLM in v0.
import { html } from 'htm/preact'
import { useState, useEffect } from 'preact/hooks'
import { commandCenterSignal, connectionSignal, authTokenSignal } from '../state.js'
import { renderSpec } from '../genui/GenuiRenderer.js'

// fetchJSON loads a server-validated spec (or the view list). The token goes in
// the Authorization header so it never leaks to logs/Referer.
async function fetchJSON(path) {
  const headers = { Accept: 'application/json' }
  const tok = authTokenSignal.value
  if (tok) headers['Authorization'] = 'Bearer ' + tok
  const res = await fetch(path, { headers })
  if (!res.ok) throw new Error('HTTP ' + res.status)
  return res.json()
}

async function fetchSpec(id) {
  const headers = { Accept: 'application/json' }
  const tok = authTokenSignal.value
  if (tok) headers['Authorization'] = 'Bearer ' + tok
  const res = await fetch('/api/command-center/genui/spec/' + encodeURIComponent(id), { headers })
  if (!res.ok) throw new Error('HTTP ' + res.status)
  return res.json()
}

// bindData projects the live snapshot into the shape the spec refs expect.
// Every value here is non-secret (status, names, counts, decision questions) —
// the same projected slice the fixed Command Center already shows.
function bindData(snap) {
  if (!snap) return { totals: {}, conductors: [], sessions: [], decisionsWaiting: [], stuckSessions: [] }
  const conductors = Array.isArray(snap.conductors) ? snap.conductors : []
  const sessions = []
  const stuckSessions = []
  for (const cd of conductors) {
    for (const s of (cd.sessions || [])) {
      sessions.push(s)
      if (s.status === 'error' || s.status === 'stopped') stuckSessions.push(s)
    }
  }
  return {
    totals: snap.totals || {},
    conductors,
    sessions,
    stuckSessions,
    decisionsWaiting: Array.isArray(snap.decisionsWaiting) ? snap.decisionsWaiting : [],
  }
}

export function GenuiPane() {
  const snap = commandCenterSignal.value
  const conn = connectionSignal.value
  const [views, setViews] = useState([])
  const [activeId, setActiveId] = useState(null)
  const [spec, setSpec] = useState(null)
  const [err, setErr] = useState('')

  // Load the view list once.
  useEffect(() => {
    fetchJSON('/api/command-center/genui/views')
      .then(d => {
        const vs = (d && Array.isArray(d.views)) ? d.views : []
        setViews(vs)
        if (vs.length && !activeId) setActiveId(vs[0].id)
      })
      .catch(e => setErr('Could not load views: ' + e.message))
  }, [])

  // Load the active spec whenever the selection changes (RESHAPE).
  useEffect(() => {
    if (!activeId) return
    setSpec(null)
    fetchSpec(activeId)
      .then(s => { setSpec(s); setErr('') })
      .catch(e => setErr('Could not load spec: ' + e.message))
  }, [activeId])

  const data = bindData(snap)

  return html`
    <div class="genui-pane" data-testid="genui-pane">
      <div class="genui-bar">
        <h1 class="genui-bar-title">Generative Command Center</h1>
        <span class=${`genui-live ${conn === 'connected' ? '' : 'stale'}`} data-testid="genui-live">
          ${conn === 'connected' ? '● live' : '● offline'}
        </span>
        <span class="genui-bar-hint">same data · 3 specs · switch to reshape the whole UI</span>
        <div class="genui-switch" data-testid="genui-switch">
          ${views.map(v => html`
            <button key=${v.id}
              class=${`genui-switch-btn ${v.id === activeId ? 'active' : ''}`}
              data-testid=${'genui-view-' + v.id}
              data-active=${v.id === activeId ? 'true' : 'false'}
              onClick=${() => setActiveId(v.id)}>${v.title}</button>
          `)}
        </div>
      </div>
      <div class="genui-body" data-testid="genui-body">
        ${err && html`<div class="genui-error" data-testid="genui-load-error">⚠️ ${err}</div>`}
        ${!err && !spec && html`<div class="genui-loading" data-testid="genui-loading">Loading view…</div>`}
        ${!err && spec && renderSpec(spec, data)}
      </div>
    </div>
  `
}
