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
        <div class="journal-group">
          <span class="journal-group__label journal-group__label--plan">
            {{ t('daily.journal.plannedLabel') }}
          </span>
          <label class="journal-field">
            <span>{{ t('daily.journal.intentions') }}</span>
            <textarea v-model="draft.intentions" class="dg-reading" rows="3" :placeholder="t('daily.journal.intentionsPlaceholder')" />
          </label>
          <label class="journal-field">
            <span>{{ t('daily.journal.goals') }}</span>
            <textarea v-model="draft.goals" class="dg-reading" rows="2" :placeholder="t('daily.journal.goalsPlaceholder')" />
          </label>
        </div>
        <div class="journal-group">
          <span class="journal-group__label journal-group__label--log">
            {{ t('daily.journal.loggedLabel') }}
          </span>
          <label class="journal-field">
            <span>{{ t('daily.journal.notes') }}</span>
            <textarea v-model="draft.notes" class="dg-reading" rows="3" :placeholder="t('daily.journal.notesPlaceholder')" />
          </label>
          <label class="journal-field">
            <span>{{ t('daily.journal.reflections') }}</span>
            <textarea v-model="draft.reflections" class="dg-reading" rows="2" :placeholder="t('daily.journal.reflectionsPlaceholder')" />
          </label>
        </div>
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

/* Two labelled columns — the left holds what you meant to do (intentions,
   goals), the right what actually happened (notes, reflections) — split by a
   hairline that folds away on narrow widths. */
.journal-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0;
  padding: 0;
  overflow: hidden;
}

.journal-group {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 18px 20px 20px;
}

.journal-group + .journal-group { border-left: 1px solid var(--dg-card-border); }

.journal-group__label {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  color: var(--dg-text-tertiary);
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.07em;
  text-transform: uppercase;
}

.journal-group__label::before {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  content: '';
}

.journal-group__label--plan::before { background: var(--dg-accent); }
.journal-group__label--log::before { background: var(--dg-success, #39815e); }

.journal-field { display: flex; flex-direction: column; gap: 6px; }

.journal-field span {
  color: var(--dg-text-secondary);
  font-size: 11px;
  font-weight: 600;
}

.journal-field textarea {
  resize: none;
  border: 1px solid var(--dg-input-border);
  border-radius: 9px;
  background: var(--dg-input-fill);
  color: var(--dg-text-primary);
  font: inherit;
  font-size: 12px;
  line-height: 1.6;
  padding: 10px 12px;
  transition:
    border-color var(--dg-motion-base) var(--dg-ease-out),
    background var(--dg-motion-base) var(--dg-ease-out),
    box-shadow var(--dg-motion-base) var(--dg-ease-out);
}

.journal-field textarea::placeholder { color: var(--dg-text-muted); }

.journal-field textarea:hover { background: var(--dg-input-fill-hover); }

.journal-field textarea:focus-visible {
  outline: none;
  background: var(--dg-input-fill-hover);
  border-color: var(--dg-accent, var(--dg-focus-ring));
  box-shadow: 0 0 0 3px var(--dg-focus-ring);
}

@media (max-width: 760px) {
  .journal-grid { grid-template-columns: minmax(0, 1fr); }
  .journal-group + .journal-group { border-left: 0; border-top: 1px solid var(--dg-card-border); }
  .section-heading { flex-direction: column; align-items: flex-start; gap: 8px; }
}
</style>
