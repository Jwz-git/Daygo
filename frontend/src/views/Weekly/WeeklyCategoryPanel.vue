<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import type { WeeklyCategoryPresentation } from '@/stores/weeklyPresentation'
import { percentageLabel } from '@/stores/weeklyPresentation'

defineProps<{ categories: WeeklyCategoryPresentation[] }>()
const { t } = useI18n()

function duration(minutes: number): string {
  const rounded = Math.max(0, Math.round(minutes))
  if (rounded < 60) return t('weekly.duration.minutes', { count: rounded })
  const hours = Math.floor(rounded / 60)
  const remainder = rounded % 60
  return remainder === 0
    ? t('weekly.duration.hours', { count: hours })
    : t('weekly.duration.hoursMinutes', { hours, minutes: remainder })
}
</script>

<template>
  <section class="categories dg-card" :aria-label="t('weekly.categories.title')">
    <header class="categories__header">
      <div>
        <p>{{ t('weekly.categories.eyebrow') }}</p>
        <h2>{{ t('weekly.categories.title') }}</h2>
      </div>
      <span>{{ t('weekly.categories.count', { count: categories.length }) }}</span>
    </header>

    <div class="distribution" role="img" :aria-label="t('weekly.categories.distributionAria')">
      <span
        v-for="category in categories"
        :key="category.name"
        :class="`series-${category.seriesIndex + 1}`"
        :style="{ flexGrow: category.share }"
        :title="`${category.name} · ${percentageLabel(category.share)}`"
      />
    </div>

    <ol class="category-list">
      <li v-for="(category, index) in categories" :key="category.name">
        <span class="category-list__rank">{{ String(index + 1).padStart(2, '0') }}</span>
        <span :class="['category-list__dot', `series-${category.seriesIndex + 1}`]" />
        <div class="category-list__identity">
          <strong>{{ category.name }}</strong>
          <div class="category-list__track" aria-hidden="true">
            <span
              :class="`series-${category.seriesIndex + 1}`"
              :style="{ width: percentageLabel(category.share) }"
            />
          </div>
        </div>
        <span class="category-list__duration">{{ duration(category.minutes) }}</span>
        <span class="category-list__share">{{ percentageLabel(category.share) }}</span>
      </li>
    </ol>
  </section>
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

.distribution {
  display: flex;
  gap: 3px;
  height: 10px;
  margin: 22px 0 18px;
  overflow: hidden;
  border-radius: 4px;
  background: var(--dg-weekly-bar-track);
}

.distribution > span { min-width: 4px; border-radius: 2px; }

.category-list { list-style: none; }

.category-list li {
  display: grid;
  grid-template-columns: 24px 8px minmax(150px, 1fr) minmax(80px, auto) 44px;
  align-items: center;
  gap: 11px;
  min-height: 54px;
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
  grid-template-columns: minmax(84px, 0.55fr) minmax(90px, 1fr);
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

.series-1 { background: var(--dg-weekly-series-1); }
.series-2 { background: var(--dg-weekly-series-2); }
.series-3 { background: var(--dg-weekly-series-3); }
.series-4 { background: var(--dg-weekly-series-4); }
.series-5 { background: var(--dg-weekly-series-5); }
.series-6 { background: var(--dg-weekly-series-6); }

@media (max-width: 640px) {
  .category-list li { grid-template-columns: 18px 8px minmax(110px, 1fr) 44px; }
  .category-list__identity { display: block; }
  .category-list__track { margin-top: 7px; }
  .category-list__duration { display: none; }
}
</style>
