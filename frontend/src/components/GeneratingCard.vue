<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

/*
 * Placeholder card at the current time. Capturing shows the nine-cell wave
 * and the recording label; paused shows a hold icon and asks for resume —
 * Dayflow-style. Geometry matches the activity cards: square corners, hairline
 * border, card fill.
 */
const props = defineProps<{ state?: 'off' | 'capturing' | 'paused' }>()

const { t } = useI18n()
const CELLS = Array.from({ length: 9 }, (_, index) => index)
const paused = computed(() => props.state === 'paused')
</script>

<template>
  <div class="gen-card" :class="{ 'is-paused': paused }" role="status">
    <span v-if="paused" class="gen-card__hold" aria-hidden="true">
      <svg viewBox="0 0 12 12"><rect x="2.8" y="2.2" width="2.4" height="7.6" rx="1" fill="currentColor" /><rect x="6.8" y="2.2" width="2.4" height="7.6" rx="1" fill="currentColor" /></svg>
    </span>
    <span v-else class="gen-card__icon" aria-hidden="true">
      <i
        v-for="cell in CELLS"
        :key="cell"
        :style="{ animationDelay: `${cell * 0.11}s` }"
      ></i>
    </span>
    <span>{{ paused ? t('timeline.pausedHere') : t('timeline.recordingNow') }}</span>
  </div>
</template>

<style scoped>
.gen-card {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 10px;
  overflow: hidden;
  border: 1px solid var(--dg-timeline-card-border);
  border-radius: 4px;
  /* Same square-ish shape as the week cards. */
  background: var(--dg-timeline-card-fill);
  color: var(--dg-text-secondary);
  font-size: 11px;
  font-weight: 550;
  white-space: nowrap;
}

.gen-card > span:last-child {
  overflow: hidden;
  text-overflow: ellipsis;
}

.gen-card:not(.is-paused) {
  background: linear-gradient(
    100deg,
    color-mix(in srgb, var(--dg-accent) 30%, var(--dg-timeline-card-fill)),
    color-mix(in srgb, #e8804a 24%, var(--dg-timeline-card-fill))
  );
  color: var(--dg-text-primary);
}

.gen-card__icon {
  flex: none;
  display: grid;
  grid-template-columns: repeat(3, 4px);
  gap: 2px;
  padding: 3px;
  border-radius: 3px;
  background: rgba(255, 255, 255, 0.35);
}

.gen-card__icon i {
  width: 4px;
  height: 4px;
  border-radius: 1.5px;
  background: var(--dg-accent);
  opacity: 0.25;
  animation: gen-wave 1.7s infinite;
}

@keyframes gen-wave {
  0%, 55%, 100% { opacity: 0.25; }
  18% { opacity: 1; }
}

.gen-card__hold {
  flex: none;
  display: grid;
  width: 18px;
  height: 18px;
  place-items: center;
  border-radius: 4px;
  background: color-mix(in srgb, #e8804a 22%, transparent);
  color: #b25a22;
}

:root[data-dg-appearance='dark'] .gen-card__hold { color: #eda06c; }

.gen-card__hold svg { width: 10px; height: 10px; }

@media (prefers-reduced-motion: reduce) {
  .gen-card__icon i { animation: none; opacity: 0.6; }
}
</style>
