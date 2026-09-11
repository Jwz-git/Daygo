<script setup lang="ts">
import PageHeader from '@/components/PageHeader.vue'
import PlannedNotice from '@/components/PlannedNotice.vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

/*
 * Six cards. The chart sub-models of WeeklyDashboardDTO are still an open item
 * (docs/05-interface-contract.md §5.11), so this is layout only — no data shape
 * is committed to here.
 */
const cards = [
  { key: 'donut' },
  { key: 'context' },
  { key: 'workflow' },
  { key: 'heatmap' },
  { key: 'treemap' },
  { key: 'sankey' },
] as const
</script>

<template>
  <div class="page">
    <PageHeader :title="t('weekly.title')">
      <template #lead>
        <div class="nav-group" role="group" :aria-label="t('weekly.title')">
          <button type="button" class="dg-chip" disabled>
            {{ t('common.action.previous') }}
          </button>
          <button type="button" class="dg-chip" disabled>
            {{ t('common.action.next') }}
          </button>
        </div>
      </template>
    </PageHeader>

    <div class="body dg-scroll">
      <div class="grid">
        <PlannedNotice
          v-for="card in cards"
          :key="card.key"
          :title-key="`weekly.card.${card.key}`"
          :description-key="`weekly.card.${card.key}Description`"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.nav-group {
  display: flex;
  align-items: center;
  gap: 8px;
}

.body {
  flex: 1;
  min-height: 0;
  padding: 2px var(--dg-page-padding) var(--dg-page-padding);
}

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(320px, 100%), 1fr));
  gap: 18px;
}
</style>
