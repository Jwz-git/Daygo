<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import PageHeader from '@/components/PageHeader.vue'

import AgentAccessSection from './AgentAccessSection.vue'
import AppearanceSection from './AppearanceSection.vue'
import PrivacySection from './PrivacySection.vue'
import ProvidersSection from './ProvidersSection.vue'
import RecognitionSection from './RecognitionSection.vue'
import StorageSection from './StorageSection.vue'
import OutputLanguageSection from './OutputLanguageSection.vue'
import { SETTINGS_SECTIONS, settingsSectionFromQuery, type SettingsSection } from './navigation'

const { t } = useI18n()
const isDevelopment = import.meta.env.DEV

/*
 * A section is either implemented or a planned placeholder. The implemented
 * ones are routed explicitly in the template; every other key renders a
 * PlannedNotice. There is no account section: v1 has no account system by
 * design.
 *
 * storage / privacy / agentAccess consume the real GetSettings /
 * UpdateSettings bindings. The values persist, but each setting's downstream
 * consumer (recorder, cleanup loop, agent.sock) is still unimplemented — the
 * agentAccess hint says so explicitly.
 */
const sections = SETTINGS_SECTIONS

const route = useRoute()
const router = useRouter()

const active = ref<SettingsSection>(settingsSectionFromQuery(route.query.section))
watch(() => route.query.section, (value) => { active.value = settingsSectionFromQuery(value) })

function selectSection(section: SettingsSection): void {
  active.value = section
  void router.replace({ query: { ...route.query, section } })
}
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
              @click="selectSection(section)"
            >
              {{ t(`settings.nav.${section}`) }}
            </button>
          </li>
        </ul>
      </nav>

      <div class="content dg-scroll">
        <div class="content__inner">
          <template v-if="active === 'general'">
            <AppearanceSection />
          </template>
          <template v-else-if="active === 'recording'">
            <PrivacySection />
          </template>
          <template v-else-if="active === 'ai'">
            <ProvidersSection />
            <OutputLanguageSection />
            <RecognitionSection />
          </template>
          <template v-else-if="active === 'storage'">
            <StorageSection />
            <RouterLink v-if="isDevelopment" class="diagnostics-link dg-button" :to="{ name: 'capture-test' }">
              {{ t('settings.diagnostics.captureTest') }}
            </RouterLink>
          </template>
          <AgentAccessSection v-else-if="active === 'agentAccess'" />
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
    background var(--dg-motion-fast) ease;
}

.nav__item:hover {
  background: var(--dg-hover-fill);
  color: var(--dg-text-primary);
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

.diagnostics-link {
  align-self: flex-start;
  color: var(--dg-button-secondary-text);
  text-decoration: none;
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

}

@media (prefers-reduced-motion: reduce) {
  .nav__item {
    transition: none;
  }
}
</style>
