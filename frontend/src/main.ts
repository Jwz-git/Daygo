import { createPinia } from 'pinia'
import { createApp } from 'vue'

import App from './App.vue'
import { i18n } from './i18n'
import { router } from './router'
import { useAppearanceStore } from './stores/appearance'
import './styles/index.css'

/*
 * Suppress the webview context menu in every build. Wails' own switch
 * (options.App.EnableDefaultContextMenu) is forced on in debug builds, so the
 * guard lives here — production relies on it too, so behaviour never depends
 * on which build is running. WKWebView honours preventDefault on contextmenu,
 * which is the same mechanism the Wails runtime itself uses.
 */
window.addEventListener('contextmenu', (event) => event.preventDefault())

async function bootstrap(): Promise<void> {
  const app = createApp(App)

  app.use(createPinia())
  app.use(i18n)

  // Resolve theme and language before the first paint: otherwise copy flashes
  // in the wrong locale and a dark-appearance window flashes light.
  await useAppearanceStore().hydrate()

  app.use(router)
  await router.isReady()

  app.mount('#app')
}

void bootstrap()
