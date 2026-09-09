import { createPinia } from 'pinia'
import { createApp } from 'vue'

import App from './App.vue'
import { i18n } from './i18n'
import { router } from './router'
import { useAppearanceStore } from './stores/appearance'
import './styles/index.css'

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
