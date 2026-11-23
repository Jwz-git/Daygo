import { createPinia } from 'pinia'
import { createApp } from 'vue'

import App from './App.vue'
import { recoverWindow } from './api/windowResume'
import { trackWindowAction } from './lib/windowResume'
import { i18n } from './i18n'
import { clearFatalError, reportFatalError } from './lib/fatalError'
import { resolveDesktopPlatform } from './lib/desktopPlatform'
import { initializeDesktopBridge } from './api/desktopBridge'
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

/*
 * Trap errors that escape Vue's render tree — async callbacks, Wails event
 * handlers, rejected promises. A packaged Wails window has no devtools, so
 * without this an uncaught error is an unexplained blank window with nothing in
 * any log. FatalErrorOverlay turns each into a readable, copyable message.
 */
window.addEventListener('error', (event) => {
  reportFatalError('window.error', event.error ?? event.message)
})
window.addEventListener('unhandledrejection', (event) => {
  reportFatalError('unhandledrejection', event.reason)
})

async function bootstrap(): Promise<void> {
  await initializeDesktopBridge()
  document.documentElement.dataset.dgPlatform = await resolveDesktopPlatform()
  const resume = await recoverWindow()
  const app = createApp(App, { resumed: resume !== null })

  // Render/lifecycle errors that no ErrorBoundary caught land here.
  app.config.errorHandler = (error, _instance, info) => {
    reportFatalError(`vue (${info})`, error)
  }

  const pinia = createPinia()
  pinia.use(({ store }) => {
    store.$onAction(({ after, onError }) => {
      const done = trackWindowAction()
      after(done)
      onError(done)
    }, true)
  })
  app.use(pinia)
  app.use(i18n)

  // Resolve theme and language before the first paint: otherwise copy flashes
  // in the wrong locale and a dark-appearance window flashes light.
  await useAppearanceStore().hydrate()

  if (resume) await router.replace({
    name: resume.route,
    query: resume.route !== 'weekly' && resume.day ? { day: resume.day } : {},
  })
  app.use(router)
  // A new location is a fresh chance: clear the previous fatal error so a
  // working page is not hidden behind a stale overlay. If the destination also
  // throws, its ErrorBoundary reports again.
  router.afterEach(() => clearFatalError())
  await router.isReady()

  app.mount('#app')
}

void bootstrap()
