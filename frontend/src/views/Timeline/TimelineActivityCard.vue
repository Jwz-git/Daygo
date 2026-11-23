<script setup lang="ts">
import { computed, type CSSProperties } from 'vue'
import { useI18n } from 'vue-i18n'

import type { TimelineCardDTO } from '@/api/dto'
import AppSiteIcon from '@/components/AppSiteIcon.vue'
import { appSiteValues } from '@/lib/appSiteIcon'
import { categoryLabel } from '@/lib/categoryLabel'
import { useDurationFormat } from '@/lib/duration'
import { useUIVisibilityStore } from '@/stores/uiVisibility'

import { cardIsCompact, cardShowsText, MIN_CARD_HEIGHT } from './layout'

const props = defineProps<{
  card: TimelineCardDTO
  color: string
  selected: boolean
  /** A batch covering this card is being analyzed again; the card is stale. */
  regenerating: boolean
  top: number
  height: number
  /** Minutes this row covers, after any overlap trim. */
  minutes: number
}>()

const emit = defineEmits<{ select: [id: number] }>()
const candidateSites = computed(() => appSiteValues(props.card.appSites))
const hasIcon = computed(() => candidateSites.value.length > 0)
const { t } = useI18n()
const formatDuration = useDurationFormat()
const visibility = useUIVisibilityStore()

const translatedCategory = computed(() => categoryLabel(props.card.category, t))

// Mirrors the reference timeline: a row of a few minutes cannot hold a title
// without shrinking it past legibility, so it stays a bare bar; a slightly
// longer one keeps a tight, vertically centred line.
const showsText = computed(() => cardShowsText(props.minutes))
const compact = computed(() => cardIsCompact(props.minutes))
const showsSummary = computed(
  () => showsText.value && !compact.value && !props.card.isIdle && props.height >= 58 && props.card.summary.trim() !== '',
)
const showsFooter = computed(
  () => showsText.value && !compact.value && !props.card.isIdle && props.height >= 114,
)
const showsWrapTitle = computed(
  () => showsText.value && !compact.value && props.height >= 138,
)
const centered = computed(
  () => showsText.value && !compact.value && !showsSummary.value && !showsFooter.value && props.height < 64,
)
const summaryLines = computed(() => {
  if (props.height >= 196) return 4
  if (props.height >= 164) return 3
  if (props.height >= 80) return 2
  return 1
})
const durationLabel = computed(() =>
  formatDuration(props.card.durationMinutes > 0 ? props.card.durationMinutes : props.minutes),
)

function cardStyle(): CSSProperties {
  return {
    top: `${props.top}px`,
    height: `${Math.max(MIN_CARD_HEIGHT, props.height)}px`,
    '--timeline-category': props.color,
  }
}
</script>

<template>
  <button
    type="button"
    class="activity-card"
    :class="{
      'is-selected': props.selected,
      'is-bare': !showsText,
      'is-compact': showsText && compact,
      'is-centered': centered,
      'is-detailed': showsWrapTitle,
      'is-idle': props.card.isIdle,
      'is-regenerating': props.regenerating,
      'is-window-hidden': !visibility.visible,
    }"
    :style="cardStyle()"
    :title="!showsText ? `${props.card.title} · ${props.card.start} – ${props.card.end}` : undefined"
    :aria-pressed="props.selected"
    :aria-busy="props.regenerating"
    :aria-label="`${props.card.title}, ${props.card.start} – ${props.card.end}, ${translatedCategory}`"
    @click="emit('select', props.card.id)"
  >
    <span class="activity-card__rail" aria-hidden="true"></span>
    <template v-if="showsText">
      <span class="activity-card__head">
        <span v-if="hasIcon" class="activity-card__icon-slot" aria-hidden="true">
          <AppSiteIcon
            class="activity-card__icon"
            :sites="candidateSites"
            :accent="props.color"
            :size="compact ? 15 : 18"
          />
        </span>
        <span class="activity-card__title">{{ props.card.title }}</span>
        <span class="activity-card__time">{{ props.card.start }} – {{ props.card.end }}</span>
      </span>

      <span
        v-if="showsSummary"
        class="activity-card__summary"
        :class="{ 'is-indented': hasIcon }"
        :style="{ '-webkit-line-clamp': summaryLines }"
      >{{ props.card.summary }}</span>

      <span
        v-if="showsFooter"
        class="activity-card__footer"
        :class="{ 'is-indented': hasIcon }"
      >
        <span class="activity-card__category">
          <i class="activity-card__category-dot" aria-hidden="true"></i>
          {{ translatedCategory }}
        </span>
        <span class="activity-card__duration">{{ durationLabel }}</span>
      </span>
    </template>
  </button>
