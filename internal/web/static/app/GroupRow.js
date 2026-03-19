// GroupRow.js -- Collapsible group header row
import { html } from 'htm/preact'
import { groupExpandedSignal, toggleGroup, isGroupExpanded } from './groupState.js'

export function GroupRow({ item }) {
  const group = item.group
  // Read groupExpandedSignal.value to subscribe this component
  void groupExpandedSignal.value
  const expanded = isGroupExpanded(group.path, group.expanded)

  return html`
    <li>
      <button
        type="button"
        onClick=${() => toggleGroup(group.path, group.expanded)}
        class="w-full flex items-center gap-2 px-3 py-1.5 text-xs font-semibold
          uppercase tracking-wide dark:text-tn-muted text-gray-500
          hover:dark:text-tn-fg hover:text-gray-700 transition-colors"
        style="padding-left: calc(${item.level || 0} * 1rem + 0.75rem)"
        aria-expanded=${expanded}
      >
        <span class="text-base leading-none select-none">${expanded ? '\u25BE' : '\u25B8'}</span>
        <span class="flex-1 truncate text-left">${group.name || group.path}</span>
        <span class="dark:text-tn-muted/60 text-gray-400 font-normal">
          (${group.sessionCount || 0})
        </span>
      </button>
    </li>
  `
}
