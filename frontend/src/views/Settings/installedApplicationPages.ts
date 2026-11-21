import { computed, onScopeDispose, ref, shallowRef, watch } from 'vue'
import type { Ref } from 'vue'

import { describeApplications, listInstalledApplications } from '@/api/application'
import type { ApplicationDTO } from '@/api/application'

const pageSize = 48
const iconBatchSize = 32

export function createInstalledApplicationPages(locale: Readonly<Ref<string>>, api = {
  list: listInstalledApplications,
  describe: describeApplications,
}) {
  const installed = shallowRef<ApplicationDTO[]>([])
  const installedState = ref<'loading' | 'ready' | 'unavailable'>('loading')
  const query = ref('')
  const page = ref(0)
  const icons = shallowRef(new Map<string, string>())
  let active = true
  let installedRequest = 0
  let iconRequest = 0

  const installedNames = computed(() =>
    new Map(installed.value.map((application) => [application.id, application.name])),
  )
  const filtered = computed(() => {
    const needle = query.value.trim().toLowerCase()
    if (needle === '') return installed.value
    return installed.value.filter((app) => app.name.toLowerCase().includes(needle) || app.id.toLowerCase().includes(needle))
  })
  const total = computed(() => filtered.value.length)
  const pageCount = computed(() => Math.max(1, Math.ceil(total.value / pageSize)))
  const items = computed(() => filtered.value.slice(page.value * pageSize, (page.value + 1) * pageSize))
  const shown = computed(() => items.value.map((app) => ({
    ...app, iconDataUrl: icons.value.get(app.id) ?? app.iconDataUrl,
  })))

  function navigate(offset: -1 | 1): void {
    page.value = Math.max(0, Math.min(pageCount.value - 1, page.value + offset))
  }

  async function refreshInstalled(): Promise<void> {
    if (!active) return
    const request = ++installedRequest
    iconRequest++
    installedState.value = 'loading'
    try {
      const listing = await api.list(locale.value)
      if (request !== installedRequest) return
      const listed = new Set(listing.map(({ id }) => id))
      icons.value = new Map([...icons.value].filter(([id]) => listed.has(id)))
      installed.value = listing
      page.value = Math.min(page.value, pageCount.value - 1)
      installedState.value = 'ready'
    } catch {
      if (request !== installedRequest) return
      installed.value = []
      page.value = 0
      installedState.value = 'unavailable'
    }
  }

  async function loadIcons(): Promise<void> {
    const request = ++iconRequest
    if (installedState.value !== 'ready') return
    const visibleIds = new Set(items.value.map(({ id }) => id))
    icons.value = new Map([...icons.value].filter(([id]) => visibleIds.has(id)))
    const missing = items.value.filter(({ id, iconDataUrl }) => !icons.value.has(id) && !iconDataUrl)
    for (let from = 0; from < missing.length; from += iconBatchSize) {
      const batch = missing.slice(from, from + iconBatchSize)
      try {
        const details = await api.describe(batch.map(({ id }) => id))
        if (request !== iconRequest) return
        const next = new Map(icons.value)
        for (const { id } of batch) next.set(id, '')
        for (const { id, iconDataUrl } of details) next.set(id, iconDataUrl)
        icons.value = next
      } catch {
        // The visible tiles retain their fallback monograms.
        break
      }
    }
  }

  watch(query, () => { page.value = 0 }, { flush: 'sync' })
  watch([page, query, installedState, installed], () => void loadIcons())
  watch(locale, () => void refreshInstalled())
  onScopeDispose(() => {
    active = false
    installedRequest++
    iconRequest++
  })

  return { installedNames, installedState, navigate, page, pageCount, query, refreshInstalled, shown, total }
}
