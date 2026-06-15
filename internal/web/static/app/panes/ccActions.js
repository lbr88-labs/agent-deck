// panes/ccActions.js -- shared Command Center v2 routing + acknowledgement
// logic, used by both the list god-view and the per-project detail pages.
//
// The mission loop (COMMAND-CENTER-DESIGN.md Addendum 4): an ask is never
// fired into the void. routeAsk() POSTs /api/command-center/ask, immediately
// records an acknowledgement ("got it → routed to X"), then polls
// /api/command-center/reply to advance the ack to the result ("→ here's what
// happened"). New sessions the conductor spawns reflect back via the live SSE
// snapshot (handled in the panes), closing idea→reality end-to-end.
import { apiFetch } from '../api.js'
import { pushAck, updateAck } from '../state.js'

// routeAsk sends an instruction to a target (a conductor or maestro) with
// optional comment context, records the progression ack, and kicks off the
// correlated read-back. Returns the ack's correlationId.
//
// "Never silence" (Addendum 4): the ack is recorded the instant the user sends,
// BEFORE the request resolves — so even a delivery failure (no live agent behind
// the target) surfaces as a visible ack, never a fire-into-the-void. The ack is
// then advanced to "routed" on success, or marked failed, and finally to
// "result" by the correlated read-back.
export async function routeAsk({ text, target, context }) {
  const msg = (text || '').trim()
  if (!msg) return null
  const routedTo = target || 'maestro'
  // Optimistic ack — visible immediately, with a temporary correlation id.
  const tempId = 'cc-pending-' + Date.now()
  pushAck({
    correlationId: tempId,
    target: routedTo,
    text: msg,
    stage: 'received',
    ack: 'got it…',
    reply: '',
    at: new Date().toISOString(),
  })
  let resp
  try {
    resp = await apiFetch('POST', '/api/command-center/ask', {
      target: routedTo, text: msg, context: context || {},
    })
  } catch (e) {
    // Delivery failed — keep the ack, mark it so the user still sees their input
    // landed and where it tried to go (never silence).
    updateAck(tempId, { stage: 'failed', ack: 'sent, but delivery failed: ' + (e.message || 'error') })
    return tempId
  }
  const correlationId = resp.correlationId || tempId
  updateAck(tempId, {
    correlationId,
    target: resp.routedTo || routedTo,
    stage: resp.stage || 'routed',
    ack: resp.ack || ('got it — routed to ' + (resp.routedTo || routedTo)),
  })
  // Advance the ack to the result via correlated read-back (best-effort, async).
  pollReply(correlationId, resp.routedTo || routedTo)
  return correlationId
}

// pollReply reads the target's latest response a few times and, when it lands,
// advances the ack from "routed" to "result". Bounded so a quiet target doesn't
// poll forever; the live SSE feed is the other (free) reflection channel.
async function pollReply(correlationId, target, attempt = 0) {
  if (!target || attempt > 6) return
  const delay = 2500 + attempt * 1500
  setTimeout(async () => {
    try {
      const q = '?correlationId=' + encodeURIComponent(correlationId) +
        '&target=' + encodeURIComponent(target)
      const r = await apiFetch('GET', '/api/command-center/reply' + q)
      if (r && r.reply) {
        updateAck(correlationId, { stage: 'result', reply: r.reply })
        return // got a reply; stop polling
      }
    } catch (_) {
      // transient; keep trying within the bound
    }
    pollReply(correlationId, target, attempt + 1)
  }, delay)
}

// commentPrefill builds the "re <ref>: " text + context for a 💬 on any entity
// (decision / session / conductor). The context routes the message to the
// owning conductor and references exactly what was commented on (Addendum 2).
export function commentPrefill(entity) {
  if (entity.kind === 'decision') {
    return {
      text: `re ${entity.id || entity.question?.slice(0, 24)}: `,
      target: entity.route || 'conductor-agent-deck',
      context: { decisionId: entity.id || '' },
    }
  }
  if (entity.kind === 'session') {
    return {
      text: `re session ${entity.title}: `,
      target: entity.conductorTarget || 'maestro',
      context: { sessionTitle: entity.title, project: entity.project || '' },
    }
  }
  // conductor / project
  return {
    text: `re ${entity.name}: `,
    target: entity.target || ('conductor-' + entity.name),
    context: { project: entity.name },
  }
}
