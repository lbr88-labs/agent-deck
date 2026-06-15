// panes/CommandCenterDetailPane.js -- per-project DETAIL page (Command Center
// v2, Phase 1). A LIVE AGGREGATION of one conductor's four feeds (Addendum 1),
// never hand-written:
//   (a) status/activity      — conductor status + status.json headline/inProgress
//   (b) produced docs        — conductor/<name>/outputs/*.md rendered inline
//   (c) live session list    — active children (error/stopped filtered out)
//   (d) decisions/questions  — its decisions-waiting (scoped subset of §D)
//
// The input box is PRE-SCOPED to this conductor (Addendum 2): no picker — the
// target is implicit from context, so typing here routes straight to THIS
// conductor's session via the supported `session send` path. 💬 on any row
// carries that row's context. Esc backs out to the list god-view.
//
// SECURITY — doc HTML trust boundary: doc.html is produced ONLY by the server's
// renderMarkdown (internal/web/markdown.go), which HTML-escapes every text
// fragment, emits only a fixed safe-tag whitelist, and drops javascript:/data:
// hrefs. It never passes through arbitrary user HTML. The server is the sanitize
// boundary (verified by markdown_test.go XSS cases); the client renders the
// already-safe HTML. We deliberately do not pull a client-side DOMPurify dep —
// the source is our own server, not untrusted input.
import { html } from 'htm/preact'
import { useState, useEffect } from 'preact/hooks'
import {
  ccDetailNameSignal, ccDetailSignal, commandCenterSignal,
  mutationsEnabledSignal,
} from '../state.js'
import { apiFetch } from '../api.js'
import { addToast } from '../Toast.js'
import { routeAsk, commentPrefill } from './ccActions.js'
import { AckList } from './CommandCenterAcks.js'

const STATUS_DOT = { running: '🟢', waiting: '🟡', idle: '⚪', error: '🔴', stopped: '⚫', absent: '⚫' }

function timeAgo(iso) {
  if (!iso) return ''
  const t = Date.parse(iso)
  if (isNaN(t)) return ''
  const s = Math.max(0, Math.floor((Date.now() - t) / 1000))
  if (s < 60) return s + 's ago'
  if (s < 3600) return Math.floor(s / 60) + 'm ago'
  if (s < 86400) return Math.floor(s / 3600) + 'h ago'
  return Math.floor(s / 86400) + 'd ago'
}

// Re-fetch the detail aggregation whenever the open conductor changes OR the
// live snapshot moves (a new doc / session transition bumps the snapshot, which
// we use as a cheap "something changed, refresh the detail" trigger).
function useDetail(name, snapVersion) {
  useEffect(() => {
    let cancelled = false
    if (!name) { ccDetailSignal.value = null; return }
    apiFetch('GET', '/api/command-center/detail/' + encodeURIComponent(name))
      .then(d => { if (!cancelled) ccDetailSignal.value = d })
      .catch(() => { if (!cancelled) ccDetailSignal.value = null })
    return () => { cancelled = true }
  }, [name, snapVersion])
}

function Doc({ doc }) {
  const [open, setOpen] = useState(true)
  // doc.html is server-sanitized (see file header). Render the safe HTML.
  return html`
    <div class="ccd-doc" data-testid="ccd-doc">
      <button class="ccd-doc-head" onClick=${() => setOpen(o => !o)}>
        <span class="ccd-doc-caret">${open ? '▾' : '▸'}</span>
        <span class="ccd-doc-title">${doc.title || doc.name}</span>
        <span class="ccd-doc-when">${timeAgo(doc.updatedAt)}</span>
      </button>
      ${open && html`<div class="ccd-doc-body md" dangerouslySetInnerHTML=${{ __html: doc.html }}></div>`}
    </div>
  `
}

