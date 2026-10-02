<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import DgIcon from '@/components/DgIcon.vue'

/*
 * Timeline status card, after Dayflow's `timelineStatusCard` /
 * `WeekRecordingStatusCard`: the one shape behind "generating", "recording
 * now" and "paused". Active states lay the blue-to-peach generating gradient
 * over the solid panel colour with white text and the 3x3 thinking spinner;
 * paused uses the muted paused gradient, a half-pixel border and a pause
 * glyph. `label` overrides the default copy (the track uses it for batches
 * being analysed).
 */
const props = defineProps<{ state?: 'off' | 'capturing' | 'paused'; label?: string }>()

const { t } = useI18n()
const paused = computed(() => props.state === 'paused')
const text = computed(() => props.label ?? (paused.value ? t('timeline.pausedHere') : t('timeline.recordingNow')))

// 3x3 cells; the wave runs along the diagonal, as in Dayflow's spinner.
const CELLS = Array.from({ length: 9 }, (_, index) => ({ index, delay: ((index % 3) + Math.floor(index / 3)) * 0.12 }))
</script>

<template>
  <div class="gen-card" :class="{ 'is-paused': paused }" role="status">
    <DgIcon v-if="paused" class="gen-card__hold" name="pause" :size="11" />
    <span v-else class="gen-card__spinner" aria-hidden="true">
      <i v-for="cell in CELLS" :key="cell.index" :style="{ animationDelay: `${cell.delay}s` }"></i>
    </span>
    <span class="gen-card__text">{{ text }}</span>
  </div>
</template>

<style scoped>
.gen-card {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 26px;
  padding: 0 12px;
  overflow: hidden;
  border-radius: 3px;
  background: var(--dg-status-generating), var(--dg-surface);
  color: var(--dg-status-generating-text);
  font-size: 12px;
  font-weight: 600;
  line-height: 1.2;
  white-space: nowrap;
}

.gen-card.is-paused {
  gap: 10px;
  /* Dayflow strokes only the paused card: cardBorder at 60%, 0.5pt. */
  box-shadow: inset 0 0 0 0.5px var(--dg-status-paused-border);
  background: var(--dg-status-paused), var(--dg-surface);
  color: var(--dg-status-paused-text);
  font-weight: 400;
}

.gen-card__text {
  overflow: hidden;
  text-overflow: ellipsis;
}

.gen-card__hold {
  flex: none;
}

/* Dayflow's TimelineThinkingSpinner at timeline scale: 3x3 rounded cells, a
   band sweeping the diagonal from dim blue through violet to hot amber, with
   a soft bloom on the brightest cells. */
.gen-card__spinner {
  flex: none;
  display: grid;
  grid-template-columns: repeat(3, 4px);
  gap: 0.5px;
}

.gen-card__spinner i {
  width: 4px;
  height: 4px;
  border-radius: 1px;
  background: rgba(67, 93, 151, 0.18);
  animation: gen-wave 1.5s ease-in-out infinite;
}

@keyframes gen-wave {
  0%, 70%, 100% { background: rgba(67, 93, 151, 0.18); box-shadow: none; }
  18% { background: #b884bc; }
  32% { background: #f6be74; box-shadow: 0 0 3px rgba(246, 190, 116, 0.9); }
  48% { background: #435d97; box-shadow: none; }
}

@media (prefers-reduced-motion: reduce) {
  .gen-card__spinner i { animation: none; background: #b884bc; }
}
</style>
