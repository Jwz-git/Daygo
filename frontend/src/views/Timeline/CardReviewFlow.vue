<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import type { CardMediaFrameDTO, TimelineCardDTO, TimelineDayDTO } from '@/api/dto'
import { getCardMedia } from '@/api/media'
import { clearCardReview, saveCardReview } from '@/api/review'
import { categoryLabel } from '@/lib/categoryLabel'
import { useDurationFormat } from '@/lib/duration'

import CardVideoPlayer from '@/components/CardVideoPlayer.vue'
import { formatClockTime } from '@/lib/timeFormat'
import { safeCategoryColor } from './layout'
import { type ReviewTotals } from './review'

/*
 * Sequential card review (审阅卡片): step through the day's activity cards and
 * judge each one's focus level. Verdicts are statistics-only — they never
 * rewrite the card's category; the session totals feed the review panel and
 * the completion bar, and 撤销 unwinds the totals.
 */
const props = defineProps<{
  day: TimelineDayDTO
  cards: TimelineCardDTO[]
  timeZone: string
  /** Persisted verdict totals for the day, loaded by the parent. */
  initialTotals: ReviewTotals
}>()

const emit = defineEmits<{
  close: []
  judged: [cardID: number, removed: boolean]
  totals: [totals: ReviewTotals]
}>()

const { t, locale } = useI18n()

/* Snapshot the queue at open time: the parent's live queue shrinks with
   every judgment (it feeds the badge), and iterating a shrinking array would
   skip cards. */
const queue = ref<TimelineCardDTO[]>([...props.cards])

const index = ref(0)
const saving = ref(false)
/** Stack of judgments for 撤销, carrying the kind so the totals unwind. */
const history = ref<Array<{ card: TimelineCardDTO; kind: 'distraction' | 'neutral' | 'focus' }>>([])
const saveFailed = ref(false)
const focusMinutes = ref(props.initialTotals.focusMinutes)
const neutralMinutes = ref(props.initialTotals.neutralMinutes)
const distractionMinutes = ref(props.initialTotals.distractionMinutes)
const duration = useDurationFormat()

const current = computed(() => queue.value[index.value] ?? null)
const total = queue.value.length
const finished = computed(() => total > 0 && index.value >= total)
const progressLabel = computed(() =>
  t('timeline.review.progress', { current: Math.min(index.value + 1, total), total }),
)

/*
 * The verdict split (分心 / 中性 / 专注) — the same presentation the
 * inspector's review panel renders. All three blocks always draw: a
 * zero-minute verdict keeps a small stub instead of vanishing.
 */
const verdictSegments = computed(() => [
  { label: t('timeline.review.distraction'), minutes: distractionMinutes.value, color: '#ef8a7a' },
  { label: t('timeline.review.neutral'), minutes: neutralMinutes.value, color: '#e7e4ec' },
  { label: t('timeline.review.focus'), minutes: focusMinutes.value, color: '#35c3a2' },
])

const timeRange = computed(() => {
  if (current.value === null) return ''
  return `${formatClockTime(current.value.startTs, locale.value, props.timeZone)} – ${formatClockTime(current.value.endTs, locale.value, props.timeZone)}`
})

const color = computed(() => {
  if (current.value === null) return safeCategoryColor(undefined)
  const category = props.day.categories.find((entry) => entry.name === current.value?.category)
  return safeCategoryColor(category?.colorHex)
})

/* Frames for the current card's timespan; a failed listing keeps the
   placeholder (media is display data, never a blocker). */
const mediaFrames = ref<CardMediaFrameDTO[]>([])
const mediaCardID = ref<number | null>(null)

watch(current, async (card) => {
  if (card === null || card.id === mediaCardID.value) return
  mediaFrames.value = []
  mediaCardID.value = card.id
  try {
    const media = await getCardMedia(card.id)
    if (mediaCardID.value === card.id) mediaFrames.value = media.frames
  } catch {
    // Placeholder stays; no invented frames.
  }
}, { immediate: true })

/*
 * Every verdict is statistics-only: judgments never rewrite the card's
 * category — the timeline keeps the LLM's classification, and the session
 * totals feed the review panel and the completion bar.
 */
async function judge(kind: 'distraction' | 'neutral' | 'focus'): Promise<void> {
  const card = current.value
  if (card === null || saving.value) return
  saving.value = true
  try {
    await saveCardReview(card.id, kind)
  } catch {
    saveFailed.value = true
    saving.value = false
    return
  }
  history.value.push({ card, kind })
  if (kind === 'focus') focusMinutes.value += card.durationMinutes
  if (kind === 'neutral') neutralMinutes.value += card.durationMinutes
  if (kind === 'distraction') distractionMinutes.value += card.durationMinutes
  emit('totals', totalsSnapshot())
  emit('judged', card.id, false)
  index.value += 1
  saving.value = false
}

