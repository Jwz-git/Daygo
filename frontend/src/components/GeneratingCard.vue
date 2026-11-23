<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import DgIcon from '@/components/DgIcon.vue'
import { useUIVisibilityStore } from '@/stores/uiVisibility'

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
const visibility = useUIVisibilityStore()
const paused = computed(() => props.state === 'paused')
const text = computed(() => props.label ?? (paused.value ? t('timeline.pausedHere') : t('timeline.recordingNow')))

// 3x3 cells; the wave runs along the diagonal, as in Dayflow's spinner.
const CELLS = Array.from({ length: 9 }, (_, index) => ({ index, delay: ((index % 3) + Math.floor(index / 3)) * 0.12 }))
</script>

<template>
  <div class="gen-card" :class="{ 'is-paused': paused, 'is-window-hidden': !visibility.visible }" role="status">
    <DgIcon v-if="paused" class="gen-card__hold" name="pause" :size="11" />
    <span v-else class="gen-card__spinner" aria-hidden="true">
      <i v-for="cell in CELLS" :key="cell.index" :style="{ animationDelay: `${cell.delay}s` }"></i>
    </span>
    <span class="gen-card__text">{{ text }}</span>
  </div>
</template>

<style scoped>
.gen-card {
  position: relative;
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 28px;
  padding: 0 13px;
  overflow: hidden;
  border-radius: 9px;
  background: var(--dg-status-generating), var(--dg-surface);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.34),
    0 2px 10px rgba(50, 118, 224, 0.14);
  color: var(--dg-status-generating-text);
  font-size: 12px;
  font-weight: 600;
  line-height: 1.2;
  white-space: nowrap;
  transition: box-shadow var(--dg-motion-base) ease-out, filter var(--dg-motion-fast) ease-out;
}

.gen-card:not(.is-paused)::after {
  content: '';
  position: absolute;
  inset: 0;
  pointer-events: none;
  background: linear-gradient(
    110deg,
    transparent 24%,
    rgba(255, 255, 255, 0.18) 50%,
    transparent 76%
  );
  transform: translateX(-100%);
  animation: gen-sheen 2.8s ease-in-out infinite;
}

.gen-card.is-window-hidden:not(.is-paused)::after {
  animation-play-state: paused;
}

@keyframes gen-sheen {
  0% { transform: translateX(-100%); }
  55%, 100% { transform: translateX(100%); }
}

.gen-card.is-paused {
  gap: 9px;
  /* Dayflow strokes only the paused card, lifted with a soft top specular edge. */
  box-shadow:
    inset 0 0 0 1px var(--dg-status-paused-border),
    inset 0 1px 0 rgba(255, 255, 255, 0.42);
  background: var(--dg-status-paused), var(--dg-surface);
  color: var(--dg-status-paused-text);
  font-weight: 500;
}

.gen-card.is-paused:hover {
  filter: brightness(1.03);
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

.is-window-hidden .gen-card__spinner i {
  animation-play-state: paused;
}

@keyframes gen-wave {
  0%, 70%, 100% { background: rgba(67, 93, 151, 0.18); box-shadow: none; }
  18% { background: #b884bc; }
  32% { background: #f6be74; box-shadow: 0 0 3px rgba(246, 190, 116, 0.9); }
  48% { background: #435d97; box-shadow: none; }
}

@media (prefers-reduced-motion: reduce) {
  .gen-card { transition: none; }
  .gen-card:not(.is-paused)::after { display: none; }
  .gen-card__spinner i { animation: none; background: #b884bc; }
}
</style>
