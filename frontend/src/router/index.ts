import { createRouter, createWebHashHistory, type RouteRecordRaw } from 'vue-router'

/**
 * Hash history, deliberately.
 *
 * History mode would require the Wails asset server to fall back to index.html
 * for unknown paths. Hash mode behaves identically under `wails dev` (Vite
 * dev server), a `wails build` bundle, and a deep link, and a desktop window
 * gains nothing from clean URLs. Revisit only if the asset-server fallback is
 * confirmed; it is a one-line change.
 *
 * `meta.navKey` lets the side rail resolve its selection without matching on
 * path strings.
 */
const routes: RouteRecordRaw[] = [
  { path: '/', redirect: { name: 'timeline' } },
  {
    path: '/timeline',
    name: 'timeline',
    component: () => import('@/views/Timeline/TimelineView.vue'),
    meta: { navKey: 'timeline' },
  },
  {
    path: '/daily',
    name: 'daily',
    component: () => import('@/views/Daily/DailyView.vue'),
    meta: { navKey: 'daily' },
  },
  {
    path: '/weekly',
    name: 'weekly',
    component: () => import('@/views/Weekly/WeeklyView.vue'),
    meta: { navKey: 'weekly' },
  },
  {
    path: '/settings',
    name: 'settings',
    component: () => import('@/views/Settings/SettingsView.vue'),
    meta: { navKey: 'settings' },
  },
  { path: '/:pathMatch(.*)*', redirect: { name: 'timeline' } },
]

export const router = createRouter({
  history: createWebHashHistory(),
  routes,
})
