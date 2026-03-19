// Topbar.js -- Full-width topbar with sidebar toggle, brand, connection, theme, settings
import { html } from 'htm/preact'
import { useState } from 'preact/hooks'
import { ThemeToggle } from './ThemeToggle.js'
import { SettingsPanel } from './SettingsPanel.js'
import { ConnectionIndicator } from './ConnectionIndicator.js'

export function Topbar({ onToggleSidebar, sidebarOpen }) {
  const [showSettings, setShowSettings] = useState(false)

  return html`
    <header class="flex items-center justify-between px-3 py-2
      dark:bg-tn-panel bg-white border-b dark:border-tn-muted/20 border-gray-200
      flex-shrink-0 relative z-50">
      <div class="flex items-center gap-3">
        <button
          type="button"
          onClick=${onToggleSidebar}
          class="lg:hidden text-lg dark:text-tn-muted text-gray-500 hover:dark:text-tn-fg hover:text-gray-700 transition-colors p-1"
          aria-label=${sidebarOpen ? 'Close sidebar' : 'Open sidebar'}
          aria-expanded=${sidebarOpen}
        >
          ${sidebarOpen ? '\u2715' : '\u2630'}
        </button>
        <span class="font-semibold text-sm dark:text-tn-fg text-gray-900">Agent Deck</span>
      </div>
      <div class="flex items-center gap-3">
        <${ConnectionIndicator} />
        <${ThemeToggle} />
        <button
          onClick=${() => setShowSettings(!showSettings)}
          class="text-xs dark:text-tn-muted text-gray-500 hover:dark:text-tn-fg hover:text-gray-700 transition-colors"
          title="Toggle settings"
          aria-expanded=${showSettings}
        >
          ${showSettings ? 'Hide' : 'Info'}
        </button>
      </div>
      ${showSettings && html`
        <div class="absolute top-full right-2 mt-1 z-50 px-3 py-2 rounded-lg
          dark:bg-tn-panel bg-white shadow-lg border
          dark:border-tn-muted/20 border-gray-200 min-w-[200px]">
          <${SettingsPanel} />
        </div>
      `}
    </header>
  `
}
