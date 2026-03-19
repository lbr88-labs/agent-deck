// SessionRow.js -- Single session item with status dot, title, tool badge, cost badge
import { html } from 'htm/preact'
import { selectedIdSignal, sessionCostsSignal } from './state.js'

const STATUS_COLORS = {
  running:  'bg-tn-blue',
  waiting:  'bg-tn-yellow animate-pulse',
  idle:     'bg-tn-muted',
  error:    'bg-tn-red',
  starting: 'bg-tn-purple animate-pulse',
  stopped:  'bg-tn-muted/50',
}

export function SessionRow({ item, focused }) {
  const session = item.session
  const isSelected = selectedIdSignal.value === session.id
  const costUSD = sessionCostsSignal.value[session.id]
  const costLabel = (costUSD != null && costUSD >= 0.001)
    ? '$' + costUSD.toFixed(2)
    : null
  const dotColor = STATUS_COLORS[session.status] || 'bg-tn-muted'

  function handleClick() {
    selectedIdSignal.value = session.id
  }

  return html`
    <li>
      <button
        type="button"
        onClick=${handleClick}
        class="w-full flex items-center gap-2 px-3 py-1.5 rounded text-left text-sm
          transition-colors
          ${isSelected
            ? 'dark:bg-tn-blue/20 bg-blue-100 dark:text-tn-fg text-gray-900'
            : focused
              ? 'dark:bg-tn-muted/10 bg-gray-100 dark:text-tn-fg text-gray-700'
              : 'dark:hover:bg-tn-muted/10 hover:bg-gray-50 dark:text-tn-fg text-gray-700'
          }"
        style="padding-left: calc(${item.level || 0} * 1rem + 0.75rem)"
        data-session-id=${session.id}
      >
        <span class="w-2 h-2 rounded-full flex-shrink-0 ${dotColor}"></span>
        <span class="flex-1 truncate">${session.title || session.id}</span>
        <span class="text-xs dark:text-tn-muted text-gray-400 flex-shrink-0">
          ${session.tool || 'shell'}
        </span>
        ${costLabel && html`
          <span class="text-xs dark:text-tn-green text-green-600 flex-shrink-0 font-mono">
            ${costLabel}
          </span>
        `}
      </button>
    </li>
  `
}
