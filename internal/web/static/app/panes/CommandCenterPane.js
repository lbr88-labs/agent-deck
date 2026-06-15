// panes/CommandCenterPane.js -- The Command Center: a live, two-way fleet
// god-view embedded in the agent-deck web UI (see
// conductor/agent-deck/COMMAND-CENTER-DESIGN.md).
//
// v1 (shipped): live SSE status, see-everything (noise filtered), decisions
// waiting, completion toasts, two-way input → `session send`.
//
// v2 Phase 1 (this file + CommandCenterDetailPane.js):
//   - NAVIGATE INTO per-project DETAIL pages (click a conductor → its live
//     aggregation; Esc / ← back). Digit-jump 1–9 to a conductor.
//   - COMMENT-ON-ANYTHING 💬 on conductors / sessions / decisions, carrying the
//     entity's context so it routes to the OWNING conductor (Addendum 2).
//   - KEYBOARD NAV: ↑/↓ move row focus, Enter opens, digits jump, '/' focuses
//     the input, Esc clears focus.
//   - ACKNOWLEDGEMENTS: every ask shows got it → routed → result (Addendum 4),
//     advancing via correlated read-back + the SSE feed (CommandCenterAcks.js).
import { html } from 'htm/preact'
import { useState, useEffect, useRef } from 'preact/hooks'
import {
  commandCenterSignal, connectionSignal, mutationsEnabledSignal,
  ccDetailNameSignal,
} from '../state.js'
import { addToast } from '../Toast.js'
import { routeAsk, commentPrefill } from './ccActions.js'
import { AckList } from './CommandCenterAcks.js'
import { CommandCenterDetailPane } from './CommandCenterDetailPane.js'

const STATUS_DOT = {
  running: '🟢', waiting: '🟡', idle: '⚪', error: '🔴', stopped: '⚫', absent: '⚫',
}
const SUBSTATE_LABEL = {
  'model-unavailable': 'model unavailable',
  'auth-401': 'auth error (401)',
  'idle-at-empty-prompt': 'idle (empty prompt)',
}

function dotFor(name, status) {
  if (name === 'maestro' && status === 'running') return '🔵'
  return STATUS_DOT[status] || '⚪'
}

function liveCounts(counts) {
  if (!counts) return ''
  return ['running', 'waiting', 'idle'].filter(k => counts[k]).map(k => `${counts[k]} ${k}`).join(' · ')
}

function DecisionCard({ decision, onComment }) {
  return html`
    <div class="cc-ask" data-testid="cc-decision">
      ${decision.id && html`<span class="cc-ask-id">${decision.id}</span>`}
      <span class="cc-ask-text">${decision.question}</span>
      <button class="cc-cmt" title="Comment / answer this"
        onClick=${() => onComment({ kind: 'decision', ...decision })}>💬</button>
    </div>
  `
}

function SessionRow({ sess, conductorTarget, project, onComment }) {
  const sub = sess.substate && SUBSTATE_LABEL[sess.substate]
  return html`
    <div class="cc-srow" data-testid="cc-session" data-status=${sess.status}>
      <span class="cc-sd">${STATUS_DOT[sess.status] || '⚪'}</span>
      <span class="cc-stt" title=${sess.workingOn || sess.title}>${sess.title}</span>
      ${sub && html`<span class="cc-sub" title=${'honest-status: ' + sess.substate}>${sub}</span>`}
      <button class="cc-cmt" title="Comment on this session"
        onClick=${() => onComment({ kind: 'session', title: sess.title, conductorTarget, project })}>💬</button>
    </div>
  `
}

function ConductorRow({ cd, index, focused, onComment, onOpen }) {
  const [open, setOpen] = useState(false)
  const sub = cd.substate && SUBSTATE_LABEL[cd.substate]
  return html`
    <div class=${`cc-cd ${open ? 'open' : ''} ${focused ? 'focused' : ''}`}
      data-testid="cc-conductor" data-name=${cd.name}>
      <div class="cc-cd-head">
        <button class="cc-cd-toggle" title="Expand sessions" onClick=${() => setOpen(o => !o)}>
          <span class="cc-jump">${index < 9 ? index + 1 : ''}</span>
          <span class="cc-dot">${dotFor(cd.name, cd.status)}</span>
          <span class="cc-nm">${cd.name}</span>
          <span class="cc-ac" title=${cd.currentlyWorkingOn || ''}>
            ${cd.currentlyWorkingOn || (cd.status === 'absent' ? 'no conductor session' : cd.status)}
            ${sub && html` · ${sub}`}
          </span>
          <span class="cc-lc">${liveCounts(cd.counts)}</span>
          ${cd.docCount > 0 && html`<span class="cc-docs" title=${cd.docCount + ' docs'}>📄${cd.docCount}</span>`}
        </button>
        <button class="cc-cmt" title="Comment on this project"
          onClick=${() => onComment({ kind: 'conductor', name: cd.name, target: cd.target })}>💬</button>
        <button class="cc-open" title="Open detail page" data-testid="cc-open-detail"
          onClick=${() => onOpen(cd.name)}>open →</button>
      </div>
      ${open && html`
        <div class="cc-cd-body">
          ${cd.sessions && cd.sessions.length
            ? cd.sessions.map(s => html`<${SessionRow} key=${s.id} sess=${s}
                conductorTarget=${cd.target} project=${cd.name} onComment=${onComment}/>`)
            : html`<div class="cc-sdone">no active sessions</div>`}
        </div>
      `}
    </div>
  `
}