</template>

<style scoped>
.activity-card {
  --card-surface-rest: linear-gradient(
    102deg,
    color-mix(in srgb, var(--timeline-category) 11%, var(--dg-timeline-card-surface)) 0%,
    color-mix(in srgb, var(--timeline-category) 4.5%, var(--dg-timeline-card-surface)) 48%,
    color-mix(in srgb, var(--timeline-category) 1.5%, var(--dg-timeline-card-surface)) 100%
  );
  --card-surface-hover: linear-gradient(
    102deg,
    color-mix(in srgb, var(--timeline-category) 16%, var(--dg-timeline-card-surface-hover)) 0%,
    color-mix(in srgb, var(--timeline-category) 7%, var(--dg-timeline-card-surface-hover)) 52%,
    color-mix(in srgb, var(--timeline-category) 3%, var(--dg-timeline-card-surface-hover)) 100%
  );
  --card-surface-selected: linear-gradient(
    102deg,
    color-mix(in srgb, var(--timeline-category) 21%, var(--dg-timeline-card-surface-selected)) 0%,
    color-mix(in srgb, var(--timeline-category) 10%, var(--dg-timeline-card-surface-selected)) 56%,
    color-mix(in srgb, var(--timeline-category) 4.5%, var(--dg-timeline-card-surface-selected)) 100%
  );
  --card-surface-bare: linear-gradient(
    90deg,
    color-mix(in srgb, var(--timeline-category) 19%, var(--dg-timeline-card-surface)) 0%,
    color-mix(in srgb, var(--timeline-category) 7%, var(--dg-timeline-card-surface)) 100%
  );
  --card-border-rest: color-mix(in srgb, var(--timeline-category) 22%, var(--dg-timeline-card-border));
  --card-border-hover: color-mix(in srgb, var(--timeline-category) 46%, var(--dg-timeline-card-border));
  --card-border-selected: color-mix(in srgb, var(--timeline-category) 66%, var(--dg-timeline-card-border));
  --card-pill-bg: color-mix(in srgb, var(--timeline-category) 12%, transparent);
  --card-pill-text: color-mix(in srgb, var(--timeline-category) 28%, var(--dg-text-primary));

  position: absolute;
  z-index: 3;
  right: 14px;
  left: 8px;
  display: flex;
  flex-direction: column;
  align-items: stretch;
  justify-content: flex-start;
  padding: 8px 13px 8px 16px;
  overflow: hidden;
  border: 1px solid var(--card-border-rest);
  border-radius: 9px;
  background: var(--card-surface-rest);
  box-shadow: var(--dg-timeline-card-shadow);
  text-align: left;
  cursor: pointer;
  transition:
    border-color var(--dg-motion-base) ease-out,
    background var(--dg-motion-base) ease-out,
    box-shadow var(--dg-motion-base) var(--dg-ease-out),
    transform 160ms var(--dg-ease-out);
}

