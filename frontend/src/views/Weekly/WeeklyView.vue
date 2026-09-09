<script setup lang="ts">
import PageHeader from '@/components/PageHeader.vue'
import MilestoneNotice from '@/components/MilestoneNotice.vue'
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
          <button type="button" class="chip" disabled>
            {{ t('common.action.previous') }}
          </button>
          <button type="button" class="chip" disabled>
            {{ t('common.action.next') }}
          </button>
        </div>
      </template>
    </PageHeader>

    <div class="body dg-scroll">
      <div class="grid">
        <MilestoneNotice
          v-for="card in cards"
          :key="card.key"
          :title-key="`weekly.card.${card.key}`"
          :description-key="`weekly.card.${card.key}Description`"
          :milestone="4"
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

.chip {
  padding: 5px 12px;
  border: 1px solid var(--dg-card-border);
  border-radius: 999px;
  color: var(--dg-text-secondary);
  font-size: 12px;
  background: transparent;
}

.chip:disabled {
  opacity: 0.55;
  cursor: default;
}

.body {
  flex: 1;
  min-height: 0;
  padding: 0 var(--dg-page-padding) var(--dg-page-padding);
}

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 16px;
}
</style>