export function CommandCenterPane() {
  // When a detail page is open, render it instead of the list god-view.
  if (ccDetailNameSignal.value) {
    return html`<${CommandCenterDetailPane}/>`
  }

  const snap = commandCenterSignal.value
  const conn = connectionSignal.value
  const canMutate = mutationsEnabledSignal.value
  const [text, setText] = useState('')
  const [target, setTarget] = useState('maestro')
  const [sending, setSending] = useState(false)
  const [status, setStatus] = useState('ready')
  const [focusIdx, setFocusIdx] = useState(-1)
  const conductorsRef = useRef([])

  const conductors = (snap && Array.isArray(snap.conductors)) ? snap.conductors : []
  conductorsRef.current = conductors

  const openDetail = (name) => { ccDetailNameSignal.value = name }

  // Keyboard nav: ↑/↓ move focus, Enter opens focused, 1–9 jump+open, '/' input.
  useEffect(() => {
    const onKey = (e) => {
      const typing = document.activeElement?.tagName === 'TEXTAREA' ||
        document.activeElement?.tagName === 'INPUT' || document.activeElement?.tagName === 'SELECT'
      if (e.key === '/' && !typing) {
        e.preventDefault()
        document.querySelector('.cc-input textarea')?.focus()
        return
      }
      if (e.key === 'Escape' && typing) { document.activeElement.blur(); return }
      if (typing) return
      const rows = conductorsRef.current
      if (e.key === 'ArrowDown') { e.preventDefault(); setFocusIdx(i => Math.min(rows.length - 1, i + 1)) }
      else if (e.key === 'ArrowUp') { e.preventDefault(); setFocusIdx(i => Math.max(0, i - 1)) }
      else if (e.key === 'Enter' && focusIdx >= 0 && rows[focusIdx]) { e.preventDefault(); openDetail(rows[focusIdx].name) }
      else if (/^[1-9]$/.test(e.key)) {
        const idx = parseInt(e.key, 10) - 1
        if (rows[idx]) { e.preventDefault(); setFocusIdx(idx); openDetail(rows[idx].name) }
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [focusIdx])

  const onComment = (entity) => {
    const p = commentPrefill(entity)
    setText(p.text)
    if (p.target) setTarget(p.target)
    document.querySelector('.cc-input textarea')?.focus()
  }

  const send = async () => {
    const msg = text.trim()
    if (!msg || sending) return
    if (!canMutate) { addToast('Two-way input is disabled (web mutations off)', 'info'); return }
    setSending(true); setStatus('sending…')
    try {
      await routeAsk({ text: msg, target })
      setStatus('✓ routed to ' + target)
      setText('')
    } catch (e) {
      setStatus('✗ ' + (e.message || 'send failed'))
    } finally {
      setSending(false)
    }
  }

  const onKey = (e) => {
    if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) { e.preventDefault(); send() }
  }

  const targets = (snap && Array.isArray(snap.askTargets) ? snap.askTargets : ['maestro'])

  if (!snap) {
    return html`
      <div class="cc" data-testid="command-center-pane">
        <div class="cc-top">
          <h1>Command Center</h1>
          <span class=${`cc-live ${conn === 'connected' ? '' : 'stale'}`}>
            ${conn === 'connected' ? '● connecting…' : '● offline'}
          </span>
        </div>
        <div class="cc-empty" data-testid="cc-loading">Waiting for the first fleet snapshot…</div>
      </div>
    `
  }

  const decisions = Array.isArray(snap.decisionsWaiting) ? snap.decisionsWaiting : []
  const totals = snap.totals || {}

  return html`
    <div class="cc" data-testid="command-center-pane">
      <div class="cc-top">
        <h1>Command Center</h1>
        <span class=${`cc-live ${conn === 'connected' ? '' : 'stale'}`} data-testid="cc-live">
          ${conn === 'connected' ? '● live' : '● offline'}
        </span>
        <span class="cc-hint">↑↓ move · Enter / 1–9 open · / type · Esc back</span>
        <span class="cc-totals" data-testid="cc-totals">
          ${totals.running || 0} running · ${totals.waiting || 0} waiting · ${totals.idle || 0} idle
        </span>
      </div>

      <div class="cc-cols">
        <div class="cc-col">
          <h2>👉 Needs you</h2>
          ${decisions.length
            ? decisions.map((d, i) => html`<${DecisionCard} key=${d.id || i} decision=${d} onComment=${onComment}/>`)
            : html`<div class="cc-sdone" data-testid="cc-no-decisions">nothing waiting on you 🎉</div>`}
          <${AckList}/>
        </div>

        <div class="cc-col">
          <h2>🛰️ The fleet — what each is doing</h2>
          ${conductors.length
            ? conductors.map((cd, i) => html`<${ConductorRow} key=${cd.name} cd=${cd} index=${i}
                focused=${i === focusIdx} onComment=${onComment} onOpen=${openDetail}/>`)
            : html`<div class="cc-sdone" data-testid="cc-no-conductors">no conductors detected</div>`}
        </div>
      </div>

      <div class="cc-input" data-testid="cc-input">
        <select value=${target} onChange=${e => setTarget(e.target.value)} title="Route to" data-testid="cc-target">
          ${targets.map(t => html`<option key=${t} value=${t === 'conductor-maestro' ? 'maestro' : t}>
            ${t === 'conductor-maestro' || t === 'maestro' ? 'Maestro (default)' : t}
          </option>`)}
        </select>
        <textarea
          placeholder="answer a decision, comment, or instruct… ⌘/Ctrl+Enter to send"
          value=${text}
          onInput=${e => setText(e.target.value)}
          onKeyDown=${onKey}></textarea>
        <button class="cc-send" disabled=${!text.trim() || sending} onClick=${send} data-testid="cc-send">➤ Send</button>
        <span class=${`cc-st ${status.startsWith('✓') ? 'ok' : status.startsWith('✗') ? 'err' : ''}`} data-testid="cc-status">${status}</span>
      </div>
    </div>
  `
}
