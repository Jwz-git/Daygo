<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { TimelineState } from '@/stores/timeline'

const props = defineProps<{ state: TimelineState }>()
const emit = defineEmits<{ retry: [] }>()
const { t } = useI18n()

const canRetry = computed(() => props.state === 'failure')
</script>

<template>
  <section class="state-panel" :class="`state-panel--${props.state}`" aria-live="polite">
    <div class="state-panel__mark" aria-hidden="true">
      <span></span><span></span><span></span>
    </div>
    <p class="state-panel__eyebrow">{{ t(`timeline.state.${props.state}.eyebrow`) }}</p>
    <h2 class="state-panel__title dg-display">
      {{ t(`timeline.state.${props.state}.title`) }}
    </h2>
    <p class="state-panel__description">
      {{ t(`timeline.state.${props.state}.description`) }}
    </p>
    <button v-if="canRetry" type="button" class="dg-button" @click="emit('retry')">
      {{ t('common.action.retry') }}
    </button>
  </section>
</template>

<style scoped>
.state-panel {
  display: flex;
  flex: 1;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 420px;
  padding: 48px;
  border: 1px solid var(--dg-timeline-grid-strong);
  border-radius: var(--dg-card-radius);
  background: var(--dg-timeline-empty-fill);
  text-align: center;
}

.state-panel__mark {
  position: relative;
  width: 76px;
  height: 76px;
  margin-bottom: 24px;
  border: 1px solid var(--dg-timeline-grid-strong);
  border-radius: 18px;
  background: var(--dg-card-fill);
  box-shadow: var(--dg-card-shadow);
}

.state-panel__mark span {
  position: absolute;
  right: 16px;
  left: 16px;
  height: 4px;
  border-radius: 999px;
  background: var(--dg-timeline-placeholder);
}

.state-panel__mark span:nth-child(1) { top: 22px; }
.state-panel__mark span:nth-child(2) { top: 35px; right: 26px; }
.state-panel__mark span:nth-child(3) { top: 48px; right: 21px; }

.state-panel__eyebrow {
  margin-bottom: 7px;
  color: var(--dg-accent-text);
  font-size: 11px;
  font-weight: 650;
  letter-spacing: 0;
}

.state-panel__title {
  color: var(--dg-text-primary);
  font-size: 27px;
  line-height: 1.12;
}

.state-panel__description {
  max-width: 430px;
  margin: 10px 0 20px;
  color: var(--dg-text-secondary);
  font-size: 13px;
}

</style>
