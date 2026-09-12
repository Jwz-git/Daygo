<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
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
import {
  MIN_CARD_HEIGHT,
  layoutTimelineCards,
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
  canRetry: boolean
  retrying: boolean
}>()

const emit = defineEmits<{
  select: [id: number]
  retry: [batchIDs: number[]]
}>()
const { locale, t } = useI18n()
const scroller = ref<HTMLElement | null>(null)

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

onMounted(() => void revealRelevantTime())
watch(() => props.context.day, () => void revealRelevantTime())
</script>

<template>
  <section ref="scroller" class="timeline-track" :aria-label="t('timeline.track.ariaLabel')">
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
          <span class="range__copy">
            <span class="range__title">{{ t('timeline.failure.title') }}</span>
            <span class="range__message">{{ failure.message }}</span>
          </span>
          <button
            v-if="failure.retryable"
            type="button"
            class="range__retry"
            :disabled="!props.canRetry || props.retrying || failure.batchIds.length === 0"
            :title="props.canRetry ? t('timeline.failure.retry') : t('timeline.failure.retryUnavailable')"
            @click="emit('retry', failure.batchIds)"
          >
            {{ props.retrying ? t('timeline.failure.retrying') : t('common.action.retry') }}
          </button>
        </div>

        <TimelineActivityCard
          v-for="card in props.cards"
          :key="card.id"
          :card="card"
          :color="categoryColors.get(card.category) ?? safeCategoryColor(undefined)"
          :selected="card.id === props.selectedCardID"
          :top="cardPlacement.get(card.id)?.top ?? placed(card.startTs, card.endTs, MIN_CARD_HEIGHT).top"
          :height="cardPlacement.get(card.id)?.height ?? placed(card.startTs, card.endTs, MIN_CARD_HEIGHT).height"
          :lane-index="cardPlacement.get(card.id)?.laneIndex ?? 0"
          :lane-count="cardPlacement.get(card.id)?.laneCount ?? 1"
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
  padding-top: 7px;
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
  background: var(--dg-timeline-processing-fill);
  color: var(--dg-text-secondary);
}

.range--failure {
  justify-content: space-between;
  gap: 10px;
  border: 1px solid color-mix(in srgb, var(--dg-danger) 34%, transparent);
  background: var(--dg-danger-fill);
  color: var(--dg-danger);
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

.range__retry {
  flex: none;
  min-height: 24px;
  padding: 3px 8px;
  border: 1px solid currentColor;
  border-radius: 5px;
  color: var(--dg-danger);
  font-size: 10px;
}

.range__retry:disabled { cursor: default; opacity: 0.42; }
.range__retry:not(:disabled):hover { background: color-mix(in srgb, var(--dg-danger) 9%, transparent); }
.range__retry:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }

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

</style>
