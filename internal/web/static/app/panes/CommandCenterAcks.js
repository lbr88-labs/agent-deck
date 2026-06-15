// panes/CommandCenterAcks.js -- the ACKNOWLEDGEMENTS surface (Command Center
// v2, Addendum 4: "never silence"). Every ask appends an ack that shows the
// progression got it → routed to X → here's what happened, advancing live as
// the correlated read-back (ccActions.pollReply) and the SSE feed report back.
import { html } from 'htm/preact'
import { ccAcksSignal } from '../state.js'

const STAGE_LABEL = {
  received: 'received',
  routed: 'routed',
  'session-created': 'session created',
  result: 'replied',
  failed: 'delivery failed',
}

function stageIcon(stage) {
  if (stage === 'result') return '✅'
  if (stage === 'session-created') return '🚀'
  if (stage === 'failed') return '⚠️'
  if (stage === 'received') return '⏳'
  return '📨'
}

export function AckList() {
  const acks = ccAcksSignal.value
  if (!acks || acks.length === 0) return null
  return html`
    <section class="ccd-sec cc-acks" data-testid="cc-acks">
      <h2>🧾 Your asks</h2>
      ${acks.map(a => html`
        <div class="cc-ack" key=${a.correlationId} data-testid="cc-ack" data-stage=${a.stage}>
          <div class="cc-ack-line">
            <span class="cc-ack-ico">${stageIcon(a.stage)}</span>
            <span class="cc-ack-text" title=${a.text}>${a.text}</span>
          </div>
          <div class="cc-ack-meta">
            got it → ${a.target} → <span class="cc-ack-stage">${STAGE_LABEL[a.stage] || a.stage}</span>
          </div>
          ${a.reply && html`<div class="cc-ack-reply" data-testid="cc-ack-reply">${a.reply}</div>`}
        </div>
      `)}
    </section>
  `
}
