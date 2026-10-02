<script setup lang="ts">
import DgIcon from '@/components/DgIcon.vue'
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
      <DgIcon name="chevronLeft" :size="14" />
    </button>
    <button
      type="button"
      class="period-nav__arrow"
      :aria-label="t('common.action.next')"
      :title="props.forwardTitle"
      :disabled="!props.canForward"
      @click="emit('navigate', 1)"
    >
      <DgIcon name="chevronRight" :size="14" />
    </button>
    <button
      type="button"
      class="period-nav__current"
      :disabled="props.currentDisabled"
      @click="emit('current')"
    >
      {{ props.currentLabel }}
    </button>
  </div>
</template>

<style scoped>
/* One quiet capsule holding the step arrows and the jump-to-current action. */
.period-nav {
  display: inline-flex;
  flex: none;
  align-items: center;
  height: 28px;
  padding: 0 2px;
  border-radius: 999px;
  background: var(--dg-capsule-fill);
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
}

.period-nav__current {
  padding: 0 10px;
  font-size: 13px;
  font-weight: 500;
  white-space: nowrap;
}

.period-nav__arrow:not(:disabled):hover,
.period-nav__current:not(:disabled):hover {
  background: var(--dg-capsule-hover);
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

@media (prefers-reduced-motion: reduce) {
  .period-nav__arrow,
  .period-nav__current {
    transition: none;
  }
}
</style>
