// App.js -- Root Preact component (app shell)
// Renders a floating bar with theme toggle, connection indicator, and settings.
// During phase 2, the main UI is still vanilla JS. In phase 3+, this component
// will expand to own the session list, layout, and other features.
import { html } from 'htm/preact'
import { useState } from 'preact/hooks'
import { ThemeToggle } from './ThemeToggle.js'
import { SettingsPanel } from './SettingsPanel.js'
import { connectionSignal } from './state.js'

export function App() {
  const connection = connectionSignal.value
  const [showSettings, setShowSettings] = useState(false)

  return html`
    <div class="fixed top-2 right-2 z-50 flex flex-col items-end gap-2">
      <div class="flex items-center gap-3 px-3 py-1.5 rounded-lg
        dark:bg-tn-panel/90 bg-white/90 backdrop-blur-sm shadow-lg border
        dark:border-tn-muted/20 border-gray-200">
        <${ThemeToggle} />
        <button
          onClick=${() => setShowSettings(!showSettings)}
          class="text-xs dark:text-tn-muted text-gray-500 hover:dark:text-tn-fg hover:text-gray-700 transition-colors"
          title="Toggle settings"
          aria-expanded=${showSettings}
        >
          ${showSettings ? 'Hide' : 'Info'}
        </button>
        <div class="w-2 h-2 rounded-full ${
          connection === 'connected' ? 'bg-tn-green' :
          connection === 'connecting' ? 'bg-tn-yellow animate-pulse' :
          'bg-tn-red'
        }" title="SSE: ${connection}"></div>
      </div>
      ${showSettings && html`
        <div class="px-3 py-2 rounded-lg
          dark:bg-tn-panel/90 bg-white/90 backdrop-blur-sm shadow-lg border
          dark:border-tn-muted/20 border-gray-200 min-w-[200px]">
          <${SettingsPanel} />
        </div>
      `}
    </div>
  `
}
