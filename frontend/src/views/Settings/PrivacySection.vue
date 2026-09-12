<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import type { ApplicationDTO } from '@/api/application'
import { getBlockedApplications, pickApplication } from '@/api/application'
import { WAILS_UNAVAILABLE } from '@/api/settings'

import { useSettingsSection } from './useSettingsSection'

type ListState = 'loading' | 'ready' | 'unavailable'

const { t } = useI18n()
const { state, settings, load, persist, writeFailed } = useSettingsSection()

/*
 * The privacy list keeps only bundle identifiers in settings; names and icons
 * are resolved on every read. A missing name means the platform could not
 * resolve the bundle, and the row then shows the identifier — the only label
 * that is actually known.
 */
const applications = ref<ApplicationDTO[]>([])
const listState = ref<ListState>('loading')
const selecting = ref(false)
const pickError = ref('')

const blockedIds = computed(() => settings.value?.privacy.blockedApplicationIds ?? [])
const canEdit = computed(() => state.value === 'ready' && listState.value === 'ready')

onMounted(() => void loadSection())

async function loadSection(): Promise<void> {
  await load()
  await refreshApplications()
}

async function refreshApplications(): Promise<void> {
  listState.value = 'loading'
  try {
    applications.value = await getBlockedApplications()
    listState.value = 'ready'
  } catch {
    listState.value = 'unavailable'
  }
}

async function onChoose(): Promise<void> {
  if (selecting.value || !canEdit.value) return
  selecting.value = true
  pickError.value = ''
  try {
    const application = await pickApplication()
    if (application === null || blockedIds.value.includes(application.id)) return
    await persist({ blockedApplicationIds: [...blockedIds.value, application.id] })
    await refreshApplications()
  } catch (cause: unknown) {
    pickError.value = cause instanceof Error && cause.message === WAILS_UNAVAILABLE
      ? t('settings.privacy.error.unavailable')
      : t('settings.privacy.error.pickFailed')
  } finally {
    selecting.value = false
  }
}

async function onRemove(id: string): Promise<void> {
  await persist({ blockedApplicationIds: blockedIds.value.filter((item) => item !== id) })
  await refreshApplications()
}

function labelOf(application: ApplicationDTO): string {
  return application.name === '' ? application.id : application.name
}
</script>

<template>
  <section class="blocked dg-card">
    <div class="blocked__text">
      <h2 class="blocked__title">{{ t('settings.privacy.blockedTitle') }}</h2>
      <p class="blocked__hint">{{ t('settings.privacy.blockedHint') }}</p>
    </div>

    <p v-if="listState === 'loading'" class="blocked__empty">
      {{ t('settings.privacy.loading') }}
    </p>
    <p v-else-if="listState === 'unavailable'" class="blocked__empty">
      {{ t('settings.privacy.unavailable') }}
    </p>
    <ul
      v-else-if="applications.length > 0"
      class="blocked__list"
      :aria-label="t('settings.privacy.blockedTitle')"
    >
      <li v-for="application in applications" :key="application.id" class="blocked__item">
        <img
          v-if="application.iconDataUrl !== ''"
          class="blocked__icon"
          :src="application.iconDataUrl"
          alt=""
          width="28"
          height="28"
        >
        <span v-else class="blocked__icon blocked__icon--fallback" aria-hidden="true">
          {{ labelOf(application).slice(0, 1).toUpperCase() }}
        </span>
        <span class="blocked__name" :class="{ 'blocked__name--raw': application.name === '' }">
          {{ labelOf(application) }}
        </span>
        <button
          type="button"
          class="blocked__remove"
          :disabled="!canEdit"
          :aria-label="t('settings.privacy.remove', { name: labelOf(application) })"
          :title="t('settings.privacy.remove', { name: labelOf(application) })"
          @click="onRemove(application.id)"
        >
          ×
        </button>
      </li>
    </ul>
    <p v-else class="blocked__empty">{{ t('settings.privacy.empty') }}</p>

    <div class="blocked__actions">
      <button
        type="button"
        class="dg-button"
        :disabled="!canEdit || selecting"
        @click="onChoose"
      >
        {{ selecting ? t('settings.privacy.selecting') : t('settings.privacy.choose') }}
      </button>
    </div>
    <p v-if="pickError" class="blocked__error" role="alert">{{ pickError }}</p>
    <p v-if="writeFailed" class="blocked__error" role="alert">
      {{ t('settings.privacy.writeError') }}
    </p>
  </section>
</template>

<style scoped>
.blocked {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 17px 18px;
}

.blocked__title {
  color: var(--dg-text-primary);
  font-size: 14px;
  font-weight: 600;
}

.blocked__hint {
  margin-top: 4px;
  color: var(--dg-text-secondary);
  font-size: 12px;
  max-width: 52ch;
}

.blocked__list {
  display: flex;
  flex-direction: column;
  list-style: none;
}

.blocked__item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 7px 4px;
  border-top: 1px solid var(--dg-card-border);
  color: var(--dg-text-primary);
  font-size: 13px;
}

.blocked__item:first-child {
  border-top: none;
}

.blocked__icon {
  flex: none;
  width: 28px;
  height: 28px;
}

.blocked__icon--fallback {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--dg-chip-border);
  border-radius: 7px;
  background: var(--dg-input-fill);
  color: var(--dg-text-secondary);
  font-size: 13px;
  font-weight: 600;
}

.blocked__name {
  flex: 1;
  min-width: 0;
  overflow-wrap: anywhere;
}

.blocked__name--raw {
  color: var(--dg-text-secondary);
  font-family: var(--dg-font-mono);
  font-size: 12px;
}

.blocked__remove {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border: none;
  border-radius: 50%;
  background: transparent;
  color: var(--dg-text-secondary);
  font-size: 14px;
  line-height: 1;
  cursor: pointer;
}

.blocked__remove:hover:not(:disabled) {
  background: var(--dg-hover-fill);
  color: var(--dg-text-primary);
}

.blocked__remove:focus-visible {
  outline: none;
  box-shadow: 0 0 0 3px var(--dg-focus-ring);
}

.blocked__remove:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.blocked__empty {
  color: var(--dg-text-secondary);
  font-size: 13px;
}

.blocked__actions {
  display: flex;
  gap: 8px;
}

.blocked__error {
  color: var(--dg-danger);
  font-size: 12px;
}
</style>
