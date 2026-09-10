<script setup lang="ts">
/**
 * Placeholder for a section that a later milestone fills in. Every string it
 * renders comes from i18n; callers pass message keys, not copy.
 */
const props = defineProps<{
  titleKey: string
  descriptionKey?: string
  milestone: 1 | 2 | 3 | 4 | 5
}>()
</script>

<template>
  <section class="milestone-notice dg-card">
    <header class="milestone-notice__head">
      <h2 class="milestone-notice__title">{{ $t(props.titleKey) }}</h2>
      <span class="milestone-notice__tag">
        {{ $t('common.placeholder.milestoneLabel', { milestone: props.milestone }) }}
      </span>
    </header>
    <p v-if="props.descriptionKey" class="milestone-notice__description">
      {{ $t(props.descriptionKey) }}
    </p>
    <p class="milestone-notice__hint">
      {{ $t('common.placeholder.milestoneHint', { milestone: props.milestone }) }}
    </p>
  </section>
</template>

<style scoped>
.milestone-notice {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 9px;
  min-height: 120px;
  padding: 20px;
  overflow: hidden;
  border-style: dashed;
}

.milestone-notice::after {
  position: absolute;
  right: -32px;
  bottom: -44px;
  width: 104px;
  height: 104px;
  border-radius: 50%;
  /*
   * A soft ambient wash, not a ringed disc. Repeated down a grid of
   * placeholders (the weekly view stacks six) a hard-edged circle reads as a
   * stamped motif; a faint radial blob reads as texture and stops competing.
   */
  background: radial-gradient(
    circle at 50% 50%,
    var(--dg-control-fill) 0%,
    transparent 72%
  );
  content: '';
  opacity: 0.6;
  pointer-events: none;
}

.milestone-notice__head {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.milestone-notice__title {
  color: var(--dg-text-primary);
  font-size: 14px;
  font-weight: 600;
}

.milestone-notice__tag {
  flex: none;
  padding: 3px 8px;
  border-radius: 999px;
  background: var(--dg-control-fill);
  color: var(--dg-text-secondary);
  font-family: var(--dg-font-mono);
  font-size: 10px;
  letter-spacing: 0.04em;
}

.milestone-notice__description {
  position: relative;
  z-index: 1;
  max-width: 42rem;
  color: var(--dg-text-secondary);
  font-size: 13px;
}

.milestone-notice__hint {
  position: relative;
  z-index: 1;
  color: var(--dg-text-muted);
  font-size: 12px;
}
</style>
