// Sidebar.js -- Sidebar wrapper containing session filter (future) and session list
import { html } from 'htm/preact'
import { SessionList } from './SessionList.js'

export function Sidebar() {
  return html`
    <div class="flex flex-col h-full">
      <div class="px-sp-12 py-sp-8 border-b dark:border-tn-muted/20 border-gray-200">
        <h2 class="text-xs font-semibold uppercase tracking-wide dark:text-tn-muted text-gray-500">
          Sessions
        </h2>
      </div>
      <div class="flex-1 overflow-y-auto">
        <${SessionList} />
      </div>
    </div>
  `
}
