<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type {
  CategoryDTO,
  DayContextDTO,
  RangeDTO,
  TimelineCardDTO,
  TimelineFailureDTO,
} from '@/api/dto'

import TimelineActivityCard from './TimelineActivityCard.vue'
import {
  MIN_CARD_HEIGHT,
  positionRange,
  safeCategoryColor,
  trackHeight,
} from './layout'

const props = defineProps<{
  context: DayContextDTO
  cards: TimelineCardDTO[]
  categories: CategoryDTO[]
  failures: TimelineFailureDTO[]
  processingRanges: RangeDTO[]
  selectedCardID: number | null
}>()

const emit = defineEmits<{ select: [id: number] }>()
const { locale, t } = useI18n()

const height = computed(() => trackHeight(props.context.dayStartTs, props.context.dayEndTs))

const categoryColors = computed(() =>
  new Map(props.categories.map((category) => [category.name, safeCategoryColor(category.colorHex)])),
)

const hourMarks = computed(() => {
  const marks: Array<{ ts: number; label: string; top: number }> = []
  const step = 60 * 60
  for (let ts = props.context.dayStartTs; ts < props.context.dayEndTs; ts += step) {
    marks.push({
      ts,
      label: new Intl.DateTimeFormat(locale.value, {
        hour: 'numeric',
        minute: '2-digit',
        timeZone: props.context.timeZone,
      }).format(new Date(ts * 1000)),
      top: positionRange(
        ts,
        ts,
        props.context.dayStartTs,
        props.context.dayEndTs,
        height.value,
      ).top,
    })
  }
  return marks
})

const nowPosition = computed(() => {
  if (
    props.context.nowTs < props.context.dayStartTs ||
    props.context.nowTs >= props.context.dayEndTs
  ) return null
  return positionRange(
    props.context.nowTs,
    props.context.nowTs,
    props.context.dayStartTs,
    props.context.dayEndTs,
    height.value,
  ).top
})

function placed(startTs: number, endTs: number, minimumHeight = 2) {
  return positionRange(
    startTs,
    endTs,
    props.context.dayStartTs,
    props.context.dayEndTs,
    height.value,
    minimumHeight,
  )
}
</script>

<template>
  <section class="timeline-track" :aria-label="t('timeline.track.ariaLabel')">
    <div class="timeline-track__canvas" :style="{ height: `${height}px` }">
      <div
        v-for="mark in hourMarks"
        :key="mark.ts"
        class="hour-mark"
        :style="{ top: `${mark.top}px` }"
      >
        <time :datetime="new Date(mark.ts * 1000).toISOString()">{{ mark.label }}</time>
        <span aria-hidden="true"></span>
      </div>

      <div class="timeline-track__events">
        <div
          v-for="range in props.processingRanges"
          :key="`processing-${range.startTs}-${range.endTs}`"
          class="range range--processing"
          :style="{
            top: `${placed(range.startTs, range.endTs, 34).top}px`,
            height: `${placed(range.startTs, range.endTs, 34).height}px`,
          }"
          role="status"
        >
          <span>{{ t('timeline.processing') }}</span>
        </div>

        <div
          v-for="failure in props.failures"
          :key="`failure-${failure.startTs}-${failure.endTs}`"
          class="range range--failure"
          :style="{
            top: `${placed(failure.startTs, failure.endTs, 42).top}px`,
            height: `${placed(failure.startTs, failure.endTs, 42).height}px`,
          }"
          role="alert"
        >
          <span class="range__title">{{ t('timeline.failure.title') }}</span>
          <span class="range__message">{{ failure.message }}</span>
        </div>

        <TimelineActivityCard
          v-for="card in props.cards"
          :key="card.id"
          :card="card"
          :color="categoryColors.get(card.category) ?? safeCategoryColor(undefined)"
          :selected="card.id === props.selectedCardID"
          :top="placed(card.startTs, card.endTs, MIN_CARD_HEIGHT).top"
          :height="placed(card.startTs, card.endTs, MIN_CARD_HEIGHT).height"
          @select="emit('select', $event)"
        />

        <div
          v-if="nowPosition !== null"
          class="now-line"
          :style="{ top: `${nowPosition}px` }"
          aria-hidden="true"
        >
          <span></span>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.timeline-track {
  min-height: 0;
  overflow-y: auto;
  overscroll-behavior: contain;
  scrollbar-gutter: stable;
}

.timeline-track__canvas {
  position: relative;
  min-width: 0;
}

.hour-mark {
  position: absolute;
  right: 0;
  left: 0;
  display: grid;
  grid-template-columns: var(--dg-timeline-time-width) minmax(0, 1fr);
  align-items: center;
  transform: translateY(-7px);
}

.hour-mark time {
  padding-right: 14px;
  color: var(--dg-text-muted);
  font-size: 10px;
  font-variant-numeric: tabular-nums;
  text-align: right;
  white-space: nowrap;
}

.hour-mark span {
  border-top: 1px solid var(--dg-timeline-grid);
}

.timeline-track__events {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  left: var(--dg-timeline-time-width);
  border-left: 1px solid var(--dg-timeline-grid-strong);
}

.range {
  position: absolute;
  z-index: 2;
  right: 10px;
  left: 2px;
  display: flex;
  align-items: center;
  min-height: 34px;
  padding: 8px 12px;
  overflow: hidden;
  border-radius: var(--dg-timeline-card-radius);
  font-size: 11px;
}

.range--processing {
  border: 1px solid var(--dg-timeline-processing-border);
  background:
    linear-gradient(90deg, transparent 0%, var(--dg-timeline-processing-shine) 48%, transparent 100%),
    var(--dg-timeline-processing-fill);
  background-size: 180% 100%, auto;
  color: var(--dg-text-secondary);
  animation: timeline-shimmer 1.8s linear infinite;
}

.range--failure {
  flex-direction: column;
  align-items: flex-start;
  justify-content: center;
  border: 1px solid color-mix(in srgb, var(--dg-danger) 34%, transparent);
  background: var(--dg-danger-fill);
  color: var(--dg-danger);
}

.range__title { font-weight: 650; }
.range__message {
  max-width: 100%;
  overflow: hidden;
  color: var(--dg-text-secondary);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.now-line {
  position: absolute;
  z-index: 7;
  right: 4px;
  left: -4px;
  height: 1px;
  background: var(--dg-accent);
  box-shadow: 0 0 10px var(--dg-focus-ring);
  pointer-events: none;
}

.now-line span {
  position: absolute;
  top: -3px;
  left: 0;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--dg-accent);
}

@keyframes timeline-shimmer {
  to { background-position: -180% 0, 0 0; }
}

@media (prefers-reduced-motion: reduce) {
  .range--processing { animation: none; }
}
</style>
