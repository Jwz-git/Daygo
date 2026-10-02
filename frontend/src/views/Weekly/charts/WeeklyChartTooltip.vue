<script setup lang="ts">
/*
 * Floating tooltip for the weekly charts, after Dayflow's treemap hover card:
 * a near-opaque rounded card above the pointer. It glides between positions
 * and fades in and out; it never takes pointer events.
 */
defineProps<{ visible: boolean; x: number; y: number }>()
</script>

<template>
  <Transition name="wk-tip">
    <div v-if="visible" class="wk-tip" :style="{ left: `${x}px`, top: `${y}px` }" role="tooltip">
      <slot />
    </div>
  </Transition>
</template>

<style scoped>
.wk-tip {
  position: absolute;
  z-index: 20;
  min-width: 140px;
  max-width: 240px;
  padding: 9px 11px;
  border: 1px solid var(--dg-wk-card-border);
  border-radius: 8px;
  background: var(--dg-wk-tooltip);
  box-shadow: 0 10px 28px -10px rgba(60, 40, 28, 0.32), 0 2px 6px rgba(60, 40, 28, 0.08);
  color: var(--dg-wk-text);
  font-size: 13px;
  line-height: 1.45;
  pointer-events: none;
  transform: translate(-50%, calc(-100% - 14px));
  transition:
    left 90ms ease-out,
    top 90ms ease-out;
}

.wk-tip :slotted(b) {
  display: block;
  margin-bottom: 2px;
  font-size: 13px;
  font-weight: 650;
}

.wk-tip :slotted(span) {
  display: block;
  color: var(--dg-wk-text-secondary);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}

.wk-tip-enter-active,
.wk-tip-leave-active {
  transition:
    opacity 160ms ease-out,
    transform 200ms cubic-bezier(0.22, 1, 0.36, 1);
}

.wk-tip-enter-from,
.wk-tip-leave-to {
  opacity: 0;
  transform: translate(-50%, calc(-100% - 6px)) scale(0.97);
}

@media (prefers-reduced-motion: reduce) {
  .wk-tip,
  .wk-tip-enter-active,
  .wk-tip-leave-active {
    transition: none;
  }
}
</style>
