<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

import PageHeader from '@/components/PageHeader.vue'
import PlannedNotice from '@/components/PlannedNotice.vue'

import AppearanceSection from './AppearanceSection.vue'
import ProvidersSection from './ProvidersSection.vue'
import RecognitionSection from './RecognitionSection.vue'

const { t } = useI18n()

/*
 * A section is either implemented or a planned placeholder. The two implemented
 * ones (providers, other) are routed explicitly in the template; every other
 * key renders a PlannedNotice. There is no account section: v1 has no account
 * system by design.
 */
const sections = [
  'providers',
  'storage',
  'privacy',
  'agentAccess',
  'dataExport',
  'other',
] as const

type SectionKey = (typeof sections)[number]

const active = ref<SectionKey>('other')
</script>

<template>
  <div class="page">
    <PageHeader :title="t('settings.title')" />

    <div class="body">
      <nav class="nav" :aria-label="t('settings.title')">
        <ul>
          <li v-for="section in sections" :key="section">
            <button
              type="button"
              class="nav__item"
              :class="{ 'is-active': active === section }"
              :aria-current="active === section ? 'true' : undefined"
              @click="active = section"
            >
              {{ t(`settings.nav.${section}`) }}
            </button>
          </li>
        </ul>
      </nav>

      <div class="content dg-scroll">
        <div class="content__inner dg-stagger">
          <ProvidersSection v-if="active === 'providers'" />

          <template v-else-if="active === 'other'">
            <AppearanceSection />
            <RecognitionSection />
            <PlannedNotice
              title-key="settings.nav.other"
              description-key="settings.section.otherDescription"
            />
          </template>

          <PlannedNotice
            v-else
            :title-key="`settings.nav.${active}`"
            :description-key="`settings.section.${active}Description`"
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
  gap: 24px;
  flex: 1;
  min-height: 0;
  padding: 0 var(--dg-page-padding) var(--dg-page-padding);
}

.nav {
  align-self: start;
  padding: 7px;
  border: 1px solid var(--dg-card-border);
  border-radius: 7px;
  background: color-mix(in srgb, var(--dg-card-fill) 72%, transparent);
  box-shadow: var(--dg-card-shadow);
}

.nav ul {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.nav__item {
  width: 100%;
  padding: 9px 11px;
  border-radius: 5px;
  color: var(--dg-text-secondary);
  font-size: 13px;
  font-weight: 500;
  text-align: left;
  transition:
    color var(--dg-motion-fast) ease,
    background var(--dg-motion-fast) ease,
    transform var(--dg-motion-fast) var(--dg-ease-out);
}

.nav__item:hover {
  background: var(--dg-hover-fill);
  color: var(--dg-text-primary);
  transform: translateX(2px);
}

.nav__item.is-active {
  background: var(--dg-nav-active-fill);
  color: var(--dg-nav-active-text);
  font-weight: 620;
}

.nav__item:focus-visible {
  outline: none;
  box-shadow: 0 0 0 3px var(--dg-focus-ring);
}

.content {
  min-width: 0;
}

.content__inner {
  display: flex;
  flex-direction: column;
  gap: 14px;
  max-width: var(--dg-settings-content-max);
  padding: 2px 4px 12px 0;
}

@media (max-width: 860px) {
  .body {
    grid-template-columns: minmax(0, 1fr);
    gap: 14px;
  }

  .nav {
    padding: 6px;
    overflow-x: auto;
  }

  .nav ul {
    flex-direction: row;
    flex-wrap: nowrap;
  }

  .nav__item {
    width: auto;
    white-space: nowrap;
  }

  .nav__item:hover {
    transform: translateY(-1px);
  }
}

@media (prefers-reduced-motion: reduce) {
  .nav__item {
    transition: none;
  }
}
</style>
