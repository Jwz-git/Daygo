<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import type { DailyRecapDTO } from '@/api/dto'

const props = defineProps<{
  recap: DailyRecapDTO | null
  unavailable: boolean
  failed: boolean
  timeZone: string
}>()

const { locale, t } = useI18n()
const copyState = ref<'idle' | 'copied' | 'failed'>('idle')
let resetTimer: number | undefined

const generatedAt = computed(() => {
  if (props.recap?.generatedAtTs == null) return null
  return new Intl.DateTimeFormat(locale.value, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    timeZone: props.timeZone,
  }).format(new Date(props.recap.generatedAtTs * 1000))
})

const copyLabel = computed(() => {
  if (copyState.value === 'copied') return t('daily.standup.copied')
  if (copyState.value === 'failed') return t('daily.standup.copyFailed')
  return t('daily.standup.copy')
})

function recapText(recap: DailyRecapDTO): string {
  const bullets = (items: string[]) => items.map((item) => `- ${item}`).join('\n')
  return [
    recap.highlightsTitle,
    bullets(recap.highlights),
    recap.tasksTitle,
    bullets(recap.tasks),
    recap.blockersTitle,
    recap.blockersBody,
  ]
    .filter(Boolean)
    .join('\n\n')
}

async function copyRecap(): Promise<void> {
  if (props.recap === null) return
  window.clearTimeout(resetTimer)
  try {
    await navigator.clipboard.writeText(recapText(props.recap))
    copyState.value = 'copied'
  } catch {
    copyState.value = 'failed'
  }
  resetTimer = window.setTimeout(() => {
    copyState.value = 'idle'
  }, 1800)
}

onBeforeUnmount(() => window.clearTimeout(resetTimer))
</script>

<template>
  <section class="daily-section" aria-labelledby="daily-recap-title">
    <header class="section-heading">
      <div>
        <h2 id="daily-recap-title">{{ t('daily.standup.title') }}</h2>
        <p>{{ t('daily.standup.description') }}</p>
      </div>
      <div class="recap-actions">
        <button
          type="button"
          class="dg-button"
          :title="t('daily.standup.generateUnavailable')"
          disabled
        >
          {{ t('common.action.regenerate') }}
        </button>
        <button
          type="button"
          class="dg-button dg-button--primary"
          :disabled="recap === null"
          @click="copyRecap"
        >
          {{ copyLabel }}
        </button>
      </div>
    </header>

    <div v-if="unavailable || failed || recap === null" class="recap-state dg-card">
      <strong>{{ failed ? t('daily.standup.failureTitle') : t('daily.standup.unavailableTitle') }}</strong>
      <span>{{ failed ? t('daily.standup.failureDescription') : t('daily.standup.unavailableDescription') }}</span>
    </div>

    <div v-else class="recap-card dg-card">
      <article class="recap-column">
        <span class="recap-index" aria-hidden="true">01</span>
        <h3>{{ recap.highlightsTitle || t('daily.standup.highlights') }}</h3>
        <ul>
          <li v-for="item in recap.highlights" :key="item">{{ item }}</li>
        </ul>
      </article>

      <article class="recap-column">
        <span class="recap-index" aria-hidden="true">02</span>
        <h3>{{ recap.tasksTitle || t('daily.standup.tasks') }}</h3>
        <ul>
          <li v-for="item in recap.tasks" :key="item">{{ item }}</li>
        </ul>
      </article>

      <article class="recap-blockers">
        <span class="recap-index" aria-hidden="true">03</span>
        <div>
          <h3>{{ recap.blockersTitle || t('daily.standup.blockers') }}</h3>
          <p>{{ recap.blockersBody || t('daily.standup.noBlockers') }}</p>
        </div>
        <span v-if="generatedAt" class="generated-at">
          {{ t('daily.standup.generatedAt', { date: generatedAt }) }}
        </span>
      </article>
    </div>
  </section>
</template>

<style scoped>
.daily-section { display: flex; flex-direction: column; gap: 10px; }

.section-heading {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 24px;
}

.section-heading h2 { color: var(--dg-text-primary); font-size: 18px; font-weight: 650; line-height: 1.25; }
.section-heading p { margin-top: 3px; color: var(--dg-text-tertiary); font-size: 12px; }
.recap-actions { display: flex; flex: none; gap: 7px; }

.recap-card {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  overflow: hidden;
}

.recap-column {
  position: relative;
  min-height: 196px;
  padding: 21px 24px 24px;
}

.recap-column + .recap-column { border-left: 1px solid var(--dg-card-border); }

.recap-index {
  display: block;
  margin-bottom: 16px;
  color: var(--dg-text-muted);
  font-size: 10px;
  font-variant-numeric: tabular-nums;
}

.recap-card h3 { color: var(--dg-text-primary); font-size: 14px; font-weight: 650; }

.recap-card ul {
  display: flex;
  flex-direction: column;
  gap: 9px;
  margin-top: 13px;
  padding: 0;
  list-style: none;
}

.recap-card li {
  position: relative;
  padding-left: 15px;
  color: var(--dg-text-secondary);
  font-size: 13px;
  line-height: 1.45;
}

.recap-card li::before {
  position: absolute;
  top: 0.57em;
  left: 1px;
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: var(--dg-accent);
  content: '';
}

.recap-blockers {
  display: grid;
  grid-column: 1 / -1;
  grid-template-columns: 34px minmax(0, 1fr) auto;
  align-items: start;
  gap: 10px;
  padding: 17px 24px 19px;
  border-top: 1px solid var(--dg-card-border);
  background: var(--dg-daily-footer-fill);
}

.recap-blockers .recap-index { margin: 3px 0 0; }
.recap-blockers p { margin-top: 3px; color: var(--dg-text-secondary); font-size: 12px; }
.generated-at { align-self: center; color: var(--dg-text-muted); font-size: 10px; white-space: nowrap; }

.recap-state {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-height: 110px;
  padding: 22px;
  justify-content: center;
}

.recap-state strong { color: var(--dg-text-primary); font-size: 13px; }
.recap-state span { color: var(--dg-text-tertiary); font-size: 12px; }

@media (max-width: 720px) {
  .section-heading { align-items: flex-start; flex-direction: column; gap: 10px; }
  .recap-actions { align-self: stretch; }
  .recap-actions .dg-button { flex: 1; }
  .recap-card { grid-template-columns: minmax(0, 1fr); }
  .recap-column + .recap-column { border-top: 1px solid var(--dg-card-border); border-left: 0; }
  .recap-blockers { grid-template-columns: 28px minmax(0, 1fr); }
  .generated-at { display: none; }
}
</style>
