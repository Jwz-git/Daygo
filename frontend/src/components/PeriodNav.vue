<script setup lang="ts">
import { useI18n } from 'vue-i18n'

/*
 * The ‹ › + "today/this week" control every period page puts in its header.
 * The captions differ per page (backend-required, future-unavailable …), so
 * the caller passes resolved title strings; this component owns the shape,
 * the arrow glyphs and the disabled treatment.
 */
const props = defineProps<{
  label: string
  backwardTitle: string
  forwardTitle: string
  canBackward: boolean
  canForward: boolean
  currentLabel: string
  currentDisabled?: boolean
}>()

const emit = defineEmits<{
  navigate: [offset: -1 | 1]
  current: []
}>()

const { t } = useI18n()
</script>

<template>
  <div class="period-nav" role="group" :aria-label="props.label">
    <button
      type="button"
      class="period-nav__arrow"
      :aria-label="t('common.action.previous')"
      :title="props.backwardTitle"
      :disabled="!props.canBackward"
      @click="emit('navigate', -1)"
    >
      <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M10 3.5 5.5 8l4.5 4.5" /></svg>
    </button>
    <button
      type="button"
      class="period-nav__current"
      :disabled="props.currentDisabled"
      @click="emit('current')"
    >
      {{ props.currentLabel }}
    </button>
    <button
      type="button"
      class="period-nav__arrow"
      :aria-label="t('common.action.next')"
      :title="props.forwardTitle"
      :disabled="!props.canForward"
      @click="emit('navigate', 1)"
    >
      <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M6 3.5 10.5 8 6 12.5" /></svg>
    </button>
  </div>
</template>

<style scoped>
/* Calendar.app's "‹ Today ›" control: one quiet capsule group. */
.period-nav {
  display: inline-flex;
  align-items: center;
  gap: 0;
  height: 28px;
  padding: 0 2px;
  border-radius: 999px;
  background: var(--dg-chip-fill);
  /* Header slots turn the surrounding region into a window-drag surface; the
     nav itself must remain clickable, so opt out at the root and let the
     buttons below inherit no-drag. */
  -webkit-app-region: no-drag;
  --wails-draggable: no-drag;
}

.period-nav__arrow,
.period-nav__current {
  display: grid;
  height: 24px;
  border-radius: 999px;
  color: var(--dg-text-primary);
  place-items: center;
  transition: background var(--dg-motion-fast) ease;
}

.period-nav__arrow {
  width: 26px;
}

.period-nav__arrow svg {
  width: 14px;
  height: 14px;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.7;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.period-nav__current {
  padding: 0 8px;
  font-size: var(--dg-text-body);
  font-weight: 500;
  white-space: nowrap;
}

.period-nav__arrow:not(:disabled):hover,
.period-nav__current:not(:disabled):hover {
  background: var(--dg-hover-fill-strong);
}

.period-nav__arrow:focus-visible,
.period-nav__current:focus-visible {
  outline: none;
  box-shadow: 0 0 0 3px var(--dg-focus-ring);
}

.period-nav__arrow:disabled,
.period-nav__current:disabled {
  color: var(--dg-text-muted);
  cursor: default;
}
</style>
