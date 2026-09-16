<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import type {
  CategoryDTO,
  DayContextDTO,
  RangeDTO,
  TimelineCardDTO,
  TimelineFailureDTO,
} from '@/api/dto'
import { safeTimeZone } from '@/lib/timeZone'

import TimelineActivityCard from './TimelineActivityCard.vue'
import GeneratingCard from '@/components/GeneratingCard.vue'
import {
  MIN_CARD_HEIGHT,
  coveredBy,
  layoutTimelineCards,
  positionRange,
  safeCategoryColor,
  trackHeight,
  uncoveredBy,
} from './layout'

const props = defineProps<{
  context: DayContextDTO
  cards: TimelineCardDTO[]
  categories: CategoryDTO[]
  failures: TimelineFailureDTO[]
  processingRanges: RangeDTO[]
  selectedCardID: number | null
  selectedFailureTs: number | null
  /** Placeholder state at the current time: off, live capture, or paused hold. */
  generating: 'off' | 'capturing' | 'paused'
}>()

const emit = defineEmits<{
  select: [id: number]
  selectFailure: [startTs: number]
  clear: []
}>()
const { locale, t } = useI18n()
const scroller = ref<HTMLElement | null>(null)
const nowTs = ref(Math.floor(Date.now() / 1000))
let nowTimer: number | null = null

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
        timeZone: safeTimeZone(props.context.timeZone),
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
    nowTs.value < props.context.dayStartTs ||
    nowTs.value >= props.context.dayEndTs
  ) return null
  return positionRange(
    nowTs.value,
    nowTs.value,
    props.context.dayStartTs,
    props.context.dayEndTs,
    height.value,
  ).top
})

const placedCards = computed(() =>
  layoutTimelineCards(
    props.cards,
    props.context.dayStartTs,
    props.context.dayEndTs,
    height.value,
  ),
)

const cardPlacement = computed(
  () => new Map(placedCards.value.map((card) => [card.id, card])),
)

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

/* Mirrors the min-height of .range in the stylesheet below: a short window
   still draws a full-height block, and that box is what has to be compared. */
const PROCESSING_MIN_HEIGHT = 34

const processingBoxes = computed(() =>
  props.processingRanges.map((range) => ({
    range,
    ...placed(range.startTs, range.endTs, PROCESSING_MIN_HEIGHT),
  })),
)

/*
 * A window belongs to the card that already covers it. Re-analyzing a day puts
 * its batches back to pending while their cards are still stored, so the two
 * would claim the same box and paint over each other. The block is the stand-in
 * for a gap, so it yields where a card exists, and that card takes the
 * regenerating state instead of a second box on top of it.
 */
const visibleProcessing = computed(() => uncoveredBy(processingBoxes.value, placedCards.value))

const regeneratingCardIDs = computed(
  () => new Set(coveredBy(placedCards.value, processingBoxes.value).map((card) => card.id)),
)

function handleTrackClick(event: MouseEvent): void {
  const target = event.target as HTMLElement
  if (target.closest('.activity-card, .range--failure') !== null) return
  emit('clear')
}

async function revealRelevantTime(): Promise<void> {
  await nextTick()
  const element = scroller.value
  if (element === null) return

  const firstEventTs = Math.min(
    ...props.cards.map((card) => card.startTs),
    ...props.processingRanges.map((range) => range.startTs),
    ...props.failures.map((failure) => failure.startTs),
  )
  const targetTs = nowPosition.value !== null
    ? props.context.nowTs
    : Number.isFinite(firstEventTs)
      ? firstEventTs
      : props.context.dayStartTs
  const targetTop = placed(targetTs, targetTs).top
  element.scrollTop = Math.max(0, targetTop - element.clientHeight * 0.28)
}

onMounted(() => {
  nowTs.value = Math.floor(Date.now() / 1000)
  nowTimer = window.setInterval(() => { nowTs.value = Math.floor(Date.now() / 1000) }, 15_000)
  void revealRelevantTime()
})
watch(() => props.context.day, () => void revealRelevantTime())
onBeforeUnmount(() => {
  if (nowTimer !== null) window.clearInterval(nowTimer)
})
</script>