async function undo(): Promise<void> {
  const last = history.value.pop()
  if (last === undefined || saving.value) return
  saving.value = true
  try {
    await clearCardReview(last.card.id)
  } catch {
    history.value.push(last)
    saveFailed.value = true
    saving.value = false
    return
  }
  if (last.kind === 'focus') focusMinutes.value = Math.max(0, focusMinutes.value - last.card.durationMinutes)
  if (last.kind === 'neutral') neutralMinutes.value = Math.max(0, neutralMinutes.value - last.card.durationMinutes)
  if (last.kind === 'distraction') distractionMinutes.value = Math.max(0, distractionMinutes.value - last.card.durationMinutes)
  emit('totals', totalsSnapshot())
  emit('judged', last.card.id, true)
  index.value = Math.max(0, index.value - 1)
  saving.value = false
}

/** Snapshot of the session totals for the inspector's review panel. */
function totalsSnapshot(): ReviewTotals {
  return {
    distractionMinutes: distractionMinutes.value,
    neutralMinutes: neutralMinutes.value,
    focusMinutes: focusMinutes.value,
  }
}
</script>

<template>
  <div class="review" role="dialog" :aria-label="t('timeline.review.title')">
    <!-- Window-level close, clear of the card so nothing overlaps it. -->
    <button type="button" class="review__close" :aria-label="t('timeline.inspector.close')" @click="emit('close')">×</button>

    <div class="review__panel">
      <template v-if="current !== null">
        <div class="review__stage">
          <CardVideoPlayer
            :frames="mediaFrames"
            :title="current.title"
            :time-label="timeRange"
            :time-zone="props.timeZone"
          autoplay
          />
        </div>

        <div class="review__body">
          <h2 class="review__title">{{ current.title }}</h2>
          <div class="review__meta">
            <span class="review__category" :style="{ borderColor: color, color }">
              <i :style="{ background: color }"></i>{{ categoryLabel(current.category, t) }}
            </span>
            <span class="review__time">{{ timeRange }}</span>
          </div>
          <p v-if="current.summary !== ''" class="review__summary">{{ current.summary }}</p>
        </div>
        <span class="review__progress">{{ progressLabel }}</span>
    </template>

    <div v-else-if="finished" class="review__done">
      <h2>{{ t('timeline.review.doneTitle') }}</h2>
      <p class="review__done-body">{{ t('timeline.review.doneBody') }}</p>
      <div class="review__done-bar" aria-hidden="true">
        <span
          v-for="segment in verdictSegments.filter((entry) => entry.minutes > 0)"
          :key="segment.label"
          :style="{ flexGrow: segment.minutes, '--seg': segment.color }"
        ></span>
      </div>
      <div class="review__done-legend">
        <div v-for="segment in verdictSegments" :key="segment.label" class="review__done-stat">
          <span class="review__done-stat-label">
            <i :style="{ background: segment.color }"></i>{{ segment.label }}
          </span>
          <strong>{{ duration(segment.minutes) }}</strong>
        </div>
      </div>
      <button type="button" class="dg-button review__done-close" @click="emit('close')">
        {{ t('common.action.close') }}
      </button>
    </div>

    <div v-else class="review__empty">
      <p>{{ t('timeline.review.empty') }}</p>
      <button type="button" class="dg-button" @click="emit('close')">{{ t('common.action.close') }}</button>
    </div>
    </div>

    <template v-if="current !== null">
      <p class="review__hint">{{ t('timeline.review.hint') }}</p>
      <p v-if="saveFailed" class="review__error" role="alert">{{ t('timeline.review.saveFailed') }}</p>
      <div class="review__actions">
        <button type="button" class="review__judge" :disabled="saving || history.length === 0" @click="undo">
          <span class="review__judge-icon review__judge-icon--undo">↺</span>
          {{ t('timeline.review.undo') }}
        </button>
        <button type="button" class="review__judge" :disabled="saving" @click="judge('distraction')">
          <span class="review__judge-icon review__judge-icon--distraction">▶</span>
          {{ t('timeline.review.distraction') }}
        </button>
        <button type="button" class="review__judge" :disabled="saving" @click="judge('neutral')">
          <span class="review__judge-icon">▲</span>
          {{ t('timeline.review.neutral') }}
        </button>
        <button type="button" class="review__judge" :disabled="saving" @click="judge('focus')">
          <span class="review__judge-icon review__judge-icon--focus">▶</span>
          {{ t('timeline.review.focus') }}
        </button>
      </div>
    </template>
  </div>
</template>

<style scoped>
.review {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 18px;
  width: min(430px, calc(100vw - 64px));
  max-height: calc(100vh - 96px);
  padding-top: 40px;
}

/* Solid close on the window corner — no see-through circle on the scrim. */
.review__close {
  position: fixed;
  top: 16px;
  right: 16px;
  z-index: 30;
  display: grid;
  width: 32px;
  height: 32px;
  place-items: center;
  border: none;
  border-radius: 50%;
  background: var(--dg-accent);
  color: #ffffff;
  font-size: 17px;
  box-shadow: var(--dg-shadow-sm);
  cursor: pointer;
}

.review__close:hover { background: var(--dg-accent-strong); }

