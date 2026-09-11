<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import type { DailyState } from '@/stores/daily'

const props = defineProps<{ state: DailyState }>()
const emit = defineEmits<{ retry: [] }>()
const { t } = useI18n()
</script>

<template>
  <div class="daily-state dg-card" role="status">
    <span class="daily-state__mark" aria-hidden="true"></span>
    <div>
      <h2>{{ t(`daily.state.${props.state}.title`) }}</h2>
      <p>{{ t(`daily.state.${props.state}.description`) }}</p>
    </div>
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
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 14px;
  min-height: 180px;
  padding: 28px;
}

.daily-state__mark {
  width: 9px;
  height: 34px;
  border-radius: 3px;
  background: var(--dg-accent);
  opacity: 0.74;
}

.daily-state h2 { color: var(--dg-text-primary); font-size: 16px; font-weight: 650; }
.daily-state p { max-width: 580px; margin-top: 4px; color: var(--dg-text-tertiary); font-size: 13px; }
</style>
