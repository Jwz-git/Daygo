<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import LiquidGlassSurface from '@/components/LiquidGlassSurface.vue'
import { categoryLabel } from '@/lib/categoryLabel'
import { useDurationFormat } from '@/lib/duration'
import type { WeeklyCategoryPresentation, WeeklyPresentation } from '@/stores/weeklyPresentation'
import { percentageLabel } from '@/stores/weeklyPresentation'

const props = defineProps<{ presentation: WeeklyPresentation }>()
const { t } = useI18n()

const duration = useDurationFormat()

const categories = computed(() => props.presentation.categories)

// SVG donut: 100-unit viewBox, stroke-dasharray arcs. Angles start at 12
// o'clock; a 1.5% gap between slices keeps them legible.
const RADIUS = 15.9155 // circumference 100 at r=15.9155
const GAP = 1.6

const arcs = computed(() => {
  let cumulative = 0
  return categories.value.map((category) => {
    const sweep = Math.max(0, category.share * 100 - GAP)
    const arc = {
      category,
      offset: 25 - cumulative, // SVG stroke starts at 3 o'clock; rotate to 12
      length: sweep,
    }
    cumulative += category.share * 100
    return arc
  })
})

const totalDuration = computed(() => duration(props.presentation.trackedMinutes))

function sliceColor(category: WeeklyCategoryPresentation): string {
  return `var(--dg-weekly-series-${category.seriesIndex + 1})`
}
</script>

<template>
  <LiquidGlassSurface intensity="air" as="section" class="categories" :aria-label="t('weekly.categories.title')">
    <header class="categories__header">
      <div>
        <p>{{ t('weekly.categories.eyebrow') }}</p>
        <h2>{{ t('weekly.categories.title') }}</h2>
      </div>
      <span>{{ t('weekly.categories.count', { count: categories.length }) }}</span>
    </header>

    <div class="categories__body">
      <div class="donut" role="img" :aria-label="t('weekly.categories.distributionAria')">
        <svg viewBox="0 0 42 42" class="donut__svg">
          <circle class="donut__track" cx="21" cy="21" :r="RADIUS" fill="none" />
          <circle
            v-for="arc in arcs"
            :key="arc.category.name"
            class="donut__slice"
            :class="{ 'donut__slice--idle': arc.category.name === 'Idle' }"
            cx="21"
            cy="21"
            :r="RADIUS"
            fill="none"
            :stroke="sliceColor(arc.category)"
            :stroke-dasharray="`${arc.length} ${100 - arc.length}`"
            :stroke-dashoffset="arc.offset"
            :title="`${categoryLabel(arc.category.name, t)} · ${percentageLabel(arc.category.share)}`"
          />
        </svg>
        <div class="donut__center">
          <strong>{{ totalDuration }}</strong>
          <span>{{ t('weekly.categories.total') }}</span>
        </div>
      </div>

      <ol class="category-list">
        <li v-for="(category, index) in categories" :key="category.name">
          <span class="category-list__rank">{{ String(index + 1).padStart(2, '0') }}</span>
          <span
            class="category-list__dot"
            :style="{ background: sliceColor(category) }"
          />
          <div class="category-list__identity">
            <strong>{{ categoryLabel(category.name, t) }}</strong>
            <div class="category-list__track" aria-hidden="true">
              <span
                :style="{ width: percentageLabel(category.share), background: sliceColor(category) }"
              />
            </div>
          </div>
          <span class="category-list__duration">{{ duration(category.minutes) }}</span>
          <span class="category-list__share">{{ percentageLabel(category.share) }}</span>
        </li>
      </ol>
    </div>
  </LiquidGlassSurface>
</template>

<style scoped>
.categories { padding: 24px 26px 14px; }

.categories__header {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 20px;
}

.categories__header p {
  color: var(--dg-accent-text);
  font-size: 10px;
  font-weight: 650;
  letter-spacing: 0.02em;
}

.categories__header h2 {
  margin-top: 4px;
  color: var(--dg-text-primary);
  font-size: 18px;
  font-weight: 650;
  letter-spacing: -0.012em;
}

.categories__header > span { color: var(--dg-text-muted); font-size: 11px; }

.categories__body {
  display: grid;
  grid-template-columns: 168px minmax(0, 1fr);
  gap: 30px;
  align-items: center;
  padding: 18px 0 12px;
}

.donut { position: relative; width: 168px; height: 168px; }

.donut__svg {
  width: 100%;
  height: 100%;
  transform: rotate(0deg);
}

.donut__track {
  stroke: var(--dg-weekly-bar-track);
  stroke-width: 5;
}

.donut__slice {
  stroke-width: 5;
  stroke-linecap: butt;
}

.donut__slice--idle { opacity: 0.55; }

.donut__center {
  position: absolute;
  inset: 0;
  display: grid;
  place-content: center;
  text-align: center;
}

.donut__center strong {
  color: var(--dg-text-primary);
  font-size: 16px;
  font-weight: 620;
  letter-spacing: -0.02em;
}

.donut__center span {
  margin-top: 2px;
  color: var(--dg-text-muted);
  font-size: 9px;
}

.category-list { list-style: none; }

.category-list li {
  display: grid;
  grid-template-columns: 24px 8px minmax(120px, 1fr) minmax(72px, auto) 44px;
  align-items: center;
  gap: 11px;
  min-height: 46px;
  border-top: 1px solid var(--dg-card-border);
}

.category-list__rank { color: var(--dg-text-muted); font-size: 10px; font-variant-numeric: tabular-nums; }

.category-list__dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
}

.category-list__identity {
  display: grid;
  grid-template-columns: minmax(84px, 0.55fr) minmax(80px, 1fr);
  align-items: center;
  gap: 18px;
  min-width: 0;
}

.category-list__identity strong {
  overflow: hidden;
  color: var(--dg-text-primary);
  font-size: 12px;
  font-weight: 560;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.category-list__track {
  height: 5px;
  overflow: hidden;
  border-radius: 2px;
  background: var(--dg-weekly-bar-track);
}

.category-list__track span { display: block; height: 100%; border-radius: 2px; }

.category-list__duration,
.category-list__share {
  color: var(--dg-text-secondary);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
  text-align: right;
  white-space: nowrap;
}

.category-list__share { color: var(--dg-text-muted); }

@media (prefers-reduced-motion: no-preference) {
  .donut__slice {
    transform-origin: 21px 21px;
    animation: donut-fade 600ms var(--dg-ease-glide) both;
  }

  .category-list__track span {
    transform-origin: 0 50%;
    animation: weekly-bar-grow 560ms var(--dg-ease-glide) both;
  }
}

@keyframes donut-fade {
  from { opacity: 0; }
}

@keyframes weekly-bar-grow {
  from {
    transform: scaleX(0);
  }
}

@media (max-width: 760px) {
  .categories__body { grid-template-columns: minmax(0, 1fr); justify-items: center; }
  .category-list { width: 100%; }
}

@media (max-width: 640px) {
  .category-list li { grid-template-columns: 18px 8px minmax(110px, 1fr) 44px; }
  .category-list__identity { display: block; }
  .category-list__track { margin-top: 7px; }
  .category-list__duration { display: none; }
}
</style>