/* The white card; hint and judge buttons live on the scrim below it. */
.review__panel {
  position: relative;
  display: flex;
  flex-direction: column;
  width: 100%;
  overflow-y: auto;
  padding: 10px 12px 12px;
  border-radius: 14px;
  background: var(--dg-popover-fill, var(--dg-surface));
  box-shadow: var(--lg-shadow-dense, var(--dg-shadow-lg));
}

.review__close {
  position: absolute;
  top: 14px;
  right: 14px;
  display: grid;
  width: 30px;
  height: 30px;
  place-items: center;
  border: 1px solid color-mix(in srgb, var(--dg-danger) 45%, transparent);
  border-radius: 50%;
  background: transparent;
  color: var(--dg-danger);
  font-size: 16px;
  cursor: pointer;
}

.review__close:hover { background: color-mix(in srgb, var(--dg-danger) 10%, transparent); }

.review__stage { border-radius: 12px; overflow: hidden; }

.review__body { padding: 16px 2px 0; }

.review__title {
  margin: 0 0 12px;
  color: var(--dg-text-primary);
  font-size: 26px;
  font-weight: 700;
  line-height: 1.4;
}

.review__meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.review__category {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 12px;
  border: 1px solid;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
}

.review__category i { width: 7px; height: 7px; border-radius: 50%; background: currentColor; }

.review__time {
  padding: 4px 12px;
  border-radius: 999px;
  background: var(--dg-track-fill);
  color: var(--dg-text-secondary);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}

/* The 1 / 2 page marker, pinned to the card's bottom-right. */
.review__progress {
  align-self: flex-end;
  margin-top: 8px;
  color: var(--dg-text-muted);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}

.review__summary {
  margin: 12px 0 0;
  color: var(--dg-text-secondary);
  font-size: 13px;
  line-height: 1.6;
}

.review__empty {
  display: grid;
  gap: 14px;
  justify-items: center;
  padding: 40px 0 24px;
  color: var(--dg-text-secondary);
  font-size: 13px;
}

/* Completion screen (已全部处理完毕) */
.review__done {
  display: grid;
  justify-items: center;
  gap: 6px;
  padding: 36px 0 18px;
  text-align: center;
}

.review__done h2 {
  margin: 0 0 8px;
  color: var(--dg-text-primary);
  font-size: 28px;
  font-weight: 700;
}

.review__done-body {
  margin: 0;
  color: var(--dg-text-secondary);
  font-size: 13px;
  line-height: 1.7;
  max-width: 46ch;
}

/* Segmented verdict bar: separate rounded blocks with a gap, widths
   proportional to judged minutes. */
.review__done-bar {
  display: flex;
  gap: 8px;
  width: min(560px, 100%);
  height: 44px;
  margin: 18px 0 0;
}

.review__done-bar span {
  flex-basis: 56px;
  border-radius: 12px;
  background: linear-gradient(
    180deg,
    color-mix(in srgb, var(--seg) 72%, #ffffff),
    var(--seg) 55%,
    color-mix(in srgb, var(--seg) 86%, #000000)
  );
  box-shadow: 0 6px 14px rgba(45, 50, 80, 0.16);
}

.review__done-legend {
  display: flex;
  justify-content: center;
  gap: 28px;
  margin-top: 12px;
}

.review__done-stat {
  display: grid;
  justify-items: center;
  gap: 3px;
}

.review__done-stat-label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--dg-text-secondary);
  font-size: 12px;
}

.review__done-stat-label i {
  width: 16px;
  height: 10px;
  border-radius: 4px;
}

.review__done-stat strong {
  color: var(--dg-text-primary);
  font-size: 14px;
  font-weight: 650;
}

.review__done-close { margin-top: 16px; min-width: 96px; }

.review__hint {
  margin: 0;
  color: rgba(255, 255, 255, 0.85);
  font-size: 12px;
  text-align: center;
}

.review__actions {
  display: flex;
  justify-content: center;
  gap: 14px;
  padding: 0;
}

.review__judge {
  display: inline-flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  min-width: 64px;
  padding: 8px 10px;
  border: none;
  border-radius: 10px;
  background: transparent;
  color: rgba(255, 255, 255, 0.92);
  font-size: 12px;
  font-weight: 550;
  cursor: pointer;
  transition: background var(--dg-motion-fast) ease, transform var(--dg-motion-fast) var(--dg-ease-out);
}

.review__judge:hover:not(:disabled) { background: var(--dg-hover-fill); }
.review__judge:active:not(:disabled) { transform: scale(0.94); }
.review__judge:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }
.review__judge:disabled { opacity: 0.45; cursor: default; }

/* All four judge icons share one opaque, high-contrast surface. */
.review__judge-icon {
  display: grid;
  width: 34px;
  height: 34px;
  place-items: center;
  border-radius: 9px;
  background: var(--dg-accent);
  color: #ffffff;
  font-size: 13px;
}

.review__judge-icon--undo,
.review__judge-icon--distraction,
.review__judge-icon--focus { background: var(--dg-accent); }
</style>
