<script setup lang="ts">
import { computed, onMounted, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import type { ApplicationDTO } from '@/api/application'
import {
  describeApplications,
  getBlockedApplications,
  getPrivacyCompatibility,
  listInstalledApplications,
  pickApplication,
  type PrivacyCompatibilityDTO,
} from '@/api/application'
import { WAILS_UNAVAILABLE } from '@/api/settings'

import { useSettingsSection } from './useSettingsSection'

type ListState = 'loading' | 'ready' | 'unavailable'

const { t, locale } = useI18n()
const { state, settings, load, persist, writeFailed } = useSettingsSection()

/*
 * The privacy list keeps only platform application identifiers in settings;
 * names and icons are resolved on every read. The installed grid enumerates
 * identifier/name pairs and resolves icons per batch, so a platform without
 * icon capability still renders a usable grid with fallback monograms.
 */
const applications = ref<ApplicationDTO[]>([])
const listState = ref<ListState>('loading')
const installed = shallowRef<ApplicationDTO[]>([])
const installedState = ref<ListState>('loading')
const selecting = ref(false)
const pickError = ref('')
const compatibility = ref<PrivacyCompatibilityDTO | null>(null)
const query = ref('')

const blockedIds = computed(() => settings.value?.privacy.blockedApplicationIds ?? [])
const blockedIdSet = computed(() => new Set(blockedIds.value))
/*
 * Names come from the enumeration, which resolves them in the UI language;
 * tiles outside the installed list (an uninstalled app that is still
 * configured) fall back to what the platform inspector can resolve.
 */
const installedNames = computed(() => {
  const names = new Map<string, string>()
  for (const application of installed.value) names.set(application.id, application.name)
  return names
})
const canEdit = computed(() => state.value === 'ready' && listState.value === 'ready')

const filteredInstalled = computed(() => {
  const needle = query.value.trim().toLowerCase()
  if (needle === '') return installed.value
  return installed.value.filter(
    (application) =>
      application.name.toLowerCase().includes(needle) ||
      application.id.toLowerCase().includes(needle),
  )
})

onMounted(() => void loadSection())

// Re-enumerate when the UI language changes so app names follow it; icons are
// preserved by id because they do not depend on the language.
watch(locale, () => void refreshInstalled())

async function loadSection(): Promise<void> {
	await load()
	await Promise.all([refreshApplications(), refreshInstalled(), refreshCompatibility()])
}

async function refreshCompatibility(): Promise<void> {
  try {
    compatibility.value = await getPrivacyCompatibility()
  } catch {
    compatibility.value = null
  }
}

async function refreshApplications(options?: { silent?: boolean }): Promise<void> {
  // A silent refresh keeps the current tiles rendered while refetching: the
  // v-for keys are stable, so Vue patches rows in place and the icons (same
  // data URLs) never flash. The loading note is for the first load only.
  if (!options?.silent) listState.value = 'loading'
  try {
    applications.value = await getBlockedApplications()
    listState.value = 'ready'
  } catch {
    if (applications.value.length === 0) listState.value = 'unavailable'
  }
}

/**
 * Rebuilds the blocked tiles from the authoritative id list in `settings`
 * (written by the persist response) plus identities the page already knows —
 * grid entries and their cached descriptions. Tiles appear on the next tick
 * with their icons; the silent refetch afterwards only fills unknowns.
 */
async function syncApplicationsFromSettings(): Promise<void> {
  if (!settings.value) return
  const known = new Map(applications.value.map((application) => [application.id, application]))
  applications.value = settings.value.privacy.blockedApplicationIds.map(
    (id) => known.get(id) ?? { id, name: installedNames.value.get(id) ?? '', iconDataUrl: '' },
  )
  const unresolved = applications.value.filter((application) => application.iconDataUrl === '')
  if (unresolved.length > 0) {
    try {
      const resolved = await describeApplications(unresolved.map((application) => application.id))
      const byId = new Map(resolved.map((application) => [application.id, application]))
      applications.value = applications.value.map(
        (application) => byId.get(application.id) ?? application,
      )
    } catch {
      // Display data only; monogram fallbacks remain until the next refresh.
    }
  }
}

/**
 * Enumerates the installed applications and then resolves their icons in
 * batches. The listing itself is cheap identifier/name pairs; icon payloads
 * are large enough that one batched describe per chunk beats a single
 * oversized call, and a chunk failing only costs its own icons.
 */
async function refreshInstalled(): Promise<void> {
  installedState.value = 'loading'
  try {
    const listing = await listInstalledApplications(locale.value)
    const previousIcons = new Map(installed.value.map((application) => [application.id, application.iconDataUrl]))
    installed.value = listing.map((application) => ({
      ...application,
      iconDataUrl: previousIcons.get(application.id) ?? application.iconDataUrl,
    }))
    installedState.value = 'ready'
    await resolveIcons(installed.value)
  } catch {
    installed.value = []
    installedState.value = 'unavailable'
  }
}

const iconBatchSize = 32

async function resolveIcons(listing: ApplicationDTO[]): Promise<void> {
  const withoutIcon = listing.filter((application) => application.iconDataUrl === '')
  for (let start = 0; start < withoutIcon.length; start += iconBatchSize) {
    const batch = withoutIcon.slice(start, start + iconBatchSize)
    try {
      const resolved = await describeApplications(batch.map((application) => application.id))
      const icons = new Map(resolved.map((application) => [application.id, application.iconDataUrl]))
      installed.value = installed.value.map((application) => ({
        ...application,
        iconDataUrl: icons.get(application.id) ?? application.iconDataUrl,
      }))
    } catch {
      // Icons are display data; a failed batch leaves monogram fallbacks.
      break
    }
  }
}

async function onAdd(id: string): Promise<void> {
  if (!canEdit.value || blockedIdSet.value.has(id)) return
  await persist({ blockedApplicationIds: [...blockedIds.value, id] })
  await syncApplicationsFromSettings()
}

async function onRemove(id: string): Promise<void> {
  if (!canEdit.value) return
  await persist({ blockedApplicationIds: blockedIds.value.filter((item) => item !== id) })
  await syncApplicationsFromSettings()
}

async function onClear(): Promise<void> {
  if (!canEdit.value || blockedIds.value.length === 0) return
  await persist({ blockedApplicationIds: [] })
  await syncApplicationsFromSettings()
}

async function onChoose(): Promise<void> {
  if (selecting.value || !canEdit.value) return
  selecting.value = true
  pickError.value = ''
  try {
    const application = await pickApplication()
    if (application === null || blockedIdSet.value.has(application.id)) return
    await onAdd(application.id)
  } catch (cause: unknown) {
    pickError.value = cause instanceof Error && cause.message === WAILS_UNAVAILABLE
      ? t('settings.privacy.error.unavailable')
      : t('settings.privacy.error.pickFailed')
  } finally {
    selecting.value = false
  }
}

function labelOf(application: ApplicationDTO): string {
  if (installedNames.value.has(application.id)) return installedNames.value.get(application.id)!
  return application.name === '' ? application.id : application.name
}
</script>

<template>
  <section class="privacy">
    <div class="privacy__text">
      <h2 class="privacy__title">{{ t('settings.privacy.blockedTitle') }}</h2>
      <p class="privacy__hint">{{ t('settings.privacy.blockedHint') }}</p>
    </div>

    <div
      v-if="compatibility?.platform === 'windows'"
      class="privacy__compatibility"
      :class="{ 'privacy__compatibility--unsupported': !compatibility.supported }"
      role="status"
    >
      <strong>
        {{ compatibility.supported
          ? t('settings.privacy.compatibility.supported')
          : t('settings.privacy.compatibility.unsupported') }}
      </strong>
      <span>
        {{ t('settings.privacy.compatibility.version', {
          version: compatibility.version,
          minimumBuild: compatibility.minimumBuild,
        }) }}
      </span>
    </div>

    <!--
      Installed applications: search, then a grid of tiles. Clicking a tile
      adds the application to the privacy list; a tile already on the list
      shows a badge and stays inert, because removal lives in the blocked row.
    -->
    <div class="privacy__search">
      <input
        v-model="query"
        class="dg-input"
        type="search"
        :placeholder="t('settings.privacy.searchPlaceholder')"
        :aria-label="t('settings.privacy.searchPlaceholder')"
      >
    </div>

    <div class="privacy__section-head">
      <h3 class="privacy__section-title">{{ t('settings.privacy.installedTitle') }}</h3>
      <span v-if="installedState === 'ready'" class="privacy__section-count">
        {{ t('settings.privacy.installedCount', { count: filteredInstalled.length }) }}
      </span>
    </div>

    <p v-if="installedState === 'loading'" class="privacy__note">
      {{ t('settings.privacy.loading') }}
    </p>
    <div v-else-if="installedState === 'unavailable'" class="privacy__note">
      <p>{{ t('settings.privacy.installedUnavailable') }}</p>
      <button
        type="button"
        class="dg-button"
        :disabled="!canEdit || selecting"
        @click="onChoose"
      >
        {{ selecting ? t('settings.privacy.selecting') : t('settings.privacy.choose') }}
      </button>
    </div>
    <p v-else-if="filteredInstalled.length === 0" class="privacy__note">
      {{ t('settings.privacy.searchEmpty') }}
    </p>
    <div v-else class="privacy__panel dg-scroll" role="listbox" :aria-label="t('settings.privacy.installedTitle')">
      <button
        v-for="application in filteredInstalled"
        :key="application.id"
        type="button"
        class="app-tile"
        :class="{ 'app-tile--blocked': blockedIdSet.has(application.id) }"
        :aria-pressed="blockedIdSet.has(application.id)"
        :disabled="blockedIdSet.has(application.id) || !canEdit"
        :title="blockedIdSet.has(application.id)
          ? t('settings.privacy.blockedBadge')
          : t('settings.privacy.add', { name: labelOf(application) })"
        :aria-label="blockedIdSet.has(application.id)
          ? t('settings.privacy.blockedBadge')
          : t('settings.privacy.add', { name: labelOf(application) })"
        @click="onAdd(application.id)"
      >
        <span class="app-tile__frame">
          <img
            v-if="application.iconDataUrl !== ''"
            class="app-tile__icon"
            :src="application.iconDataUrl"
            alt=""
            width="44"
            height="44"
          >
          <span v-else class="app-tile__icon app-tile__icon--fallback" aria-hidden="true">
            {{ labelOf(application).slice(0, 1).toUpperCase() }}
          </span>
          <span v-if="blockedIdSet.has(application.id)" class="app-tile__badge" aria-hidden="true">
            <svg viewBox="0 0 12 12" width="8" height="8" fill="none">
              <rect x="2.4" y="5.2" width="7.2" height="5" rx="1" fill="currentColor" />
              <path d="M4 5V3.6a2 2 0 1 1 4 0V5" stroke="currentColor" stroke-width="1.4" />
            </svg>
          </span>
        </span>
        <span class="app-tile__name">{{ labelOf(application) }}</span>
      </button>
    </div>

    <div class="privacy__section-head">
      <h3 class="privacy__section-title">
        {{ t('settings.privacy.blockedApplicationsTitle') }}
        <span class="privacy__section-count">
          {{ t('settings.privacy.blockedCount', { count: blockedIds.length }) }}
        </span>
      </h3>
      <button
        type="button"
        class="dg-button"
        :disabled="!canEdit || blockedIds.length === 0"
        @click="onClear"
      >
        {{ t('settings.privacy.clear') }}
      </button>
    </div>

    <p v-if="listState === 'loading'" class="privacy__note">
      {{ t('settings.privacy.loading') }}
    </p>
    <p v-else-if="listState === 'unavailable'" class="privacy__note">
      {{ t('settings.privacy.unavailable') }}
    </p>
    <div
      v-else-if="applications.length > 0"
      class="privacy__panel privacy__panel--blocked"
      :aria-label="t('settings.privacy.blockedApplicationsTitle')"
    >
      <button
        v-for="application in applications"
        :key="application.id"
        type="button"
        class="app-tile"
        :disabled="!canEdit"
        :aria-label="t('settings.privacy.remove', { name: labelOf(application) })"
        :title="t('settings.privacy.remove', { name: labelOf(application) })"
        @click="onRemove(application.id)"
      >
        <span class="app-tile__frame">
          <img
            v-if="application.iconDataUrl !== ''"
            class="app-tile__icon"
            :src="application.iconDataUrl"
            alt=""
            width="44"
            height="44"
          >
          <span v-else class="app-tile__icon app-tile__icon--fallback" aria-hidden="true">
            {{ labelOf(application).slice(0, 1).toUpperCase() }}
          </span>
          <span class="app-tile__badge" aria-hidden="true">
            <svg viewBox="0 0 12 12" width="8" height="8" fill="none">
              <rect x="2.4" y="5.2" width="7.2" height="5" rx="1" fill="currentColor" />
              <path d="M4 5V3.6a2 2 0 1 1 4 0V5" stroke="currentColor" stroke-width="1.4" />
            </svg>
          </span>
        </span>
        <span class="app-tile__name">{{ labelOf(application) }}</span>
      </button>
    </div>
    <p v-else class="privacy__note">{{ t('settings.privacy.empty') }}</p>

    <div class="privacy__actions">
      <button
        type="button"
        class="dg-button"
        :disabled="!canEdit || selecting"
        @click="onChoose"
      >
        {{ selecting ? t('settings.privacy.selecting') : t('settings.privacy.choose') }}
      </button>
    </div>
    <p v-if="pickError" class="privacy__error" role="alert">{{ pickError }}</p>
    <p v-if="writeFailed" class="privacy__error" role="alert">
      {{ t('settings.privacy.writeError') }}
    </p>
  </section>
</template>

<style scoped>
.privacy {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 17px 18px;
}

.privacy__title {
  color: var(--dg-text-primary);
  font-size: 14px;
  font-weight: 600;
}

.privacy__hint {
  margin-top: 4px;
  color: var(--dg-text-secondary);
  font-size: 12px;
  max-width: 52ch;
}

.privacy__compatibility {
  display: flex;
  flex-direction: column;
  gap: 3px;
  padding: 10px 12px;
  border: 1px solid color-mix(in srgb, var(--dg-success, #3d8b58) 35%, var(--dg-card-border));
  border-radius: 7px;
  background: color-mix(in srgb, var(--dg-success, #3d8b58) 8%, transparent);
  color: var(--dg-text-secondary);
  font-size: 12px;
}

.privacy__compatibility strong {
  color: var(--dg-text-primary);
  font-size: 13px;
}

.privacy__compatibility--unsupported {
  border-color: color-mix(in srgb, var(--dg-danger) 38%, var(--dg-card-border));
  background: color-mix(in srgb, var(--dg-danger) 8%, transparent);
}

.privacy__compatibility--unsupported strong {
  color: var(--dg-danger);
}

.privacy__search {
  max-width: 460px;
}

.privacy__section-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
}

.privacy__section-title {
  color: var(--dg-text-primary);
  font-size: 13px;
  font-weight: 600;
}

.privacy__section-count {
  margin-left: 8px;
  color: var(--dg-text-secondary);
  font-size: 12px;
  font-weight: 400;
}

.privacy__panel {
  display: grid;
  /* Auto-fill capped at seven columns: the min column width bottoms out at
     one-seventh of the row, so a wide window adds tile width instead of an
     eighth column, and a narrow one falls back to the 92px tile. */
  grid-template-columns: repeat(auto-fill, minmax(max(92px, calc((100% - 36px) / 7)), 1fr));
  gap: 6px;
  max-height: 320px;
  padding: 10px;
  border: 1px solid var(--dg-card-border);
  border-radius: var(--dg-card-radius);
  background: var(--dg-card-fill);
}

.privacy__panel--blocked {
  max-height: 200px;
}

.privacy__note {
  color: var(--dg-text-secondary);
  font-size: 13px;
}

.privacy__actions {
  display: flex;
  gap: 8px;
}

.privacy__error {
  color: var(--dg-danger);
  font-size: 12px;
}

.app-tile {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 10px 6px 8px;
  border: 1px solid transparent;
  border-radius: 8px;
  background: transparent;
  cursor: pointer;
  text-align: center;
}

.app-tile:hover:not(:disabled) {
  border-color: var(--dg-card-border);
  background: var(--dg-hover-fill);
}

.app-tile:focus-visible {
  outline: none;
  box-shadow: 0 0 0 3px var(--dg-focus-ring);
}

.app-tile:disabled {
  cursor: default;
}

/* A tile already on the privacy list: accent ring marks it, and removal
   happens in the blocked row, not here. */
.app-tile--blocked {
  border-color: color-mix(in srgb, var(--dg-accent) 40%, transparent);
  background: color-mix(in srgb, var(--dg-accent) 8%, transparent);
}

.app-tile__frame {
  position: relative;
  width: 44px;
  height: 44px;
}

.app-tile__icon {
  width: 44px;
  height: 44px;
}

.app-tile__icon--fallback {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--dg-chip-border);
  border-radius: 9px;
  background: var(--dg-input-fill);
  color: var(--dg-text-secondary);
  font-size: 18px;
  font-weight: 600;
}

.app-tile__badge {
  position: absolute;
  right: -5px;
  bottom: -3px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 15px;
  height: 15px;
  border-radius: 50%;
  background: var(--dg-accent);
  color: #ffffff;
}

.app-tile__name {
  max-width: 100%;
  overflow: hidden;
  color: var(--dg-text-primary);
  font-size: 12px;
  line-height: 1.25;
  text-align: center;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}
</style>