:root[data-dg-appearance='dark'] .activity-card {
  --card-surface-rest: linear-gradient(
    102deg,
    color-mix(in srgb, var(--timeline-category) 18%, var(--dg-timeline-card-surface)) 0%,
    color-mix(in srgb, var(--timeline-category) 8.5%, var(--dg-timeline-card-surface)) 48%,
    color-mix(in srgb, var(--timeline-category) 3.5%, var(--dg-timeline-card-surface)) 100%
  );
  --card-surface-hover: linear-gradient(
    102deg,
    color-mix(in srgb, var(--timeline-category) 24%, var(--dg-timeline-card-surface-hover)) 0%,
    color-mix(in srgb, var(--timeline-category) 12%, var(--dg-timeline-card-surface-hover)) 52%,
    color-mix(in srgb, var(--timeline-category) 5.5%, var(--dg-timeline-card-surface-hover)) 100%
  );
  --card-surface-selected: linear-gradient(
    102deg,
    color-mix(in srgb, var(--timeline-category) 30%, var(--dg-timeline-card-surface-selected)) 0%,
    color-mix(in srgb, var(--timeline-category) 16%, var(--dg-timeline-card-surface-selected)) 56%,
    color-mix(in srgb, var(--timeline-category) 8%, var(--dg-timeline-card-surface-selected)) 100%
  );
  --card-surface-bare: linear-gradient(
    90deg,
    color-mix(in srgb, var(--timeline-category) 26%, var(--dg-timeline-card-surface)) 0%,
    color-mix(in srgb, var(--timeline-category) 11%, var(--dg-timeline-card-surface)) 100%
  );
  --card-border-rest: color-mix(in srgb, var(--timeline-category) 28%, var(--dg-timeline-card-border));
  --card-border-hover: color-mix(in srgb, var(--timeline-category) 50%, var(--dg-timeline-card-border));
  --card-border-selected: color-mix(in srgb, var(--timeline-category) 72%, var(--dg-timeline-card-border));
  --card-pill-bg: color-mix(in srgb, var(--timeline-category) 20%, transparent);
  --card-pill-text: color-mix(in srgb, var(--timeline-category) 32%, var(--dg-text-primary));
}

/* Single-line standard cards (e.g. the common 15m / 20m rows) center cleanly
   on the vertical midline instead of hanging top-heavy. */
.activity-card.is-centered {
  justify-content: center;
  padding: 0 13px 0 16px;
}

/* A row too short for the roomy layout: tight, vertically centred single line. */
.activity-card.is-compact {
  justify-content: center;
  padding: 0 11px 0 15px;
  border-radius: 7px;
}

/* A very short sliver (<10 min): tinted capsule bar with no text. */
.activity-card.is-bare {
  padding: 0;
  border-radius: 5px;
  background: var(--card-surface-bare);
}

/* Idle intervals recede quietly so active work blocks carry visual priority. */
.activity-card.is-idle {
  border-style: dashed;
  border-color: var(--dg-timeline-card-idle-border);
  background-color: var(--dg-timeline-card-idle-fill);
  background-image: repeating-linear-gradient(
    -45deg,
    transparent 0 7px,
    var(--dg-timeline-card-idle-hatch) 7px 8px
  );
  box-shadow: none;
}

.activity-card.is-idle .activity-card__rail {
  width: 2.5px;
  opacity: 0.45;
  box-shadow: none;
}

.activity-card.is-idle .activity-card__title {
  color: var(--dg-text-muted);
  font-size: 12.5px;
  font-weight: 500;
}

.activity-card.is-idle .activity-card__time {
  opacity: 0.72;
}

/* Hover elevates with depth and tonal glow — no scale transform, so CJK and
   Latin glyphs stay pixel-crisp in macOS WKWebView. */
.activity-card:hover {
  z-index: 5;
  border-color: var(--card-border-hover);
  background: var(--card-surface-hover);
  box-shadow:
    var(--dg-timeline-card-shadow-hover),
    0 2px 10px color-mix(in srgb, var(--timeline-category) 13%, transparent);
  transform: translateY(-1px);
}