<template>
  <section ref="scroller" class="timeline-track" :aria-label="t('timeline.track.ariaLabel')" @click="handleTrackClick">
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
          v-for="block in visibleProcessing"
          :key="`processing-${block.range.startTs}-${block.range.endTs}`"
          class="range range--processing"
          :style="{ top: `${block.top}px`, height: `${block.height}px` }"
          role="status"
        >
          <svg viewBox="0 0 14 14" aria-hidden="true">
            <rect x="1" y="8" width="3" height="3" rx="0.8" fill="currentColor" opacity="0.55" />
            <rect x="5.5" y="5" width="3" height="6" rx="0.8" fill="currentColor" opacity="0.8" />
            <rect x="10" y="2.5" width="3" height="8.5" rx="0.8" fill="currentColor" />
          </svg>
          <span>{{ t('timeline.generating') }}</span>
        </div>

        <button
          v-for="failure in props.failures"
          :key="`failure-${failure.startTs}-${failure.endTs}`"
          type="button"
          class="range range--failure"
          :class="{ 'is-selected': failure.startTs === props.selectedFailureTs }"
          :style="{
            top: `${placed(failure.startTs, failure.endTs, 42).top}px`,
            height: `${placed(failure.startTs, failure.endTs, 42).height}px`,
          }"
          :aria-pressed="failure.startTs === props.selectedFailureTs"
          :aria-label="`${t('timeline.failure.title')}, ${failure.message}`"
          @click="emit('selectFailure', failure.startTs)"
        >
          <span class="range__copy">
            <span class="range__title">{{ t('timeline.failure.title') }}</span>
            <span class="range__message">{{ failure.message }}</span>
          </span>
        </button>

        <TimelineActivityCard
          v-for="card in props.cards"
          :key="card.id"
          :card="card"
          :color="categoryColors.get(card.category) ?? safeCategoryColor(undefined)"
          :selected="card.id === props.selectedCardID"
          :regenerating="regeneratingCardIDs.has(card.id)"
          :top="cardPlacement.get(card.id)?.top ?? placed(card.startTs, card.endTs, MIN_CARD_HEIGHT).top"
          :height="cardPlacement.get(card.id)?.height ?? placed(card.startTs, card.endTs, MIN_CARD_HEIGHT).height"
          :lane-index="cardPlacement.get(card.id)?.laneIndex ?? 0"
          :lane-count="cardPlacement.get(card.id)?.laneCount ?? 1"
          @select="emit('select', $event)"
        />

        <GeneratingCard
          v-if="props.generating !== 'off' && nowPosition !== null"
          :state="props.generating"
          :style="{
            top: `${nowPosition + 6}px`,
            right: '14px',
            left: '2px',
            position: 'absolute',
          }"
        />
      </div>
    </div>
  </section>
</template>

<style scoped>
.timeline-track {
  min-height: 0;
  padding-top: 7px;
  overflow-y: auto;
  overscroll-behavior: contain;
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
  transform: translateY(-8px);
}

.hour-mark time {
  padding-right: 14px;
  color: var(--dg-text-tertiary);
  font-size: 11px;
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
  /* Tracks the single-lane activity-card edge: 2px left + (100% - 12px) lane span - 4px card inset. */
  right: 14px;
  left: 2px;
  display: flex;
  align-items: center;
  min-height: 34px;
  padding: 8px 12px;
  overflow: hidden;
  font-size: 11px;
}

.range--processing {
  gap: 9px;
  border: none;
  border-radius: 8px;
  background: linear-gradient(
    100deg,
    color-mix(in srgb, var(--dg-accent) 30%, transparent),
    color-mix(in srgb, #e8804a 24%, transparent)
  );
  color: var(--dg-text-primary);
  font-weight: 550;
}

.range--processing svg { flex: none; width: 13px; height: 13px; }

.range--failure {
  justify-content: space-between;
  gap: 10px;
  border: 1px solid color-mix(in srgb, var(--dg-danger) 34%, transparent);
  background: var(--dg-danger-fill);
  color: var(--dg-danger);
  cursor: pointer;
  text-align: left;
  transition:
    border-color var(--dg-motion-base) ease-in-out,
    background var(--dg-motion-base) ease-in-out,
    transform var(--dg-motion-base) ease-in-out;
}

.range--failure:hover {
  border-color: color-mix(in srgb, var(--dg-danger) 55%, transparent);
  background: color-mix(in srgb, var(--dg-danger) 12%, var(--dg-danger-fill));
}

.range--failure:focus-visible {
  outline: none;
  box-shadow: 0 0 0 3px var(--dg-focus-ring);
}

.range--failure.is-selected {
  border-color: color-mix(in srgb, var(--dg-danger) 65%, transparent);
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--dg-danger) 25%, transparent);
}

.range--failure:active {
  box-shadow: inset 0 1px 3px rgba(20, 16, 25, 0.1);
  transform: scale(0.985);
}

.range__copy { display: flex; min-width: 0; flex-direction: column; }
.range__title { font-weight: 650; }
.range__message {
  max-width: 100%;
  overflow: hidden;
  color: var(--dg-text-secondary);
  text-overflow: ellipsis;
  white-space: nowrap;
}

</style>
