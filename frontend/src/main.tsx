import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './design-system/tokens/global.css'
import './shared/i18n'
import { initTheme } from './shared/theme'
import { App } from './app/App'

initTheme()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)

// service worker (cache offline do shell + push); no desenvolvimento ele vem do devOptions do vite-plugin-pwa
if ('serviceWorker' in navigator) {
  void import('virtual:pwa-register').then(({ registerSW }) => registerSW({ immediate: true }))
}
