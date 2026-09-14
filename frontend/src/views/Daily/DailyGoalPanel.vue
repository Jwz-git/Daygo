<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import type { CategoryDTO, DayGoalDTO } from '@/api/dto'
import GoalEditor from '@/views/Timeline/GoalEditor.vue'

/* The daily page's goal panel: shell (heading, save button slot, degraded
   states) around the shared GoalEditor form. */
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
</script>

<template>
  <section class="daily-section" aria-labelledby="daily-goal-title">
    <header class="section-heading">
      <div>
        <h2 id="daily-goal-title">{{ t('daily.goal.title') }}</h2>
        <p>{{ t('daily.goal.description') }}</p>
      </div>
    </header>

    <div v-if="unavailable || failed" class="goal-state dg-card">
      <strong>{{ failed ? t('daily.goal.failureTitle') : t('daily.goal.unavailableTitle') }}</strong>
      <span>{{ failed ? t('daily.goal.failureDescription') : t('daily.goal.unavailableDescription') }}</span>
    </div>

    <div v-else class="goal-card dg-card">
      <GoalEditor
        :goal="goal"
        :categories="categories"
        :saving="saving"
        @save="(next) => emit('save', next)"
      />
    </div>
  </section>
</template>

<style scoped>
.daily-section { display: flex; flex-direction: column; gap: 10px; }

.section-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.section-heading h2 { font-size: 15px; font-weight: 650; }
.section-heading p { color: var(--dg-text-muted); font-size: 11px; }

.goal-state { display: grid; gap: 5px; padding: 16px; }
.goal-state strong { color: var(--dg-text-primary); font-size: 12px; }
.goal-state span { color: var(--dg-text-muted); font-size: 11px; }
</style>
