<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { computed, onBeforeUnmount, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'

import PageHeader from '@/components/PageHeader.vue'
import { useWeeklyStore } from '@/stores/weekly'

import WeeklyCategoryPanel from './WeeklyCategoryPanel.vue'
import WeeklyOverviewPanel from './WeeklyOverviewPanel.vue'
import WeeklyStatePanel from './WeeklyStatePanel.vue'
import { shiftCalendarDate } from '@/stores/weeklyPresentation'

const weekly = useWeeklyStore()
const {
  dashboard,
  state,
  presentation,
  usingDevelopmentFixture,
  navigationAvailable,
  canNavigateForward,
} = storeToRefs(weekly)
const { locale, t } = useI18n()

const dateTitle = computed(() => {
  const value = dashboard.value
  if (value === null) return t('weekly.title')

  const format = new Intl.DateTimeFormat(locale.value, {
    month: 'short',
    day: 'numeric',
    timeZone: 'UTC',
  })
  const endKey = shiftCalendarDate(value.weekStart, 6)
  const start = format.format(new Date(`${value.weekStart}T12:00:00Z`))
  if (endKey === null) return start
  const end = format.format(new Date(`${endKey}T12:00:00Z`))
  return `${start} – ${end}`
})

onMounted(() => {
  weekly.startEvents()
  void weekly.load()
})

onBeforeUnmount(() => weekly.stopListening())
</script>

<template>
  <div class="page weekly-page">
    <PageHeader :title="dateTitle">
      <template #lead>
        <div class="week-nav" role="group" :aria-label="t('weekly.navigation.label')">
          <button
            type="button"
            class="week-nav__arrow"
            :aria-label="t('common.action.previous')"
            :title="navigationAvailable ? t('common.action.previous') : t('weekly.navigation.backendRequired')"
            :disabled="!navigationAvailable"
            @click="weekly.navigate(-1)"
          >
            ‹
          </button>
          <button
            type="button"
            class="week-nav__arrow"
            :aria-label="t('common.action.next')"
            :title="navigationAvailable ? t('common.action.next') : t('weekly.navigation.backendRequired')"
            :disabled="!canNavigateForward"
            @click="weekly.navigate(1)"
          >
            ›
          </button>
          <button type="button" class="dg-chip dg-chip--filled" @click="weekly.load()">
            {{ t('weekly.navigation.current') }}
          </button>
        </div>
      </template>

      <template #trail>
        <span v-if="usingDevelopmentFixture" class="development-badge">
          {{ t('weekly.developmentFixture') }}
        </span>
        <span v-if="dashboard" class="week-meta">
          {{ t('weekly.meta.tracked', { count: dashboard.trackedMinutes }) }}
        </span>
      </template>
    </PageHeader>

    <main class="weekly-body dg-scroll">
      <div class="weekly-content">
        <WeeklyStatePanel
          v-if="state !== 'populated'"
          :state="state"
          @retry="weekly.load(dashboard?.weekStart ?? '')"
        />

        <template v-else-if="presentation">
          <div class="weekly-intro">
            <div>
              <span>{{ t('weekly.intro.eyebrow') }}</span>
              <h2>{{ t('weekly.intro.title') }}</h2>
            </div>
            <p>{{ t('weekly.intro.description') }}</p>
          </div>

          <WeeklyOverviewPanel :presentation="presentation" />
          <WeeklyCategoryPanel :categories="presentation.categories" />

          <p class="weekly-scope-note">{{ t('weekly.scopeNote') }}</p>
        </template>
      </div>
    </main>
  </div>
</template>

<style scoped>
.week-nav { display: flex; align-items: center; gap: 5px; }

.week-nav__arrow {
  display: grid;
  width: 28px;
  height: 28px;
  border-radius: 50%;
  color: var(--dg-text-secondary);
  font-size: 25px;
  line-height: 1;
  place-items: center;
}

.week-nav__arrow:not(:disabled):hover { background: var(--dg-hover-fill); }

.week-nav__arrow:focus-visible {
  outline: none;
  box-shadow: 0 0 0 3px var(--dg-focus-ring);
}

.week-nav__arrow:disabled { color: var(--dg-text-muted); cursor: default; opacity: 0.5; }

.development-badge {
  padding: 3px 7px;
  border: 1px solid var(--dg-chip-border);
  border-radius: 5px;
  background: var(--dg-hover-fill);
  color: var(--dg-text-tertiary);
  font-size: 10px;
  font-weight: 600;
}

.week-meta { color: var(--dg-text-muted); font-size: 10px; white-space: nowrap; }

.weekly-body {
  flex: 1;
  min-height: 0;
  padding: 2px var(--dg-page-padding) var(--dg-page-padding);
}

.weekly-content {
  display: flex;
  flex-direction: column;
  gap: 20px;
  width: 100%;
  max-width: var(--dg-weekly-content-max);
  margin: 0 auto;
}

.weekly-intro {
  display: grid;
  grid-template-columns: minmax(260px, 0.85fr) minmax(300px, 1.15fr);
  align-items: end;
  gap: 32px;
  padding: 4px 2px 2px;
}

.weekly-intro span {
  color: var(--dg-accent-text);
  font-size: 11px;
  font-weight: 650;
}

.weekly-intro h2 {
  margin-top: 4px;
  color: var(--dg-text-primary);
  font-size: 22px;
  font-weight: 650;
  letter-spacing: -0.015em;
}

.weekly-intro p { color: var(--dg-text-tertiary); font-size: 12px; line-height: 1.55; }

.weekly-scope-note {
  padding: 0 2px 6px;
  color: var(--dg-text-muted);
  font-size: 10px;
  line-height: 1.5;
}

@media (max-width: 760px) {
  .week-meta { display: none; }
  .weekly-body { padding-right: 16px; padding-left: 16px; }
  .weekly-intro { grid-template-columns: minmax(0, 1fr); gap: 8px; }
}
</style>
