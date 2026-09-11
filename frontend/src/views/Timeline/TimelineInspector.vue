<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { CategoryDTO, TimelineCardDTO, TimelineDayDTO } from '@/api/dto'

import { safeCategoryColor } from './layout'

const props = defineProps<{
  day: TimelineDayDTO
  card: TimelineCardDTO | null
  canWrite: boolean
}>()

const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()

interface CategoryTotal {
  category: CategoryDTO
  minutes: number
  percentage: number
}

const categoryTotals = computed<CategoryTotal[]>(() => {
  const totals = new Map<string, number>()
  for (const card of props.day.cards) {
    if (card.isIdle || card.category === 'System') continue
    totals.set(card.category, (totals.get(card.category) ?? 0) + card.durationMinutes)
  }

  const denominator = Math.max(1, [...totals.values()].reduce((sum, value) => sum + value, 0))
  return props.day.categories
    .filter((category) => totals.has(category.name))
    .map((category) => {
      const minutes = totals.get(category.name) ?? 0
      return { category, minutes, percentage: (minutes / denominator) * 100 }
    })
    .sort((left, right) => right.minutes - left.minutes)
})

const selectedColor = computed(() => {
  const category = props.day.categories.find((entry) => entry.name === props.card?.category)
  return safeCategoryColor(category?.colorHex)
})

function duration(minutes: number): string {
  const hours = Math.floor(minutes / 60)
  const rest = minutes % 60
  if (hours === 0) return t('timeline.duration.minutes', { count: rest })
  if (rest === 0) return t('timeline.duration.hours', { count: hours })
  return t('timeline.duration.hoursMinutes', { hours, minutes: rest })
}
</script>

<template>
  <aside class="inspector dg-card" :aria-label="t('timeline.inspector.title')">
    <template v-if="props.card === null">
      <header class="inspector__header">
        <div>
          <p class="inspector__eyebrow">{{ t('timeline.overview.eyebrow') }}</p>
          <h2 class="inspector__title dg-display">{{ t('timeline.overview.title') }}</h2>
        </div>
      </header>

      <div class="totals">
        <div class="total total--primary">
          <strong>{{ duration(props.day.trackedMinutes) }}</strong>
          <span>{{ t('timeline.overview.tracked') }}</span>
        </div>
        <div class="total">
          <strong>{{ duration(props.day.idleMinutes) }}</strong>
          <span>{{ t('timeline.overview.idle') }}</span>
        </div>
      </div>

      <div class="category-list">
        <p v-if="categoryTotals.length === 0" class="category-list__empty">
          {{ t('timeline.overview.noCategories') }}
        </p>
        <div v-for="item in categoryTotals" :key="item.category.id" class="category-total">
          <div class="category-total__meta">
            <span>
              <i :style="{ background: safeCategoryColor(item.category.colorHex) }"></i>
              {{ item.category.name }}
            </span>
            <strong>{{ duration(item.minutes) }}</strong>
          </div>
          <div class="category-total__track" aria-hidden="true">
            <span
              :style="{
                width: `${item.percentage}%`,
                background: safeCategoryColor(item.category.colorHex),
              }"
            ></span>
          </div>
        </div>
      </div>
    </template>

    <template v-else>
      <header class="inspector__header">
        <div>
          <p class="inspector__eyebrow">{{ props.card.category }}</p>
          <h2 class="inspector__title inspector__title--card">{{ props.card.title }}</h2>
        </div>
        <button
          type="button"
          class="inspector__close"
          :aria-label="t('timeline.inspector.close')"
          @click="emit('close')"
        >
          ×
        </button>
      </header>

      <div class="card-time">
        <span :style="{ background: selectedColor }"></span>
        {{ props.card.start }} – {{ props.card.end }} · {{ duration(props.card.durationMinutes) }}
      </div>

      <section class="inspector__section">
        <h3>{{ t('timeline.inspector.summary') }}</h3>
        <p>{{ props.card.detailedSummary || props.card.summary || t('timeline.inspector.noSummary') }}</p>
      </section>

      <section v-if="props.card.appSites" class="inspector__section">
        <h3>{{ t('timeline.inspector.apps') }}</h3>
        <div class="tags">
          <span v-if="props.card.appSites.primary">{{ props.card.appSites.primary }}</span>
          <span v-if="props.card.appSites.secondary">{{ props.card.appSites.secondary }}</span>
        </div>
      </section>

      <section v-if="props.card.distractions.length > 0" class="inspector__section">
        <h3>{{ t('timeline.inspector.distractions') }}</h3>
        <article
          v-for="distraction in props.card.distractions"
          :key="distraction.id"
          class="distraction"
        >
          <span>{{ distraction.startTime }} – {{ distraction.endTime }}</span>
          <strong>{{ distraction.title }}</strong>
        </article>
      </section>

      <section class="inspector__section inspector__section--frames">
        <div>
          <h3>{{ t('timeline.inspector.frames') }}</h3>
          <p>{{ t('timeline.inspector.framesUnavailable') }}</p>
        </div>
        <div class="frame-placeholder" aria-hidden="true">
          <span></span><span></span><span></span>
        </div>
      </section>

      <div class="inspector__actions" :title="t('timeline.inspector.actionsUnavailable')">
        <button type="button" class="dg-button" disabled>{{ t('common.action.edit') }}</button>
        <button type="button" class="dg-button" disabled>{{ t('common.action.delete') }}</button>
        <span v-if="!props.canWrite" class="inspector__readonly">
          {{ t('timeline.inspector.readOnly') }}
        </span>
      </div>
    </template>
  </aside>