.activity-card.is-bare:hover {
  transform: none;
}

.activity-card.is-idle:not(.is-selected):not(.is-regenerating):hover {
  border-style: solid;
  border-color: var(--card-border-hover);
  background-color: var(--dg-timeline-card-idle-hover);
  background-image: repeating-linear-gradient(
    -45deg,
    transparent 0 7px,
    var(--dg-timeline-card-idle-hatch) 7px 8px
  );
  box-shadow: var(--dg-timeline-card-shadow);
}

.activity-card.is-idle:not(.is-selected):hover .activity-card__title {
  color: var(--dg-text-secondary);
}

.activity-card:hover .activity-card__rail {
  box-shadow: 0 0 10px color-mix(in srgb, var(--timeline-category) 42%, transparent);
}

.activity-card:hover .activity-card__time {
  color: var(--dg-text-secondary);
  opacity: 1;
}

.activity-card:focus-visible {
  z-index: 6;
  outline: none;
  box-shadow: 0 0 0 3px var(--dg-focus-ring), var(--dg-timeline-card-shadow);
}

.activity-card.is-selected {
  z-index: 4;
  border-style: solid;
  border-color: var(--card-border-selected);
  background: var(--card-surface-selected);
  box-shadow:
    inset 0 0 0 1px color-mix(in srgb, var(--timeline-category) 22%, transparent),
    0 3px 12px color-mix(in srgb, var(--timeline-category) 16%, transparent),
    var(--dg-timeline-card-shadow);
}

.activity-card.is-selected:hover {
  z-index: 5;
}

.activity-card.is-selected .activity-card__rail {
  width: 4px;
  opacity: 1;
  box-shadow: 0 0 10px color-mix(in srgb, var(--timeline-category) 48%, transparent);
}

.activity-card.is-selected.is-idle .activity-card__title {
  color: var(--dg-text-primary);
}

/* Press settles the card back onto the track without scaling the text. */
.activity-card:active {
  z-index: 6;
  border-style: solid;
  border-color: var(--card-border-selected);
  background: var(--card-surface-selected);
  box-shadow:
    inset 0 0 0 1px color-mix(in srgb, var(--timeline-category) 20%, transparent),
    inset 0 1px 3px rgba(20, 16, 25, 0.1);
  transform: translateY(0);
  transition:
    border-color var(--dg-motion-fast) ease-out,
    background var(--dg-motion-fast) ease-out,
    box-shadow var(--dg-motion-fast) ease-out,
    transform 100ms var(--dg-ease-out);
}

/* A card whose window is being analyzed again: carries the generating wash
   plus a subtle sweep so stale cards read as live regeneration. */
.activity-card.is-regenerating {
  border-style: solid;
  border-color: color-mix(in srgb, var(--dg-accent) 44%, var(--dg-timeline-card-border));
  background-color: var(--dg-timeline-card-surface);
  background-image: var(--dg-status-generating);
}

.activity-card.is-regenerating.is-selected {
  border-color: var(--card-border-selected);
}

.activity-card.is-regenerating::after {
  content: '';
  position: absolute;
  inset: 0;
  pointer-events: none;
  background: linear-gradient(
    110deg,
    transparent 24%,
    rgba(255, 255, 255, 0.24) 50%,
    transparent 76%
  );
  transform: translateX(-100%);
  animation: activity-card-shimmer 2.1s ease-in-out infinite;
}

.activity-card.is-regenerating.is-window-hidden::after {
  animation-play-state: paused;
}

@keyframes activity-card-shimmer {
  0% { transform: translateX(-100%); }
  65%, 100% { transform: translateX(100%); }
}

.activity-card__rail {
  position: absolute;
  top: 7px;
  bottom: 7px;
  left: 6px;
  width: 3.5px;
  border-radius: 99px;
  background: var(--timeline-category);
  box-shadow: 0 0 6px color-mix(in srgb, var(--timeline-category) 24%, transparent);
  transition:
    width var(--dg-motion-fast) ease-out,
    opacity var(--dg-motion-fast) ease-out,
    box-shadow var(--dg-motion-base) ease-out;
}

