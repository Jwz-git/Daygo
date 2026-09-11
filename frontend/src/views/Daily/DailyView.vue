<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { useI18n } from 'vue-i18n'

import PageHeader from '@/components/PageHeader.vue'
import { useDailyStore } from '@/stores/daily'

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
  state,
  presentation,
  usingDevelopmentFixture,
  dayNavigationAvailable,
} = storeToRefs(daily)
const { locale, t } = useI18n()

const dateTitle = computed(() => {
  if (context.value === null) return t('daily.title')
  return new Intl.DateTimeFormat(locale.value, {
    weekday: 'short',
    month: 'long',
    day: 'numeric',
    timeZone: context.value.timeZone,
  }).format(new Date(context.value.dayStartTs * 1000))
})

onMounted(() => {
  daily.startEvents()
  void daily.load()
})

onBeforeUnmount(() => daily.stopListening())
</script>

<template>
  <div class="page daily-page">
    <PageHeader :title="dateTitle">
      <template #lead>
        <div class="date-nav" role="group" :aria-label="t('daily.navigation.label')">
          <button
            type="button"
            class="date-nav__arrow"
            :aria-label="t('common.action.previous')"
            :title="t('daily.navigation.backendRequired')"
            disabled
          >
            ‹
          </button>
          <button
            type="button"
            class="date-nav__arrow"
            :aria-label="t('common.action.next')"
            :title="t('daily.navigation.backendRequired')"
            disabled
          >
            ›
          </button>
          <button
            type="button"
            class="dg-chip dg-chip--filled"
            :disabled="!dayNavigationAvailable && !usingDevelopmentFixture"
            @click="daily.load()"
          >
            {{ t('common.action.today') }}
          </button>
        </div>
      </template>

      <template #trail>
        <span v-if="usingDevelopmentFixture" class="development-badge">
          {{ t('daily.developmentFixture') }}
        </span>
        <span v-if="day" class="day-meta">
          {{ t('daily.meta.tracked', { count: day.trackedMinutes }) }}
        </span>
      </template>
    </PageHeader>

    <main class="daily-body dg-scroll">
      <div class="daily-content">
        <DailyStatePanel
          v-if="state !== 'populated'"
          :state="state"
          @retry="daily.load(context?.day ?? '')"
        />

        <template v-else-if="context && presentation">
          <div class="daily-intro">
            <div>
              <span>{{ t('daily.overview.eyebrow') }}</span>
              <h2>{{ t('daily.overview.title') }}</h2>
            </div>
            <p>{{ t('daily.overview.description') }}</p>
          </div>

          <DailyWorkflowOverview
            :presentation="presentation"
            :time-zone="context.timeZone"
          />
          <DailyMetricsPanel :metrics="presentation.metrics" />
          <DailyRecapPanel
            :recap="recap"
            :unavailable="recapUnavailable"
            :failed="recapError !== null"
            :time-zone="context.timeZone"
          />
        </template>
      </div>
    </main>
  </div>
</template>

<style scoped>
.date-nav { display: flex; align-items: center; gap: 5px; }

.date-nav__arrow {
  display: grid;
  width: 28px;
  height: 28px;
  border-radius: 50%;
  color: var(--dg-text-secondary);
  font-size: 25px;
  line-height: 1;
  place-items: center;
}

.date-nav__arrow:disabled { color: var(--dg-text-muted); cursor: default; opacity: 0.55; }

.development-badge {
  padding: 3px 7px;
  border: 1px solid var(--dg-chip-border);
  border-radius: 5px;
  background: var(--dg-hover-fill);
  color: var(--dg-text-tertiary);
  font-size: 10px;
  font-weight: 600;
}

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
  color: var(--dg-accent-text);
  font-size: 11px;
  font-weight: 650;
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
