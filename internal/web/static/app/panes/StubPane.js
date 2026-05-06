// panes/StubPane.js -- Designed empty state for tabs whose data lives in the
// TUI but is not exposed via the web API today.
//
// Used by MCP / Skills / Conductor / Watchers tabs. Per the parity matrix
// these are MISSING endpoints; rather than inventing data we render a clean,
// bundle-styled empty state that explains the gap and points users at the
// TUI hotkey.
import { html } from 'htm/preact'
import { Logo } from '../icons.js'

export function StubPane({ title, message, hotkey }) {
  return html`
    <div class="costs">
      <div class="chart-card" style="display: flex; flex-direction: column; align-items: center; justify-content: center; text-align: center; gap: 14px; padding: 48px 24px; min-height: 320px;">
        <${Logo}/>
        <div class="title" style="font-size: 16px;">${title}</div>
        <div style="font-family: var(--mono); font-size: 12px; color: var(--text-dim); line-height: 1.6; max-width: 460px;">
          ${message}
        </div>
        <div style="font-family: var(--mono); font-size: 11px; color: var(--muted); padding-top: 8px;">
          No data yet — see TUI for now${hotkey ? ` ` : '.'}
          ${hotkey && html`<span class="kbd" style="border:1px solid var(--border); padding: 1px 6px; border-radius: 3px; color: var(--text); margin-left: 4px;">${hotkey}</span>`}
        </div>
      </div>
    </div>
  `
}