.activity-card.is-compact .activity-card__rail {
  top: 4px;
  bottom: 4px;
  left: 5px;
  width: 3px;
}

.activity-card.is-bare .activity-card__rail {
  top: 2px;
  bottom: 2px;
  left: 4px;
  width: 3px;
}

.activity-card__head {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  min-width: 0;
}

.activity-card.is-compact .activity-card__head {
  gap: 7px;
}

.activity-card.is-detailed .activity-card__head {
  align-items: flex-start;
}

.activity-card__icon-slot {
  flex: none;
  width: 18px;
  height: 18px;
  display: grid;
  place-items: center;
}

.activity-card.is-compact .activity-card__icon-slot {
  width: 15px;
  height: 15px;
}

.activity-card.is-detailed .activity-card__icon-slot {
  /* Matches the title's first line (13.5px x 1.35 ≈ 18.2px). */
  height: 19px;
}

.activity-card__title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  color: var(--dg-text-primary);
  font-size: 13.5px;
  font-weight: 600;
  letter-spacing: -0.008em;
  line-height: 1.35;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.activity-card.is-compact .activity-card__title {
  font-size: 12.5px;
  line-height: 1.25;
}

/* A tall card wraps its title onto a second line instead of ellipsizing on one. */
.activity-card.is-detailed .activity-card__title {
  display: -webkit-box;
  white-space: normal;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.activity-card__time {
  flex: none;
  margin-left: 6px;
  color: var(--dg-card-time);
  font-size: 11px;
  font-weight: 500;
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.01em;
  line-height: 18px;
  white-space: nowrap;
  opacity: 0.88;
  transition: color var(--dg-motion-fast) ease-out, opacity var(--dg-motion-fast) ease-out;
}

.activity-card.is-compact .activity-card__time {
  font-size: 10.5px;
  line-height: 15px;
}

.activity-card.is-detailed .activity-card__time {
  line-height: 19px;
}

/* Summary spans the full card width right beneath the timestamp, aligned on
   the left with the title's edge whenever an icon is present. */
.activity-card__summary {
  display: -webkit-box;
  margin-top: 4px;
  overflow: hidden;
  color: var(--dg-text-secondary);
  font-size: 12px;
  font-weight: 400;
  line-height: 1.45;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.activity-card__summary.is-indented,
.activity-card__footer.is-indented {
  padding-left: 26px;
}

.activity-card__footer {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: auto;
  padding-top: 6px;
  min-width: 0;
}

.activity-card__category {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  max-width: 100%;
  padding: 2px 8px 2px 6px;
  overflow: hidden;
  border-radius: 999px;
  background: var(--card-pill-bg);
  color: var(--card-pill-text);
  font-size: 11px;
  font-weight: 550;
  line-height: 1.3;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.activity-card__category-dot {
  flex: none;
  width: 5.5px;
  height: 5.5px;
  border-radius: 50%;
  background: var(--timeline-category);
}

.activity-card__duration {
  flex: none;
  color: var(--dg-text-muted);
  font-size: 11px;
  font-weight: 500;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

@media (max-width: 720px) {
  .activity-card__time { display: none; }
}

@media (prefers-reduced-motion: reduce) {
  .activity-card,
  .activity-card__rail,
  .activity-card__time {
    transition: none;
  }

  .activity-card:hover {
    transform: none;
  }

  .activity-card.is-regenerating::after {
    display: none;
  }
}

@media (forced-colors: active) {
  .activity-card,
  .activity-card.is-idle,
  .activity-card.is-bare {
    border: 1px solid ButtonText;
    background: Canvas;
    box-shadow: none;
  }

  .activity-card.is-selected {
    border-color: Highlight;
  }
}
</style>
