<script setup lang="ts">
import PageHeader from '@/components/PageHeader.vue'
import MilestoneNotice from '@/components/MilestoneNotice.vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

/*
 * GetDailyRecap takes the CALENDAR day (midnight boundary), not the logical
 * 4 AM day — the one documented exception in §5.3.2. Both still come from the
 * backend; nothing here computes a date.
 */
</script>

<template>
  <div class="page">
    <PageHeader :title="t('daily.title')">
      <template #lead>
        <div class="nav-group" role="group" :aria-label="t('daily.title')">
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
      <div class="content">
        <MilestoneNotice
          title-key="daily.workflow.title"
          description-key="daily.workflow.description"
          :milestone="4"
        />
        <MilestoneNotice
          title-key="daily.stats.title"
          description-key="daily.stats.description"
          :milestone="4"
        />
        <MilestoneNotice
          title-key="daily.standup.title"
          description-key="daily.standup.description"
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

.content {
  display: flex;
  flex-direction: column;
  gap: 16px;
  width: 100%;
  max-width: var(--dg-daily-content-max);
  margin: 0 auto;
}
</style>
