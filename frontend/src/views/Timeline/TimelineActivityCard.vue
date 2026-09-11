<script setup lang="ts">
import type { CSSProperties } from 'vue'

import type { TimelineCardDTO } from '@/api/dto'

const props = defineProps<{
  card: TimelineCardDTO
  color: string
  selected: boolean
  top: number
  height: number
}>()

const emit = defineEmits<{ select: [id: number] }>()

function cardStyle(): CSSProperties {
  return {
    top: `${props.top}px`,
    height: `${props.height}px`,
    '--timeline-category': props.color,
  }
}
</script>

<template>
  <button
    type="button"
    class="activity-card"
    :class="{ 'is-selected': props.selected, 'is-compact': props.height < 54 }"
    :style="cardStyle()"
    :aria-pressed="props.selected"
    @click="emit('select', props.card.id)"
  >
    <span class="activity-card__rail" aria-hidden="true"></span>
    <span class="activity-card__copy">
      <span class="activity-card__title">{{ props.card.title }}</span>
      <span v-if="props.height >= 64" class="activity-card__summary">{{ props.card.summary }}</span>
    </span>
    <span class="activity-card__time">{{ props.card.start }} – {{ props.card.end }}</span>
  </button>
</template>

<style scoped>
.activity-card {
  position: absolute;
  z-index: 3;
  right: 10px;
  left: 2px;
  display: flex;
  align-items: flex-start;
  gap: 11px;
  min-height: 38px;
  padding: 9px 12px 9px 14px;
  overflow: hidden;
  border: 1px solid var(--dg-timeline-card-border);
  border-radius: var(--dg-timeline-card-radius);
  background: var(--dg-timeline-card-fill);
  box-shadow: var(--dg-timeline-card-shadow);
  text-align: left;
  transition:
    border-color var(--dg-motion-fast) ease,
    background var(--dg-motion-fast) ease,
    box-shadow var(--dg-motion-fast) ease;
}

.activity-card:hover {
  z-index: 5;
  border-color: color-mix(in srgb, var(--timeline-category) 40%, var(--dg-timeline-card-border));
  background: var(--dg-timeline-card-hover);
}

.activity-card:focus-visible {
  z-index: 6;
  outline: none;
  box-shadow: 0 0 0 3px var(--dg-focus-ring), var(--dg-timeline-card-shadow);
}

.activity-card.is-selected {
  z-index: 4;
  border-color: color-mix(in srgb, var(--timeline-category) 52%, var(--dg-timeline-card-border));
  background: var(--dg-timeline-card-selected);
  box-shadow:
    inset 0 0 0 1px color-mix(in srgb, var(--timeline-category) 15%, transparent),
    var(--dg-timeline-card-shadow);
}

.activity-card__rail {
  position: absolute;
  top: 7px;
  bottom: 7px;
  left: 5px;
  width: 3px;
  border-radius: 99px;
  background: var(--timeline-category);
}

.activity-card__copy {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-width: 0;
}

.activity-card__title {
  overflow: hidden;
  color: var(--dg-text-primary);
  font-size: 13px;
  font-weight: 620;
  line-height: 18px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.activity-card__summary {
  display: -webkit-box;
  margin-top: 4px;
  overflow: hidden;
  color: var(--dg-text-secondary);
  font-size: 11px;
  line-height: 16px;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.activity-card__time {
  flex: none;
  color: var(--dg-card-time);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
  line-height: 18px;
  white-space: nowrap;
}

.activity-card.is-compact {
  align-items: center;
  padding-top: 6px;
  padding-bottom: 6px;
}

@media (max-width: 720px) {
  .activity-card__time { display: none; }
}
</style>
