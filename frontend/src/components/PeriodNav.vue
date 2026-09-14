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
      ‹
    </button>
    <button
      type="button"
      class="period-nav__arrow"
      :aria-label="t('common.action.next')"
      :title="props.forwardTitle"
      :disabled="!props.canForward"
      @click="emit('navigate', 1)"
    >
      ›
    </button>
    <button
      type="button"
      class="dg-chip dg-chip--filled"
      :disabled="props.currentDisabled"
      @click="emit('current')"
    >
      {{ props.currentLabel }}
    </button>
  </div>
</template>

<style scoped>
.period-nav {
  display: flex;
  align-items: center;
  gap: 5px;
  /* Header slots turn the surrounding region into a window-drag surface; the
     nav itself must remain clickable, so opt out at the root and let the
     buttons below inherit no-drag. */
  -webkit-app-region: no-drag;
  --wails-draggable: no-drag;
}

.period-nav__arrow {
  display: grid;
  width: 28px;
  height: 28px;
  border-radius: 50%;
  color: var(--dg-text-secondary);
  font-size: 25px;
  line-height: 1;
  place-items: center;
}

.period-nav__arrow:not(:disabled):hover {
  background: var(--dg-hover-fill);
}

.period-nav__arrow:focus-visible {
  outline: none;
  box-shadow: 0 0 0 3px var(--dg-focus-ring);
}

.period-nav__arrow:disabled {
  color: var(--dg-text-muted);
  cursor: default;
  opacity: 0.55;
}
</style>