</template>

<style scoped>
.inspector {
  min-height: 0;
  padding: 22px;
  overflow-y: auto;
  background: var(--dg-timeline-inspector-fill);
}

.inspector__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding-bottom: 18px;
  border-bottom: 1px solid var(--dg-timeline-grid);
}

.inspector__eyebrow {
  margin-bottom: 5px;
  color: var(--dg-accent-text);
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.13em;
  text-transform: uppercase;
}

.inspector__title {
  color: var(--dg-text-primary);
  font-size: 26px;
  line-height: 1.1;
}

.inspector__title--card {
  font-size: 18px;
  font-weight: 650;
  line-height: 1.3;
}

.inspector__close {
  display: grid;
  flex: none;
  width: 28px;
  height: 28px;
  border-radius: 50%;
  color: var(--dg-text-secondary);
  font-size: 20px;
  line-height: 1;
  place-items: center;
}

.inspector__close:hover { background: var(--dg-hover-fill); }

.totals {
  display: grid;
  grid-template-columns: 1.35fr 1fr;
  gap: 8px;
  padding: 18px 0 22px;
}

.total {
  display: flex;
  flex-direction: column;
  min-width: 0;
  padding: 13px;
  border: 1px solid var(--dg-timeline-grid);
  border-radius: 8px;
  background: var(--dg-track-fill);
}

.total strong {
  overflow: hidden;
  color: var(--dg-text-primary);
  font-size: 16px;
  font-weight: 650;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.total span,
.category-list__empty {
  color: var(--dg-text-muted);
  font-size: 10px;
}

.total--primary {
  border-color: color-mix(in srgb, var(--dg-accent) 22%, transparent);
  background: var(--dg-control-fill);
}

.category-list { display: grid; gap: 15px; }
.category-total__meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 6px;
  color: var(--dg-text-secondary);
  font-size: 11px;
}

.category-total__meta span { display: flex; align-items: center; gap: 7px; min-width: 0; }
.category-total__meta i { width: 7px; height: 7px; border-radius: 50%; }
.category-total__meta strong { color: var(--dg-text-primary); font-weight: 600; }
.category-total__track { height: 4px; overflow: hidden; border-radius: 99px; background: var(--dg-track-fill); }
.category-total__track span { display: block; height: 100%; border-radius: inherit; }

.card-time {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 14px 0;
  color: var(--dg-text-secondary);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}

.card-time span { width: 7px; height: 7px; border-radius: 50%; }

.inspector__section { padding: 16px 0; border-top: 1px solid var(--dg-timeline-grid); }
.inspector__section:first-of-type { border-top: 0; }
.inspector__section h3 { margin-bottom: 6px; color: var(--dg-text-primary); font-size: 11px; font-weight: 650; }
.inspector__section p { color: var(--dg-text-secondary); font-size: 11px; line-height: 1.6; }
.tags { display: flex; flex-wrap: wrap; gap: 6px; }
.tags span { padding: 4px 8px; border-radius: 99px; background: var(--dg-track-fill); color: var(--dg-text-secondary); font-size: 10px; }
.distraction { display: grid; gap: 2px; padding: 8px 0; }
.distraction span { color: var(--dg-text-muted); font-size: 9px; }
.distraction strong { color: var(--dg-text-secondary); font-size: 11px; font-weight: 550; }

.inspector__section--frames { display: grid; gap: 12px; }
.frame-placeholder { display: grid; grid-template-columns: repeat(3, 1fr); gap: 5px; }
.frame-placeholder span { height: 52px; border: 1px solid var(--dg-timeline-grid); border-radius: 5px; background: var(--dg-timeline-frame-fill); }
.inspector__actions { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; padding-top: 18px; border-top: 1px solid var(--dg-timeline-grid); }
.inspector__readonly { width: 100%; color: var(--dg-text-muted); font-size: 10px; }
</style>
