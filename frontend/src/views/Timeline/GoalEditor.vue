<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import type { CategoryDTO, DayGoalDTO } from '@/api/dto'
import { categoryLabel } from '@/lib/categoryLabel'

/*
 * The day-goal form: focus/distraction minute targets, skip flag and the
 * focus/distraction category chips. Shared by the timeline inspector's
 * default pane and the daily page's goal panel, so the editing contract
 * lives in exactly one place.
 */
const props = defineProps<{
  goal: DayGoalDTO | null
  categories: CategoryDTO[]
  saving: boolean
}>()

const emit = defineEmits<{
  save: [goal: DayGoalDTO]
}>()

const { t } = useI18n()

interface Draft {
  focusTargetMinutes: number
  distractionLimitMinutes: number
  isSkipped: boolean
  focusCategories: string[]
  distractionCategories: string[]
}

const draft = ref<Draft>({
  focusTargetMinutes: 0,
  distractionLimitMinutes: 0,
  isSkipped: false,
  focusCategories: [],
  distractionCategories: [],
})

function fromGoal(goal: DayGoalDTO | null): Draft {
  return {
    focusTargetMinutes: goal?.focusTargetMinutes ?? 0,
    distractionLimitMinutes: goal?.distractionLimitMinutes ?? 0,
    isSkipped: goal?.isSkipped ?? false,
    focusCategories: (goal?.focusCategories ?? []).map((ref) => ref.categoryId),
    distractionCategories: (goal?.distractionCategories ?? []).map((ref) => ref.categoryId),
  }
}

watch(() => props.goal, (next) => { draft.value = fromGoal(next) }, { immediate: true })

// System categories are excluded: they never carry user meaning as a target.
const selectableCategories = computed(() =>
  props.categories.filter((category) => !category.isSystem),
)

function categoryById(id: string): CategoryDTO | undefined {
  return selectableCategories.value.find((category) => category.id === id)
}

function toggle(list: string[], id: string): string[] {
  return list.includes(id) ? list.filter((item) => item !== id) : [...list, id]
}

// A category can be focus or distraction, not both.
function toggleFocus(id: string): void {
  const next = toggle(draft.value.focusCategories, id)
  draft.value.focusCategories = next
  if (next.includes(id)) {
    draft.value.distractionCategories = draft.value.distractionCategories.filter((item) => item !== id)
  }
}

function toggleDistraction(id: string): void {
  const next = toggle(draft.value.distractionCategories, id)
  draft.value.distractionCategories = next
  if (next.includes(id)) {
    draft.value.focusCategories = draft.value.focusCategories.filter((item) => item !== id)
  }
}

const canSave = computed(() => !props.saving && props.goal !== null)

function submit(): void {
  if (!canSave.value) return
  const toRefs = (ids: string[]) =>
    ids.map((id) => {
      const category = categoryById(id)
      return {
        categoryId: id,
        name: category?.name ?? '',
        colorHex: category?.colorHex ?? '',
        sortOrder: category?.sortOrder ?? 0,
      }
    })
  emit('save', {
    day: props.goal?.day ?? '',
    focusTargetMinutes: draft.value.focusTargetMinutes,
    distractionLimitMinutes: draft.value.distractionLimitMinutes,
    isSkipped: draft.value.isSkipped,
    focusCategories: toRefs(draft.value.focusCategories),
    distractionCategories: toRefs(draft.value.distractionCategories),
    exists: true,
  })
}
</script>

<template>
  <div class="goal-editor">
    <div class="goal-numbers">
      <label class="goal-number">
        <span>{{ t('daily.goal.focusTarget') }}</span>
        <input
          v-model.number="draft.focusTargetMinutes"
          class="dg-input"
          type="number"
          min="0"
          step="15"
        >
      </label>
      <label class="goal-number">
        <span>{{ t('daily.goal.distractionLimit') }}</span>
        <input
          v-model.number="draft.distractionLimitMinutes"
          class="dg-input"
          type="number"
          min="0"
          step="15"
        >
      </label>
      <label class="goal-skip">
        <input
          v-model="draft.isSkipped"
          class="dg-checkbox"
          type="checkbox"
        >
        <span>{{ t('daily.goal.skipped') }}</span>
      </label>
    </div>

    <div v-if="selectableCategories.length > 0" class="goal-categories">
      <div class="goal-category-group">
        <span class="goal-category-label">{{ t('daily.goal.focusCategories') }}</span>
        <div class="goal-chips" role="group" :aria-label="t('daily.goal.focusCategories')">
          <button
            v-for="category in selectableCategories"
            :key="category.id"
            type="button"
            class="goal-chip"
            :class="{ 'goal-chip--active': draft.focusCategories.includes(category.id) }"
            :style="{ '--chip-color': category.colorHex }"
            @click="toggleFocus(category.id)"
          >
            {{ categoryLabel(category.name, t) }}
          </button>
        </div>
      </div>
      <div class="goal-category-group">
        <span class="goal-category-label">{{ t('daily.goal.distractionCategories') }}</span>
        <div class="goal-chips" role="group" :aria-label="t('daily.goal.distractionCategories')">
          <button
            v-for="category in selectableCategories"
            :key="category.id"
            type="button"
            class="goal-chip"
            :class="{ 'goal-chip--active': draft.distractionCategories.includes(category.id) }"
            :style="{ '--chip-color': category.colorHex }"
            @click="toggleDistraction(category.id)"
          >
            {{ categoryLabel(category.name, t) }}
          </button>
        </div>
      </div>
    </div>
    <p v-else class="goal-empty">{{ t('daily.goal.noCategories') }}</p>

    <div class="goal-editor__actions">
      <button
        type="button"
        class="dg-button dg-button--primary"
        :disabled="!canSave"
        @click="submit"
      >
        {{ saving ? t('daily.goal.saving') : t('daily.goal.save') }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.goal-editor { display: grid; gap: 13px; }

.goal-numbers { display: flex; flex-wrap: wrap; align-items: flex-end; gap: 10px; }

.goal-number { display: grid; flex: 1; min-width: 90px; gap: 5px; }

.goal-number span { color: var(--dg-text-secondary); font-size: 10px; font-weight: 600; }

.goal-skip { display: flex; align-items: center; gap: 6px; color: var(--dg-text-secondary); font-size: 11px; }

.goal-categories { display: grid; gap: 12px; }

.goal-category-group { display: grid; gap: 6px; }

.goal-category-label { color: var(--dg-text-muted); font-size: 10px; font-weight: 600; }

.goal-chips { display: flex; flex-wrap: wrap; gap: 6px; }

.goal-chip {
  --chip-color: var(--dg-accent);
  padding: 4px 11px;
  border: 1px solid var(--dg-timeline-grid);
  border-radius: 99px;
  background: var(--dg-track-fill);
  color: var(--dg-text-secondary);
  font-size: 10px;
  transition: background var(--dg-motion-base) ease, border-color var(--dg-motion-base) ease;
}

.goal-chip--active {
  border-color: color-mix(in srgb, var(--chip-color) 55%, transparent);
  background: color-mix(in srgb, var(--chip-color) 14%, transparent);
  color: var(--dg-text-primary);
}

.goal-empty { margin: 0; color: var(--dg-text-muted); font-size: 11px; }

.goal-editor__actions { display: flex; justify-content: flex-end; }
</style>
