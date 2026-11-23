<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { WeeklyState } from '@/stores/weekly'

const props = defineProps<{ state: Exclude<WeeklyState, 'populated'> }>()
const emit = defineEmits<{ retry: [] }>()
const { t } = useI18n()

const title = computed(() => t(`weekly.state.${props.state}.title`))
const description = computed(() => t(`weekly.state.${props.state}.description`))
</script>

<template>
  <section
    :class="['weekly-state', 'dg-card', `weekly-state--${state}`]"
    role="status"
    aria-live="polite"
  >
    <div class="weekly-state__mark" aria-hidden="true">
      <span /><span /><span />
    </div>
    <h2>{{ title }}</h2>
    <p>{{ description }}</p>
    <button
      v-if="state === 'failure'"
      type="button"
      class="dg-button"
      @click="emit('retry')"
    >
      {{ t('common.action.retry') }}
    </button>
  </section>
</template>

<style scoped>
.weekly-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 220px;
  padding: 36px 28px;
  text-align: center;
}

.weekly-state__mark {
  position: relative;
  width: 56px;
  height: 56px;
  margin-bottom: 16px;
  border: 1px solid var(--dg-timeline-grid-strong);
  border-radius: 14px;
  background: var(--dg-surface);
  box-shadow: var(--dg-card-shadow);
}

.weekly-state__mark span {
  position: absolute;
  right: 12px;
  left: 12px;
  height: 3.5px;
  border-radius: 999px;
  background: var(--dg-timeline-placeholder);
}

.weekly-state__mark span:nth-child(1) { top: 16px; }
.weekly-state__mark span:nth-child(2) { top: 26px; right: 20px; }
.weekly-state__mark span:nth-child(3) { top: 36px; right: 16px; }

.weekly-state--failure .weekly-state__mark span:nth-child(1) {
  background: var(--dg-danger);
  opacity: 0.7;
}

.weekly-state--loading .weekly-state__mark span {
  animation: weekly-shimmer 1.5s ease-in-out infinite;
}
.weekly-state--loading .weekly-state__mark span:nth-child(2) { animation-delay: 0.18s; }
.weekly-state--loading .weekly-state__mark span:nth-child(3) { animation-delay: 0.36s; }

@keyframes weekly-shimmer {
  0%, 100% { opacity: 0.4; }
  50% { opacity: 1; }
}

.weekly-state h2 {
  color: var(--dg-text-primary);
  font-size: 16px;
  font-weight: 650;
}

.weekly-state p {
  max-width: 460px;
  margin-top: 6px;
  color: var(--dg-text-secondary);
  font-size: 13px;
  line-height: 1.55;
}

.weekly-state .dg-button { margin-top: 16px; }

@media (prefers-reduced-motion: reduce) {
  .weekly-state--loading .weekly-state__mark span { animation: none; }
}
</style>
