<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import type { CategoryDTO, DayGoalDTO } from '@/api/dto'

const props = defineProps<{
  goal: DayGoalDTO | null
  categories: CategoryDTO[]
  unavailable: boolean
  failed: boolean
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
  <section class="daily-section" aria-labelledby="daily-goal-title">
    <header class="section-heading">
      <div>
        <h2 id="daily-goal-title">{{ t('daily.goal.title') }}</h2>
        <p>{{ t('daily.goal.description') }}</p>
      </div>
      <button
        type="button"
        class="dg-button dg-button--primary"
        :disabled="!canSave"
        @click="submit"
      >
        {{ saving ? t('daily.goal.saving') : t('daily.goal.save') }}
      </button>
    </header>

    <div v-if="unavailable || failed" class="goal-state dg-card">
      <strong>{{ failed ? t('daily.goal.failureTitle') : t('daily.goal.unavailableTitle') }}</strong>
      <span>{{ failed ? t('daily.goal.failureDescription') : t('daily.goal.unavailableDescription') }}</span>
    </div>

    <div v-else class="goal-card dg-card">
      <div class="goal-numbers">
        <label class="goal-number">
          <span>{{ t('daily.goal.focusTarget') }}</span>
          <input
            v-model.number="draft.focusTargetMinutes"
            type="number"
            min="0"
            step="15"
          >
        </label>
        <label class="goal-number">
          <span>{{ t('daily.goal.distractionLimit') }}</span>
          <input
            v-model.number="draft.distractionLimitMinutes"
            type="number"
            min="0"
            step="15"
          >
        </label>
        <label class="goal-skip">
          <input
            v-model="draft.isSkipped"
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
              {{ category.name }}
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
              {{ category.name }}
            </button>
          </div>
        </div>
      </div>
      <p v-else class="goal-empty">{{ t('daily.goal.noCategories') }}</p>
    </div>
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

.goal-state { display: flex; flex-direction: column; gap: 6px; padding: 18px; }

.goal-card { display: flex; flex-direction: column; gap: 16px; padding: 16px; }

.goal-numbers {
  display: grid;
  grid-template-columns: repeat(2, minmax(140px, 200px)) auto;
  gap: 14px;
  align-items: end;
}

.goal-number { display: flex; flex-direction: column; gap: 6px; }

.goal-number span {
  color: var(--dg-text-secondary);
  font-size: 11px;
  font-weight: 600;
}

.goal-number input {
  width: 100%;
  border: 1px solid var(--dg-chip-border);
  border-radius: 8px;
  background: var(--dg-surface);
  color: var(--dg-text-primary);
  font: inherit;
  font-size: 12px;
  padding: 8px 10px;
}

.goal-number input:focus-visible {
  outline: none;
  border-color: var(--dg-accent, var(--dg-focus-ring));
  box-shadow: 0 0 0 3px var(--dg-focus-ring);
}

.goal-skip { display: flex; align-items: center; gap: 7px; padding-bottom: 8px; }

.goal-skip span { color: var(--dg-text-secondary); font-size: 12px; }

.goal-categories { display: flex; flex-direction: column; gap: 12px; }

.goal-category-group { display: flex; flex-direction: column; gap: 6px; }

.goal-category-label {
  color: var(--dg-text-secondary);
  font-size: 11px;
  font-weight: 600;
}

.goal-chips { display: flex; flex-wrap: wrap; gap: 6px; }

.goal-chip {
  display: inline-flex;
  align-items: center;
  padding: 4px 11px;
  border: 1px solid var(--dg-chip-border);
  border-radius: 999px;
  background: var(--dg-hover-fill);
  color: var(--dg-text-secondary);
  font-size: 11px;
  font-weight: 600;
}

.goal-chip--active {
  border-color: var(--chip-color, var(--dg-accent));
  color: var(--dg-text-primary);
  background: color-mix(in srgb, var(--chip-color, var(--dg-accent)) 18%, transparent);
}

.goal-chip:focus-visible {
  outline: none;
  box-shadow: 0 0 0 3px var(--dg-focus-ring);
}

.goal-empty { color: var(--dg-text-muted); font-size: 12px; }

@media (max-width: 760px) {
  .section-heading { flex-direction: column; align-items: flex-start; gap: 8px; }
  .goal-numbers { grid-template-columns: minmax(0, 1fr); }
}
</style>
