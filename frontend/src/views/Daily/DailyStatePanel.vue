<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import type { DailyState } from '@/stores/daily'

const props = defineProps<{ state: DailyState }>()
const emit = defineEmits<{ retry: [] }>()
const { t } = useI18n()
</script>

<template>
  <div class="daily-state dg-card" :class="`daily-state--${props.state}`" role="status" aria-live="polite">
    <div class="daily-state__mark" aria-hidden="true">
      <span /><span /><span />
    </div>
    <h2>{{ t(`daily.state.${props.state}.title`) }}</h2>
    <p>{{ t(`daily.state.${props.state}.description`) }}</p>
    <button
      v-if="props.state === 'failure'"
      type="button"
      class="dg-button"
      @click="emit('retry')"
    >
      {{ t('common.action.retry') }}
    </button>
  </div>
</template>

<style scoped>
.daily-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 220px;
  padding: 36px 28px;
  text-align: center;
}

.daily-state__mark {
  position: relative;
  width: 56px;
  height: 56px;
  margin-bottom: 16px;
  border: 1px solid var(--dg-timeline-grid-strong);
  border-radius: 14px;
  background: var(--dg-surface);
  box-shadow: var(--dg-card-shadow);
}

.daily-state__mark span {
  position: absolute;
  right: 12px;
  left: 12px;
  height: 3.5px;
  border-radius: 999px;
  background: var(--dg-timeline-placeholder);
}

.daily-state__mark span:nth-child(1) { top: 16px; }
.daily-state__mark span:nth-child(2) { top: 26px; right: 20px; }
.daily-state__mark span:nth-child(3) { top: 36px; right: 16px; }

.daily-state--failure .daily-state__mark span:nth-child(1) { background: var(--dg-danger); opacity: 0.7; }

.daily-state--loading .daily-state__mark span {
  animation: daily-shimmer 1.5s ease-in-out infinite;
}
.daily-state--loading .daily-state__mark span:nth-child(2) { animation-delay: 0.18s; }
.daily-state--loading .daily-state__mark span:nth-child(3) { animation-delay: 0.36s; }

@keyframes daily-shimmer {
  0%, 100% { opacity: 0.4; }
  50% { opacity: 1; }
}

.daily-state h2 { color: var(--dg-text-primary); font-size: 16px; font-weight: 650; }
.daily-state p { max-width: 460px; margin-top: 6px; color: var(--dg-text-secondary); font-size: 13px; line-height: 1.55; }
.daily-state .dg-button { margin-top: 16px; }

@media (prefers-reduced-motion: reduce) {
  .daily-state--loading .daily-state__mark span { animation: none; }
}
</style>