export function CommandCenterDetailPane() {
  const name = ccDetailNameSignal.value
  const snap = commandCenterSignal.value
  const detail = ccDetailSignal.value
  const canMutate = mutationsEnabledSignal.value
  const [text, setText] = useState('')
  const [sending, setSending] = useState(false)
  const [status, setStatus] = useState('ready')

  // Bump the detail re-fetch whenever the live snapshot's generatedAt changes.
  useDetail(name, snap && snap.generatedAt)

  const back = () => { ccDetailNameSignal.value = null; ccDetailSignal.value = null }

  useEffect(() => {
    const onKey = (e) => {
      // Esc backs out (unless typing in the input, which clears focus first).
      if (e.key === 'Escape') {
        if (document.activeElement && document.activeElement.tagName === 'TEXTAREA') {
          document.activeElement.blur()
        } else {
          back()
        }
      }
      if (e.key === '/' && document.activeElement?.tagName !== 'TEXTAREA') {
        e.preventDefault()
        document.querySelector('.ccd-input textarea')?.focus()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  const target = (detail && detail.target) || ('conductor-' + name)

  const send = async (ctx) => {
    const msg = text.trim()
    if (!msg || sending) return
    if (!canMutate) { addToast('Two-way input is disabled (web mutations off)', 'info'); return }
    setSending(true); setStatus('sending…')
    try {
      // Detail-page input is PRE-SCOPED to this conductor (Addendum 2).
      await routeAsk({ text: msg, target, context: ctx || { project: name } })
      setStatus('✓ routed to ' + target)
      setText('')
    } catch (e) {
      setStatus('✗ ' + (e.message || 'send failed'))
    } finally {
      setSending(false)
    }
  }

  const onComment = (entity) => {
    const p = commentPrefill(entity)
    setText(p.text)
    document.querySelector('.ccd-input textarea')?.focus()
  }

  const onKey = (e) => {
    if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) { e.preventDefault(); send() }
  }

  const sessions = (detail && detail.sessions) || []
  const docs = (detail && detail.docs) || []
  const decisions = (detail && detail.decisions) || []
  const inProgress = (detail && detail.inProgress) || []
  const recentlyDone = (detail && detail.recentlyDone) || []

  return html`
    <div class="ccd" data-testid="command-center-detail-pane">
      <div class="ccd-top">
        <button class="ccd-back" onClick=${back} data-testid="ccd-back" title="Back (Esc)">← back</button>
        <span class="ccd-dot">${STATUS_DOT[detail && detail.status] || '⚪'}</span>
        <h1>${name}</h1>
        <span class="ccd-headline" data-testid="ccd-headline">${(detail && detail.headline) || ''}</span>
      </div>

      ${!detail && html`<div class="cc-empty" data-testid="ccd-loading">loading ${name}…</div>`}

      ${detail && html`
        <div class="ccd-grid">
          <div class="ccd-main">
            ${(inProgress.length > 0 || recentlyDone.length > 0) && html`
              <section class="ccd-sec">
                ${inProgress.length > 0 && html`
                  <h2>🛠 In progress</h2>
                  <ul class="ccd-list">${inProgress.map((x, i) => html`<li key=${i}>${x}</li>`)}</ul>`}
                ${recentlyDone.length > 0 && html`
                  <h2>✅ Recently done</h2>
                  <ul class="ccd-list">${recentlyDone.map((x, i) => html`<li key=${i}>${x}</li>`)}</ul>`}
              </section>`}

            <section class="ccd-sec" data-testid="ccd-docs">
              <h2>📄 Produced docs ${docs.length ? html`<span class="ccd-count">${docs.length}</span>` : ''}</h2>
              ${docs.length
                ? docs.map(d => html`<${Doc} key=${d.name} doc=${d}/>`)
                : html`<div class="cc-sdone" data-testid="ccd-no-docs">no docs yet — drops here from ${name}'s outputs/</div>`}
            </section>
          </div>

          <div class="ccd-side">
            <section class="ccd-sec">
              <h2>👉 Decisions</h2>
              ${decisions.length
                ? decisions.map((d, i) => html`
                    <div class="cc-ask" key=${d.id || i}>
                      ${d.id && html`<span class="cc-ask-id">${d.id}</span>`}
                      <span class="cc-ask-text">${d.question}</span>
                      <button class="cc-cmt" title="Answer this"
                        onClick=${() => onComment({ kind: 'decision', ...d })}>💬</button>
                    </div>`)
                : html`<div class="cc-sdone">none waiting</div>`}
            </section>

            <section class="ccd-sec" data-testid="ccd-sessions">
              <h2>🛰️ Live sessions</h2>
              ${sessions.length
                ? sessions.map(s => html`
                    <div class="cc-srow" key=${s.id} data-testid="ccd-session" data-status=${s.status}>
                      <span class="cc-sd">${STATUS_DOT[s.status] || '⚪'}</span>
                      <span class="cc-stt" title=${s.workingOn || s.title}>${s.title}</span>
                      <button class="cc-cmt" title="Comment on this session"
                        onClick=${() => onComment({ kind: 'session', title: s.title, conductorTarget: target, project: name })}>💬</button>
                    </div>`)
                : html`<div class="cc-sdone">no active sessions</div>`}
            </section>

            <${AckList}/>
          </div>
        </div>

        <div class="ccd-input cc-input" data-testid="ccd-input">
          <span class="ccd-scope" title="This input routes to ${target}">→ ${target}</span>
          <textarea
            placeholder=${'talk to ' + name + '… ⌘/Ctrl+Enter to send, Esc to go back'}
            value=${text}
            onInput=${e => setText(e.target.value)}
            onKeyDown=${onKey}></textarea>
          <button class="cc-send" disabled=${!text.trim() || sending} onClick=${() => send()} data-testid="ccd-send">➤ Send</button>
          <span class=${`cc-st ${status.startsWith('✓') ? 'ok' : status.startsWith('✗') ? 'err' : ''}`} data-testid="ccd-status">${status}</span>
        </div>
      `}
    </div>
  `
}
