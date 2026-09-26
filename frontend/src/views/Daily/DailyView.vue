<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import DevelopmentBadge from '@/components/DevelopmentBadge.vue'
import PageHeader from '@/components/PageHeader.vue'
import PeriodNav from '@/components/PeriodNav.vue'
import { calendarDayQuery, shiftCalendarDate } from '@/lib/calendarDate'
import { safeTimeZone } from '@/lib/timeZone'
import { useDailyStore } from '@/stores/daily'

import DailyJournalPanel from './DailyJournalPanel.vue'
import DailyMetricsPanel from './DailyMetricsPanel.vue'
import DailyRecapPanel from './DailyRecapPanel.vue'
import DailyStatePanel from './DailyStatePanel.vue'
import DailyWorkflowOverview from './DailyWorkflowOverview.vue'

const daily = useDailyStore()
const {
  context,
  day,
  recap,
  recapUnavailable,
  recapError,
  recapGenerating,
  recapGenerateError,
  recapGenerationAvailable,
  journal,
  journalUnavailable,
  journalError,
  journalSaving,
  state,
  presentation,
  usingDevelopmentFixture,
  dayNavigationAvailable,
} = storeToRefs(daily)
const { locale, t } = useI18n()
const route = useRoute()
const router = useRouter()

const dateTitle = computed(() => {
  if (context.value === null) return t('daily.title')
  return new Intl.DateTimeFormat(locale.value, {
    weekday: 'short',
    month: 'long',
    day: 'numeric',
    timeZone: safeTimeZone(context.value.timeZone),
  }).format(new Date(context.value.dayStartTs * 1000))
})

const canNavigateBackward = computed(
  () => dayNavigationAvailable.value && context.value !== null,
)

const canNavigateForward = computed(
  () =>
    dayNavigationAvailable.value &&
    context.value !== null &&
    context.value.nowTs >= context.value.dayEndTs,
)

function routeDay(): string {
  return calendarDayQuery(route.query.day)
}

function navigate(offset: -1 | 1): void {
  const current = context.value
  if (current === null) return
  const target = shiftCalendarDate(current.day, offset)
  if (target === null) return
  void router.push({ name: 'daily', query: { ...route.query, day: target } })
}

function goToToday(): void {
  if (route.query.day === undefined) {
    void daily.load()
    return
  }
  const query = { ...route.query }
  delete query.day
  void router.push({ name: 'daily', query })
}

onMounted(() => {
  daily.startEvents()
})

watch(() => route.query.day, () => void daily.load(routeDay()), { immediate: true })

onBeforeUnmount(() => daily.stopListening())
</script>

<template>
  <div class="page daily-page">
    <PageHeader :title="dateTitle">
      <template #lead>
        <PeriodNav
          :label="t('daily.navigation.label')"
          :backward-title="dayNavigationAvailable ? t('common.action.previous') : t('daily.navigation.backendRequired')"
          :forward-title="!dayNavigationAvailable ? t('daily.navigation.backendRequired') : canNavigateForward ? t('common.action.next') : t('daily.navigation.futureUnavailable')"
          :can-backward="canNavigateBackward"
          :can-forward="canNavigateForward"
          :current-label="t('common.action.today')"
          :current-disabled="!dayNavigationAvailable && !usingDevelopmentFixture"
          @navigate="navigate"
          @current="goToToday"
        />
      </template>

      <template #trail>
        <DevelopmentBadge v-if="usingDevelopmentFixture">
          {{ t('daily.developmentFixture') }}
        </DevelopmentBadge>
        <span v-if="day" class="day-meta">
          {{ t('daily.meta.tracked', { count: day.trackedMinutes }) }}
        </span>
      </template>
    </PageHeader>

    <main class="daily-body dg-scroll">
      <div class="daily-content">
        <DailyStatePanel
          v-if="state !== 'populated' && state !== 'empty'"
          :state="state"
          @retry="daily.load(context?.day ?? '')"
        />

        <template v-else-if="context">
          <div class="daily-intro">
            <div>
              <span>{{ t('daily.overview.eyebrow') }}</span>
              <h2>{{ t('daily.overview.title') }}</h2>
            </div>
            <p>{{ t('daily.overview.description') }}</p>
          </div>

          <template v-if="presentation">
            <DailyWorkflowOverview
              :presentation="presentation"
              :time-zone="context.timeZone"
            />
            <DailyMetricsPanel :metrics="presentation.metrics" />
          </template>
          <DailyStatePanel
            v-else
            state="empty"
            @retry="daily.load(context.day)"
          />
          <DailyRecapPanel
            :recap="recap"
            :journal="journal"
            :unavailable="recapUnavailable"
            :failed="recapError !== null"
            :time-zone="context.timeZone"
            :day-start-ts="context.dayStartTs"
            :is-today="context.nowTs < context.dayEndTs"
            :generating="recapGenerating"
            :generate-failed="recapGenerateError !== null"
            :generation-available="recapGenerationAvailable"
            @regenerate="daily.regenerateRecap"
          />
          <DailyJournalPanel
            :journal="journal"
            :unavailable="journalUnavailable"
            :failed="journalError !== null"
            :saving="journalSaving"
            @save="daily.saveJournal"
          />
        </template>
      </div>
    </main>
  </div>
</template>

<style scoped>

.day-meta { color: var(--dg-text-muted); font-size: 10px; white-space: nowrap; }

.daily-body {
  flex: 1;
  min-height: 0;
  padding: 2px var(--dg-page-padding) var(--dg-page-padding);
}

.daily-content {
  display: flex;
  flex-direction: column;
  gap: 20px;
  width: 100%;
  max-width: var(--dg-daily-content-max);
  margin: 0 auto;
}

.daily-intro {
  display: grid;
  grid-template-columns: minmax(260px, 0.85fr) minmax(300px, 1.15fr);
  align-items: end;
  gap: 32px;
  padding: 4px 2px 2px;
}

.daily-intro span {
  display: inline-block;
  color: var(--dg-accent-text);
  font-size: 10px;
  font-weight: 650;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.daily-intro h2 {
  margin-top: 4px;
  color: var(--dg-text-primary);
  font-size: 22px;
  font-weight: 650;
  letter-spacing: -0.015em;
}

.daily-intro p { color: var(--dg-text-tertiary); font-size: 12px; line-height: 1.55; }

@media (max-width: 760px) {
  .day-meta { display: none; }
  .daily-body { padding-right: 16px; padding-left: 16px; }
  .daily-intro { grid-template-columns: minmax(0, 1fr); gap: 8px; }
}
</style>
