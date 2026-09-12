<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import type { JournalDayDTO } from '@/api/dto'

const props = defineProps<{
  journal: JournalDayDTO | null
  unavailable: boolean
  failed: boolean
  saving: boolean
}>()

const emit = defineEmits<{
  save: [entry: JournalDayDTO]
}>()

const { t } = useI18n()

interface Draft {
  intentions: string
  notes: string
  goals: string
  reflections: string
}

const draft = ref<Draft>({ intentions: '', notes: '', goals: '', reflections: '' })

function fromJournal(journal: JournalDayDTO | null): Draft {
  return {
    intentions: journal?.intentions ?? '',
    notes: journal?.notes ?? '',
    goals: journal?.goals ?? '',
    reflections: journal?.reflections ?? '',
  }
}

watch(() => props.journal, (next) => { draft.value = fromJournal(next) }, { immediate: true })

const status = computed(() => {
  if (draft.value.intentions.trim() !== '') {
    return draft.value.reflections.trim() !== '' ? 'complete' : 'intentions_set'
  }
  return 'draft'
})

const canSave = computed(() => !props.saving && props.journal !== null)

function submit(): void {
  if (!canSave.value) return
  const toValue = (text: string): string | null =>
    text.trim() === '' ? null : text
  emit('save', {
    day: props.journal?.day ?? '',
    intentions: toValue(draft.value.intentions),
    notes: toValue(draft.value.notes),
    goals: toValue(draft.value.goals),
    reflections: toValue(draft.value.reflections),
    summary: null,
    status: status.value,
    updatedAtTs: null,
  })
}
</script>

<template>
  <section class="daily-section" aria-labelledby="daily-journal-title">
    <header class="section-heading">
      <div>
        <h2 id="daily-journal-title">{{ t('daily.journal.title') }}</h2>
        <p>{{ t('daily.journal.description') }}</p>
      </div>
      <button
        type="button"
        class="dg-button dg-button--primary"
        :disabled="!canSave"
        @click="submit"
      >
        {{ saving ? t('daily.journal.saving') : t('daily.journal.save') }}
      </button>
    </header>

    <div v-if="unavailable || failed" class="journal-state dg-card">
      <strong>{{ failed ? t('daily.journal.failureTitle') : t('daily.journal.unavailableTitle') }}</strong>
      <span>{{ failed ? t('daily.journal.failureDescription') : t('daily.journal.unavailableDescription') }}</span>
    </div>

    <template v-else>
      <div class="journal-grid dg-card">
        <label class="journal-field">
          <span>{{ t('daily.journal.intentions') }}</span>
          <textarea v-model="draft.intentions" rows="3" :placeholder="t('daily.journal.intentionsPlaceholder')" />
        </label>
        <label class="journal-field">
          <span>{{ t('daily.journal.notes') }}</span>
          <textarea v-model="draft.notes" rows="3" :placeholder="t('daily.journal.notesPlaceholder')" />
        </label>
        <label class="journal-field">
          <span>{{ t('daily.journal.goals') }}</span>
          <textarea v-model="draft.goals" rows="2" :placeholder="t('daily.journal.goalsPlaceholder')" />
        </label>
        <label class="journal-field">
          <span>{{ t('daily.journal.reflections') }}</span>
          <textarea v-model="draft.reflections" rows="2" :placeholder="t('daily.journal.reflectionsPlaceholder')" />
        </label>
      </div>
      <div v-if="journal?.summary" class="journal-summary dg-card">
        <h3>{{ t('daily.journal.summary') }}</h3>
        <p>{{ journal.summary }}</p>
      </div>
    </template>
  </section>
</template>

<style scoped>
.daily-section { display: flex; flex-direction: column; gap: 10px; }

.section-heading {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 16px;
  padding: 4px 2px 2px;
}

.section-heading h2 {
  color: var(--dg-text-primary);
  font-size: 15px;
  font-weight: 650;
}

.section-heading p { color: var(--dg-text-tertiary); font-size: 12px; }

.journal-state { display: flex; flex-direction: column; gap: 6px; padding: 18px; }

.journal-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
  padding: 16px;
}

.journal-field { display: flex; flex-direction: column; gap: 6px; }

.journal-field span {
  color: var(--dg-text-secondary);
  font-size: 11px;
  font-weight: 600;
}

.journal-field textarea {
  resize: vertical;
  border: 1px solid var(--dg-chip-border);
  border-radius: 8px;
  background: var(--dg-surface);
  color: var(--dg-text-primary);
  font: inherit;
  font-size: 12px;
  line-height: 1.55;
  padding: 9px 11px;
}

.journal-field textarea:focus-visible {
  outline: none;
  border-color: var(--dg-accent, var(--dg-focus-ring));
  box-shadow: 0 0 0 3px var(--dg-focus-ring);
}

.journal-summary { display: flex; flex-direction: column; gap: 6px; padding: 14px 16px; }

.journal-summary h3 {
  color: var(--dg-text-secondary);
  font-size: 11px;
  font-weight: 600;
}

.journal-summary p { color: var(--dg-text-primary); font-size: 12px; line-height: 1.6; }

@media (max-width: 760px) {
  .journal-grid { grid-template-columns: minmax(0, 1fr); }
  .section-heading { flex-direction: column; align-items: flex-start; gap: 8px; }
}
</style>
