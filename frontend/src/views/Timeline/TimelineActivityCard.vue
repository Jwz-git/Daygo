<script setup lang="ts">
import { computed, type CSSProperties } from 'vue'
import { useI18n } from 'vue-i18n'

import type { TimelineCardDTO } from '@/api/dto'
import AppSiteIcon from '@/components/AppSiteIcon.vue'
import { preferredAppSite } from '@/lib/appSiteIcon'
import { categoryLabel } from '@/lib/categoryLabel'

import { MIN_CARD_HEIGHT } from './layout'

const props = defineProps<{
  card: TimelineCardDTO
  color: string
  selected: boolean
  /** A batch covering this card is being analyzed again; the card is stale. */
  regenerating: boolean
  top: number
  height: number
  laneIndex: number
  laneCount: number
}>()

const emit = defineEmits<{ select: [id: number] }>()
const appSite = computed(() => preferredAppSite(props.card.appSites))
const { t } = useI18n()

// Visual gap between consecutive cards: the slot owns `height`, the card
// renders slightly inset inside it, so neighbours never touch.
const CARD_GAP = 3

function cardStyle(): CSSProperties {
  return {
    top: `${props.top + CARD_GAP / 2}px`,
    height: `${Math.max(MIN_CARD_HEIGHT, props.height - CARD_GAP)}px`,
    '--timeline-category': props.color,
    '--timeline-lane-index': props.laneIndex,
    '--timeline-lane-count': props.laneCount,
  }
}
</script>

<template>
  <button
    type="button"
    class="activity-card"
    :class="{
      'is-selected': props.selected,
      'is-detailed': props.height >= 96,
      'is-collided': props.laneCount > 1,
      'is-regenerating': props.regenerating,
    }"
    :style="cardStyle()"
    :aria-pressed="props.selected"
    :aria-busy="props.regenerating"
    :aria-label="`${props.card.title}, ${props.card.start} – ${props.card.end}, ${categoryLabel(props.card.category, t)}`"
    @click="emit('select', props.card.id)"
  >
    <span class="activity-card__rail" aria-hidden="true"></span>
    <span class="activity-card__icon-slot" aria-hidden="true">
      <AppSiteIcon
        v-if="appSite"
        class="activity-card__icon"
        :site="appSite"
        :accent="props.color"
        :size="18"
      />
    </span>
    <span class="activity-card__copy">
      <span class="activity-card__title">{{ props.card.title }}</span>
    </span>
    <span class="activity-card__time">{{ props.card.start }} – {{ props.card.end }}</span>
  </button>
</template>

<style scoped>
.activity-card {
  position: absolute;
  z-index: 3;
  left: calc(2px + (100% - 12px) * var(--timeline-lane-index) / var(--timeline-lane-count));
  width: calc((100% - 12px) / var(--timeline-lane-count) - 4px);
  display: flex;
  /* Icon and text hang from the card top, not centered. */
  align-items: flex-start;
  gap: 9px;
  min-height: 34px;
  padding: 5px 12px 5px 14px;
  overflow: hidden;
  border: 1px solid var(--dg-timeline-card-border);
  background: var(--dg-timeline-card-fill);
  box-shadow: var(--dg-timeline-card-shadow);
  text-align: left;
  transition:
    border-color var(--dg-motion-base) ease-in-out,
    background var(--dg-motion-base) ease-in-out,
    box-shadow var(--dg-motion-base) ease-in-out,
    transform 720ms var(--dg-ease-glide);
}

.activity-card__icon-slot {
  flex: none;
  width: 18px;
  /* Same height as the title's first line (15px x 1.35), so the icon's
     optical center sits exactly on the text's center line. */
  height: 22px;
  display: grid;
  place-items: center;
}

/* Hover grows the card slightly; :active below presses it back in. */
.activity-card:hover {
  z-index: 5;
  border-color: color-mix(in srgb, var(--timeline-category) 40%, var(--dg-timeline-card-border));
  background: var(--dg-timeline-card-hover);
  box-shadow: var(--dg-timeline-card-shadow-hover);
  transform: scale(1.008);
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

/* A card whose window is being analyzed again. The tint is the one the track's
   generating block uses, so a stale card and an empty window read as the same
   state; it rides on background-image so hover and selection keep the surface. */
.activity-card.is-regenerating {
  background-image: linear-gradient(
    100deg,
    color-mix(in srgb, var(--dg-accent) 20%, transparent),
    color-mix(in srgb, #e8804a 16%, transparent)
  );
}

/* The press sinks the card into the track; the fill previews the selected
   state the click is about to lock in. */
.activity-card:active {
  z-index: 6;
  border-color: color-mix(in srgb, var(--timeline-category) 52%, var(--dg-timeline-card-border));
  background: var(--dg-timeline-card-selected);
  box-shadow:
    inset 0 0 0 1px color-mix(in srgb, var(--timeline-category) 15%, transparent),
    inset 0 1px 3px rgba(20, 16, 25, 0.1);
  transform: scale(0.985);
  transition:
    border-color var(--dg-motion-fast) ease-in-out,
    background var(--dg-motion-fast) ease-in-out,
    transform var(--dg-motion-fast) ease-in-out;
}

.activity-card__rail {
  position: absolute;
  top: 6px;
  bottom: 6px;
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
  font-size: 15px;
  font-weight: 700;
  line-height: 1.35;
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

.activity-card.is-detailed { align-items: flex-start; }

.activity-card.is-collided .activity-card__time { display: none; }

.activity-card.is-collided .activity-card__summary { -webkit-line-clamp: 1; }

@media (max-width: 720px) {
  .activity-card__time { display: none; }
}

@media (prefers-reduced-motion: reduce) {
  .activity-card {
    transition: none;
  }

  .activity-card:hover {
    transform: none;
  }
}
</style>
