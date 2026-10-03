<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import type { CategoryDTO } from '@/api/dto'
import DgSelect, { type DgSelectOption } from '@/components/DgSelect.vue'
import { categoryLabel } from '@/lib/categoryLabel'

import { safeCategoryColor } from './layout'

/*
 * Add / edit form for one plan block. It owns its draft, starting from
 * `initial`, and hands the whole draft back on save; validation is Go's
 * (end after start, title required), surfaced through `error`.
 */
export interface PlanDraft {
  id: number
  start: string
  end: string
  title: string
  notes: string
  categoryId: string
  remind: boolean
}

const props = defineProps<{
  initial: PlanDraft
  categories: CategoryDTO[]
  pending: boolean
  error: string
}>()

const emit = defineEmits<{
  save: [draft: PlanDraft]
  cancel: []
}>()

const { t } = useI18n()
const draft = ref<PlanDraft>({ ...props.initial })
const titleInput = ref<HTMLInputElement | null>(null)
onMounted(() => titleInput.value?.focus())

const categoryOptions = computed<DgSelectOption[]>(() => [
  { value: '', label: t('timeline.plan.noCategory') },
  ...props.categories
    .filter((category) => !category.isSystem)
    .map((category) => ({
      value: category.id,
      label: categoryLabel(category.name, t),
      color: safeCategoryColor(category.colorHex),
    })),
])
</script>

<template>
  <form class="plan-form" @submit.prevent="emit('save', { ...draft })">
    <input
      ref="titleInput"
      v-model="draft.title"
      class="dg-input plan-form__title"
      maxlength="200"
      :aria-label="t('timeline.plan.what')"
      :placeholder="t('timeline.plan.whatPlaceholder')"
    >
    <div class="plan-form__times">
      <input v-model="draft.start" class="dg-input" type="time" step="300" required :aria-label="t('timeline.plan.start')">
      <span aria-hidden="true">–</span>
      <input v-model="draft.end" class="dg-input" type="time" step="300" required :aria-label="t('timeline.plan.end')">
    </div>
    <textarea
      v-model="draft.notes"
      class="dg-input plan-form__notes"
      rows="2"
      maxlength="4000"
      :aria-label="t('timeline.plan.notes')"
      :placeholder="t('timeline.plan.notesPlaceholder')"
    ></textarea>
    <div class="plan-form__row">
      <DgSelect v-model="draft.categoryId" size="sm" :options="categoryOptions" :aria-label="t('timeline.plan.category')" />
      <label class="plan-form__remind">
        <input v-model="draft.remind" class="dg-checkbox" type="checkbox">
        <span>{{ t('timeline.plan.remind') }}</span>
      </label>
    </div>
    <p v-if="error" class="plan-form__error" role="alert">{{ error }}</p>
    <div class="plan-form__actions">
      <button type="button" class="dg-button" @click="emit('cancel')">{{ t('common.action.cancel') }}</button>
      <button type="submit" class="dg-button dg-button--primary" :disabled="pending">{{ t('common.action.save') }}</button>
    </div>
  </form>
</template>

<style scoped>
.plan-form {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 7px;
  min-width: 0;
  padding: 8px 0 4px;
}

.plan-form .dg-input {
  min-height: 30px;
  padding: 5px 9px;
  font-size: 12.5px;
}

/* On the inspector's near-white tile the input fill alone does not read as a
   field, so the form's controls get a hairline edge (focus keeps its ring). */
.plan-form .dg-input,
.plan-form :deep(.dg-select) {
  box-shadow: inset 0 0 0 1px var(--dg-input-border);
}

.plan-form .dg-input:focus-visible,
.plan-form :deep(.dg-select:focus-visible),
.plan-form :deep(.dg-select--open) {
  box-shadow: inset 0 0 0 1px var(--dg-input-border), 0 0 0 3px var(--dg-focus-ring);
}

.plan-form .plan-form__title {
  min-height: 34px;
  font-size: 14px;
  font-weight: 600;
}

.plan-form__times {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr);
  align-items: center;
  gap: 6px;
  color: var(--dg-text-muted);
  font-variant-numeric: tabular-nums;
}

.plan-form .plan-form__notes {
  min-height: 52px;
  resize: vertical;
  line-height: 1.5;
}

.plan-form__row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 10px;
}

.plan-form__remind {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--dg-text-secondary);
  font-size: 12px;
  white-space: nowrap;
  cursor: pointer;
}

.plan-form__error {
  margin: 0;
  color: var(--dg-danger);
  font-size: 12px;
  line-height: 1.45;
}

.plan-form__actions {
  display: flex;
  justify-content: flex-end;
  gap: 6px;
  margin-top: 2px;
}

.plan-form__actions .dg-button {
  min-height: 28px;
  padding: 4px 12px;
  font-size: 12px;
}
</style>
