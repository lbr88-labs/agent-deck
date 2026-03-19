// main.js -- Preact app entry point
// Mounted into #app-root by <script type="module" src="/static/app/main.js">
import { render } from 'preact'
import { html } from 'htm/preact'
import { App } from './App.js'

const root = document.getElementById('app-root')
if (root) {
  render(html`<${App} />`, root)
}
