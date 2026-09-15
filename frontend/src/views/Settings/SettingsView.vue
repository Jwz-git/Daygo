<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import LiquidGlassSurface from '@/components/LiquidGlassSurface.vue'
import PageHeader from '@/components/PageHeader.vue'

import AgentAccessSection from './AgentAccessSection.vue'
import AppearanceSection from './AppearanceSection.vue'
import PrivacySection from './PrivacySection.vue'
import ProvidersSection from './ProvidersSection.vue'
import StorageSection from './StorageSection.vue'
import { SETTINGS_SECTIONS, settingsSectionFromQuery, type SettingsSection } from './navigation'

const { t } = useI18n()

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

/*
 * The selection highlight is one absolutely positioned pill that slides to the
 * active item, instead of each item toggling its own background. The pill
 * measures the active button's offset inside the (positioned) list, which
 * covers both the vertical desktop nav and the horizontal narrow one; the
 * first measurement skips its transition so it does not slide in from 0,0.
 */
const navListEl = ref<HTMLElement | null>(null)
const itemEls = new Map<SettingsSection, HTMLElement>()
const pill = ref({ x: 0, y: 0, w: 0, h: 0 })
// Transitions arm one frame after the pill's position is first committed;
// before that every position change applies instantly, so the pill never
// slides in from (0,0) on page open — the ResizeObserver's initial callback
// fires after the first paint, which is exactly what made it animate.
const pillArmed = ref(false)
let pillArmPending = false
let navObserver: ResizeObserver | null = null

function measurePill(): void {
  const item = itemEls.get(active.value)
  if (!item || !navListEl.value) return
  pill.value = {
    x: item.offsetLeft, y: item.offsetTop,
    w: item.offsetWidth, h: item.offsetHeight,
  }
  if (!pillArmed.value && !pillArmPending) {
    pillArmPending = true
    requestAnimationFrame(() =>
      requestAnimationFrame(() => { pillArmed.value = true; pillArmPending = false }))
  }
}

function setItemRef(section: SettingsSection, el: unknown): void {
  if (el instanceof HTMLElement) itemEls.set(section, el)
  else itemEls.delete(section)
}

onMounted(() => {
  measurePill()
  // The narrow layout reflows the nav from vertical to horizontal; re-measure
  // whenever the list box changes size.
  navObserver = new ResizeObserver(() => measurePill())
  if (navListEl.value) navObserver.observe(navListEl.value)
})
onBeforeUnmount(() => { navObserver?.disconnect(); navObserver = null })
watch(active, () => void nextTick(measurePill))
</script>

<template>
  <div class="page">
    <PageHeader :title="t('settings.title')" />

    <div class="body">
      <LiquidGlassSurface intensity="glass" as="nav" class="nav" :aria-label="t('settings.title')">
        <ul ref="navListEl">
          <span
            class="nav__pill"
            aria-hidden="true"
            :class="{ 'nav__pill--armed': pillArmed }"
            :style="{
              transform: `translate(${pill.x}px, ${pill.y}px)`,
              width: `${pill.w}px`,
              height: `${pill.h}px`,
            }"
          />
          <li v-for="section in sections" :key="section">
            <button
              :ref="(el) => setItemRef(section, el)"
              type="button"
              class="nav__item"
              :class="{ 'is-active': active === section }"
              :aria-current="active === section ? 'page' : undefined"
              @click="selectSection(section)"
            >
              {{ t(`settings.nav.${section}`) }}
            </button>
          </li>
        </ul>
      </LiquidGlassSurface>

      <div class="content dg-scroll">
        <div class="content__inner">
          <Transition name="pane" mode="out-in">
            <div class="pane" :key="active">
              <template v-if="active === 'general'">
                <AppearanceSection />
              </template>
              <template v-else-if="active === 'recording'">
                <PrivacySection />
              </template>
              <template v-else-if="active === 'providers'">
                <ProvidersSection />
              </template>
              <template v-else-if="active === 'storage'">
                <StorageSection />
              </template>
              <AgentAccessSection v-else-if="active === 'agentAccess'" />
            </div>
          </Transition>
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
}

.nav ul {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.nav__pill {
  position: absolute;
  top: 0;
  left: 0;
  z-index: 0;
  border-radius: 5px;
  background: var(--dg-nav-active-fill);
  /* Same specular top edge as every raised glass surface. */
  box-shadow: var(--dg-nav-active-shadow);
  pointer-events: none;
}

.nav__pill--armed {
  transition:
    transform var(--dg-motion-base) var(--dg-ease-glide),
    width var(--dg-motion-base) var(--dg-ease-glide),
    height var(--dg-motion-base) var(--dg-ease-glide);
}

.nav__item {
  position: relative;
  z-index: 1;
  width: 100%;
  min-height: 44px;
  padding: 9px 11px;
  border-radius: 5px;
  color: var(--dg-text-secondary);
  font-size: 13px;
  font-weight: 500;
  text-align: left;
  transition:
    color var(--dg-motion-fast) ease,
    transform var(--dg-motion-fast) var(--dg-ease-out);
}

/* No hover box: the sliding pill is the only highlight in this nav, and text
   colour alone carries hover feedback. Pressing scales the item down slightly
   for a physical feel. */
.nav__item:hover {
  color: var(--dg-text-primary);
}

.nav__item:active:not(:disabled) {
  transform: scale(0.97);
}

.nav__item.is-active {
  color: var(--dg-nav-active-text);
  font-weight: 620;
}

.nav__item:focus-visible {
  outline: none;
  box-shadow: 0 0 0 3px var(--dg-focus-ring);
}

/*
 * Section switch: the outgoing pane fades, the incoming one fades in with a
 * small upward settle. mode="out-in" keeps both from sharing the layout, which
 * would otherwise stack two full panes for a frame.
 */
.pane {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.pane-enter-active,
.pane-leave-active {
  transition:
    opacity var(--dg-motion-base) ease,
    transform var(--dg-motion-base) var(--dg-ease-glide);
}

.pane-enter-from {
  opacity: 0;
  transform: translateY(4px);
}

.pane-leave-to {
  opacity: 0;
}

.content {
  min-width: 0;
}

.content__inner {
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

}

@media (prefers-reduced-motion: reduce) {
  .nav__item {
    transition: none;
  }

  .nav__pill--armed {
    transition: none;
  }

  .pane-enter-active,
  .pane-leave-active {
    transition: none;
  }
}

@media (forced-colors: active) {
  .nav {
    border-color: CanvasText;
    background: Canvas;
    box-shadow: none;
  }

  .nav__pill {
    display: none;
  }

  .nav__item {
    color: ButtonText;
  }

  .nav__item.is-active {
    border: 1px solid Highlight;
    background: Highlight;
    color: HighlightText;
  }

  .nav__item:focus-visible {
    outline: 2px solid Highlight;
    outline-offset: 2px;
    box-shadow: none;
  }
}
</style>
