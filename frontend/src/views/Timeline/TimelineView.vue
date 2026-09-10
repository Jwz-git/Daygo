<script setup lang="ts">
import PageHeader from '@/components/PageHeader.vue'
import MilestoneNotice from '@/components/MilestoneNotice.vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

/*
 * The header controls below are inert on purpose. Date navigation cannot be
 * built yet: the logical day (4 AM boundary) may only come from the backend via
 * GetDayContext("") — see docs/05-interface-contract.md §5.3.2 — so this
 * shell deliberately contains no date arithmetic.
 */
</script>

<template>
  <div class="page">
    <PageHeader :title="t('timeline.title')">
      <template #lead>
        <div class="nav-group" role="group" :aria-label="t('timeline.title')">
          <button type="button" class="dg-chip" disabled>
            {{ t('common.action.previous') }}
          </button>
          <button type="button" class="dg-chip" disabled>
            {{ t('common.action.next') }}
          </button>
          <button type="button" class="dg-chip dg-chip--filled" disabled>
            {{ t('common.action.today') }}
          </button>
        </div>
      </template>

      <template #trail>
        <div class="segment" role="group" :aria-label="t('timeline.title')">
          <span class="segment__option is-selected">{{ t('timeline.mode.day') }}</span>
          <span class="segment__option">{{ t('timeline.mode.week') }}</span>
        </div>
      </template>
    </PageHeader>

    <div class="filter-bar">
      <span class="dg-chip dg-chip--filled">{{ t('timeline.filter.all') }}</span>
      <span class="dg-chip">{{ t('timeline.filter.manage') }}</span>
    </div>

    <div class="body dg-scroll dg-stagger">
      <section class="track">
        <MilestoneNotice
          title-key="timeline.track.title"
          description-key="timeline.track.description"
          :milestone="3"
        />
      </section>

      <aside class="inspector">
        <MilestoneNotice
          title-key="timeline.inspector.title"
          description-key="timeline.inspector.description"
          :milestone="3"
        />
      </aside>
    </div>

    <footer class="footer">
      <span class="footer__meta">{{ t('timeline.footer.recorded') }}</span>
      <div class="footer__actions">
        <button type="button" class="dg-chip" disabled>
          {{ t('timeline.footer.copy') }}
        </button>
        <button type="button" class="dg-chip" disabled>
          {{ t('timeline.footer.review') }}
        </button>
      </div>
    </footer>
  </div>
</template>

<style scoped>
.nav-group,
.footer__actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.segment {
  display: flex;
  padding: 3px;
  border: 1px solid var(--dg-chip-border);
  border-radius: 999px;
  background: var(--dg-track-fill);
  box-shadow: inset 0 1px 2px rgba(20, 16, 25, 0.04);
}

.segment__option {
  padding: 5px 15px;
  border-radius: 999px;
  color: var(--dg-text-secondary);
  font-size: 12px;
}

.segment__option.is-selected {
  background: var(--dg-chip-fill);
  box-shadow: 0 3px 10px rgba(20, 16, 25, 0.1);
  color: var(--dg-text-primary);
}

.filter-bar {
  display: flex;
  gap: 8px;
  padding: 0 var(--dg-page-padding) 16px;
}

.body {
  display: grid;
  grid-template-columns: minmax(0, 1fr) var(--dg-inspector-width);
  gap: 16px;
  flex: 1;
  min-height: 0;
  padding: 0 var(--dg-page-padding);
}

.track,
.inspector {
  min-width: 0;
}

.footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-top: 18px;
  padding: 15px var(--dg-page-padding) 17px;
  border-top: 1px solid var(--dg-card-border);
  background: linear-gradient(180deg, transparent, var(--dg-hover-fill));
}

.footer__meta {
  color: var(--dg-text-muted);
  font-size: 12px;
}

@media (max-width: 1000px) {
  .body {
    grid-template-columns: minmax(0, 1fr);
  }

  .inspector {
    display: none;
  }
}
</style>
