<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import SettingRow from './SettingRow.vue'
import { useSettingsSection } from './useSettingsSection'

const { t } = useI18n()
const { state, settings, load, persist } = useSettingsSection()

const blockedIds = computed(() => settings.value?.privacy.blockedApplicationIds ?? [])

const draft = ref('')
const error = ref<'duplicate' | 'invalid' | null>(null)

onMounted(() => void load())

function onAdd(): void {
  const id = draft.value.trim()
  if (id === '' || /\s/.test(id)) {
    error.value = 'invalid'
    return
  }
  if (blockedIds.value.includes(id)) {
    error.value = 'duplicate'
    return
  }
  error.value = null
  draft.value = ''
  void persist({ blockedApplicationIds: [...blockedIds.value, id] })
}

function onRemove(id: string): void {
  void persist({ blockedApplicationIds: blockedIds.value.filter((item) => item !== id) })
}
</script>

<template>
  <section class="blocked dg-card">
    <div class="blocked__text">
      <h2 class="blocked__title">{{ t('settings.privacy.blockedTitle') }}</h2>
      <p class="blocked__hint">{{ t('settings.privacy.blockedHint') }}</p>
    </div>

    <ul v-if="blockedIds.length > 0" class="blocked__list" :aria-label="t('settings.privacy.blockedTitle')">
      <li v-for="id in blockedIds" :key="id" class="blocked__item">
        <span class="blocked__id">{{ id }}</span>
        <button
          type="button"
          class="blocked__remove"
          :disabled="state !== 'ready'"
          :aria-label="t('settings.privacy.remove', { id })"
          :title="t('settings.privacy.remove', { id })"
          @click="onRemove(id)"
        >
          ×
        </button>
      </li>
    </ul>
    <p v-else class="blocked__empty">{{ t('settings.privacy.empty') }}</p>

    <form
      class="blocked__form"
      :aria-label="t('settings.privacy.add')"
      @submit.prevent="onAdd"
    >
      <input
        v-model="draft"
        class="dg-input blocked__input"
        type="text"
        :placeholder="t('settings.privacy.addPlaceholder')"
        :disabled="state !== 'ready'"
        :aria-invalid="error !== null"
        @input="error = null"
      >
      <button
        type="submit"
        class="dg-button"
        :disabled="state !== 'ready' || draft.trim() === ''"
      >
        {{ t('settings.privacy.add') }}
      </button>
    </form>
    <p v-if="error !== null" class="blocked__error" role="alert">
      {{ t(`settings.privacy.error.${error}`) }}
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
  flex-wrap: wrap;
  gap: 6px;
  list-style: none;
}

.blocked__item {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 6px 3px 10px;
  border: 1px solid var(--dg-chip-border);
  border-radius: 999px;
  background: var(--dg-input-fill);
  color: var(--dg-text-primary);
  font-size: 12px;
}

.blocked__remove {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
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

.blocked__form {
  display: flex;
  gap: 8px;
}

.blocked__input {
  flex: 1;
  min-width: 0;
}

.blocked__error {
  color: var(--dg-danger);
  font-size: 12px;
}
</style>
