<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { computed, onBeforeUnmount, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'

import DevelopmentBadge from '@/components/DevelopmentBadge.vue'
import PageHeader from '@/components/PageHeader.vue'
import PeriodNav from '@/components/PeriodNav.vue'
import { shiftCalendarDate } from '@/lib/calendarDate'
import { useWeeklyStore } from '@/stores/weekly'

import WeeklyCategoryPanel from './WeeklyCategoryPanel.vue'
import WeeklyDailyTimelinePanel from './WeeklyDailyTimelinePanel.vue'
import WeeklyInsightsPanel from './WeeklyInsightsPanel.vue'
import WeeklyOverviewPanel from './WeeklyOverviewPanel.vue'
import WeeklyRhythmPanel from './WeeklyRhythmPanel.vue'
import WeeklyStatePanel from './WeeklyStatePanel.vue'
import WeeklyContextCard from './charts/WeeklyContextCard.vue'
import WeeklyDistributionCard from './charts/WeeklyDistributionCard.vue'
import WeeklyHeatmapCard from './charts/WeeklyHeatmapCard.vue'
import WeeklySankeyCard from './charts/WeeklySankeyCard.vue'
import WeeklyTreemapCard from './charts/WeeklyTreemapCard.vue'
import WeeklyWorkflowCard from './charts/WeeklyWorkflowCard.vue'

const weekly = useWeeklyStore()
const {
  dashboard,
  state,
  presentation,
  charts,
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

// Logical-day strings of the week, Monday first, for the charts' weekday labels.
const weekDays = computed(() => dashboard.value?.days.map((day) => day.day) ?? [])

onMounted(() => {
  weekly.startEvents()
  void weekly.load()
})

onBeforeUnmount(() => weekly.stopListening())
</script>

<template>
  <div class="page weekly-page">
    <PageHeader :title="dateTitle" hide-title>
      <template #lead>
        <PeriodNav
          :label="t('weekly.navigation.label')"
          :backward-title="navigationAvailable ? t('common.action.previous') : t('weekly.navigation.backendRequired')"
          :forward-title="navigationAvailable ? t('common.action.next') : t('weekly.navigation.backendRequired')"
          :can-backward="navigationAvailable"
          :can-forward="canNavigateForward"
          :current-label="t('weekly.navigation.current')"
          @navigate="(offset) => weekly.navigate(offset)"
          @current="weekly.load()"
        />
        <h1 class="dg-page-date dg-display">{{ dateTitle }}</h1>
      </template>

      <template #trail>
        <DevelopmentBadge v-if="usingDevelopmentFixture">
          {{ t('weekly.developmentFixture') }}
        </DevelopmentBadge>
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

          <!-- Dayflow's weekly charts first, in Dayflow's order; Daygo's own
               panels follow. -->
          <template v-if="charts">
            <div class="weekly-top-row">
              <WeeklyDistributionCard :presentation="presentation" :days="weekDays" />
              <WeeklyContextCard :snapshot="charts.context" :days="weekDays" />
            </div>
            <WeeklyWorkflowCard :snapshot="charts.workflow" :days="weekDays" />
            <WeeklyHeatmapCard :snapshot="charts.heatmap" :days="weekDays" />
            <WeeklyTreemapCard :categories="charts.treemap" :days="weekDays" />
            <WeeklySankeyCard :snapshot="charts.sankey" :week-label="dateTitle" :days="weekDays" />
          </template>

          <WeeklyOverviewPanel :presentation="presentation" />
          <WeeklyCategoryPanel :presentation="presentation" />
          <WeeklyDailyTimelinePanel :presentation="presentation" />
          <WeeklyRhythmPanel :presentation="presentation" />
          <WeeklyInsightsPanel :presentation="presentation" />

          <p class="weekly-scope-note">{{ t('weekly.scopeNote') }}</p>
        </template>
      </div>
    </main>
  </div>
</template>

<style scoped>

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
  display: inline-block;
  color: var(--dg-weekly-tag-text);
  font-size: 10px;
  font-weight: 650;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.weekly-intro h2 {
  margin-top: 8px;
  color: var(--dg-text-primary);
  font-family: var(--dg-font-serif);
  font-size: 30px;
  font-weight: 400;
  letter-spacing: 0;
}

.weekly-intro p { color: var(--dg-text-tertiary); font-size: 12px; line-height: 1.55; }

.weekly-scope-note {
  padding: 0 2px 6px;
  color: var(--dg-text-muted);
  font-size: 10px;
  line-height: 1.5;
}

/* Dayflow pairs the distribution donut with the context charts in one row. */
.weekly-top-row {
  display: grid;
  grid-template-columns: minmax(380px, 1fr) minmax(0, 1.9fr);
  gap: 20px;
}

@media (max-width: 1000px) {
  .weekly-top-row { grid-template-columns: minmax(0, 1fr); }
}

@media (max-width: 760px) {
  .week-meta { display: none; }
  .weekly-body { padding-right: 16px; padding-left: 16px; }
  .weekly-intro { grid-template-columns: minmax(0, 1fr); gap: 8px; }
}
</style>
