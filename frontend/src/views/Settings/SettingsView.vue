<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import PageHeader from '@/components/PageHeader.vue'
import MilestoneNotice from '@/components/MilestoneNotice.vue'

import AppearanceSection from './AppearanceSection.vue'
import ProvidersSection from './ProvidersSection.vue'

const { t } = useI18n()

/*
 * `milestone: null` marks a section that is implemented; the rest render a
 * placeholder naming the milestone that fills them in (docs/09-roadmap.md).
 * There is no account section: v1 has no account system by design.
 */
const sections = [
  { key: 'providers', milestone: null },
  { key: 'storage', milestone: 4 },
  { key: 'privacy', milestone: 4 },
  { key: 'agentAccess', milestone: 5 },
  { key: 'dataExport', milestone: 4 },
  { key: 'other', milestone: null },
] as const satisfies readonly { key: string; milestone: 4 | 5 | null }[]

type SectionKey = (typeof sections)[number]['key']

const active = ref<SectionKey>('other')

const activeMilestone = computed<4 | 5>(
  () => sections.find((section) => section.key === active.value)?.milestone ?? 4,
)
</script>

<template>
  <div class="page">
    <PageHeader :title="t('settings.title')" />

    <div class="body">
      <nav class="nav" :aria-label="t('settings.title')">
        <ul>
          <li v-for="section in sections" :key="section.key">
            <button
              type="button"
              class="nav__item"
              :class="{ 'is-active': active === section.key }"
              :aria-current="active === section.key ? 'true' : undefined"
              @click="active = section.key"
            >
              {{ t(`settings.nav.${section.key}`) }}
            </button>
          </li>
        </ul>
      </nav>

      <div class="content dg-scroll">
        <div class="content__inner">
          <ProvidersSection v-if="active === 'providers'" />

          <template v-else-if="active === 'other'">
            <AppearanceSection />
            <MilestoneNotice
              title-key="settings.nav.other"
              description-key="settings.section.otherDescription"
              :milestone="2"
            />
          </template>

          <MilestoneNotice
            v-else
            :title-key="`settings.nav.${active}`"
            :description-key="`settings.section.${active}Description`"
            :milestone="activeMilestone"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.body {
  display: grid;
  grid-template-columns: var(--dg-settings-nav-width) minmax(0, 1fr);
  flex: 1;
  min-height: 0;
  padding: 0 var(--dg-page-padding) var(--dg-page-padding);
}

.nav ul {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.nav__item {
  width: 100%;
  padding: 7px 10px;
  border-radius: 6px;
  color: var(--dg-text-secondary);
  font-size: 13px;
  text-align: left;
}

.nav__item:hover {
  background: var(--dg-hover-fill);
}

.nav__item.is-active {
  background: var(--dg-control-fill);
  color: var(--dg-text-primary);
}

.content {
  min-width: 0;
}

.content__inner {
  display: flex;
  flex-direction: column;
  gap: 16px;
  max-width: var(--dg-settings-content-max);
  padding-bottom: 8px;
}

@media (max-width: 860px) {
  .body {
    grid-template-columns: minmax(0, 1fr);
    gap: 14px;
  }

  .nav ul {
    flex-direction: row;
    flex-wrap: wrap;
  }

  .nav__item {
    width: auto;
  }
}
</style>
