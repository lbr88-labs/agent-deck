// Footer.js -- Status bar at the bottom of the shell.
// Bundle's `.footer` class with status counts, profile, keyboard hints.
import { html } from 'htm/preact'
import { menuModelSignal } from './dataModel.js'
import { connectionSignal, systemStatsSignal } from './state.js'
import { profileSignal } from './uiState.js'

export function Footer() {
  const { sessions } = menuModelSignal.value
  const conn = connectionSignal.value
  const running = sessions.filter(s => s.status === 'running').length
  const waiting = sessions.filter(s => s.status === 'waiting').length
  const errors  = sessions.filter(s => s.status === 'error').length
  const dotStyle = conn === 'connected' ? {} : { background: 'var(--tn-red)', boxShadow: '0 0 6px var(--tn-red)' }
  const stats = systemStatsSignal.value
  const cpu = stats?.cpu?.usage_percent
  const mem = stats?.memory?.usage_percent
  return html`
    <div class="footer">
      <span class="fseg"><span class="d" style=${dotStyle}/>ws · ${conn}</span>
      ${profileSignal.value && html`<span class="fseg">profile · ${profileSignal.value}</span>`}
      <span class="fseg">sessions · ${sessions.length}</span>
      <span class="fseg" style="color: var(--tn-green);">● ${running}</span>
      <span class="fseg" style="color: var(--tn-yellow);">◐ ${waiting}</span>
      <span class="fseg" style="color: var(--tn-red);">✕ ${errors}</span>
      ${typeof cpu === 'number' && html`<span class="fseg">cpu · ${cpu.toFixed(0)}%</span>`}
      ${typeof mem === 'number' && html`<span class="fseg">mem · ${mem.toFixed(0)}%</span>`}
      <span class="fspacer"/>
      <span class="fkbd"><span class="k">⌘K</span> palette</span>
      <span class="fkbd"><span class="k">/</span> filter</span>
      <span class="fkbd"><span class="k">n</span> new</span>
      <span class="fkbd"><span class="k">?</span> tweaks</span>
      <span class="fkbd"><span class="k">]</span> rail</span>
    </div>
  `
}
