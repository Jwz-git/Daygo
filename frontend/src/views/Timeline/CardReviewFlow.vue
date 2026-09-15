<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import type { CardMediaFrameDTO, TimelineCardDTO, TimelineDayDTO } from '@/api/dto'
import { getCardMedia } from '@/api/media'
import { updateCardCategory } from '@/api/timeline'
import { categoryKey, categoryLabel } from '@/lib/categoryLabel'
import { useDurationFormat } from '@/lib/duration'

import CardVideoPlayer from '@/components/CardVideoPlayer.vue'
import { formatClockTime } from '@/lib/timeFormat'
import { safeCategoryColor } from './layout'

/*
 * Sequential card review (审阅卡片): step through the day's activity cards and
 * judge each one's focus level. 分心/专注 persist through the existing
 * UpdateCardCategory binding (LLM categories remain the default; a judgment
 * only reclassifies when the user explicitly makes one). 中性 records the
 * judgment without touching the LLM category. 撤销 pops the last judgment and
 * restores the previous category.
 */
const props = defineProps<{
  day: TimelineDayDTO
  cards: TimelineCardDTO[]
  timeZone: string
}>()

const emit = defineEmits<{
  close: []
  judged: [cardID: number, removed: boolean]
}>()

const { t, locale } = useI18n()

/* Snapshot the queue at open time: the parent's live queue shrinks with
   every judgment (it feeds the badge), and iterating a shrinking array would
   skip cards. */
const queue = ref<TimelineCardDTO[]>([...props.cards])

const index = ref(0)
const saving = ref(false)
const saveFailed = ref(false)
/** Stack of judgments for 撤销, carrying the kind so the totals unwind. */
const history = ref<Array<{ card: TimelineCardDTO; previousCategory: string; kind: 'distraction' | 'neutral' | 'focus' }>>([])
const focusMinutes = ref(0)
const neutralMinutes = ref(0)
const distractionMinutes = ref(0)
const duration = useDurationFormat()

const current = computed(() => queue.value[index.value] ?? null)
const total = queue.value.length
const finished = computed(() => total > 0 && index.value >= total)
const progressLabel = computed(() =>
  t('timeline.review.progress', { current: Math.min(index.value + 1, total), total }),
)

/*
 * The completion bar is the judged time itself, split by verdict: red for
 * distraction, grey for neutral, teal for focus (all focus → all green).
 */
const doneSegments = computed(() => {
  const parts = [
    { kind: 'distraction', minutes: distractionMinutes.value, color: '#ef8a7a' },
    { kind: 'neutral', minutes: neutralMinutes.value, color: '#c9c6d2' },
    { kind: 'focus', minutes: focusMinutes.value, color: '#35c3a2' },
  ].filter((segment) => segment.minutes > 0)
  return parts
})

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

async function applyCategory(card: TimelineCardDTO, category: string): Promise<boolean> {
  try {
    await updateCardCategory(card.id, category)
    return true
  } catch {
    return false
  }
}

/*
 * Judge targets resolve against the day's real category names (seeded as
 * "Focus Work" / "Distraction" — case matters to the backend's unknown-name
 * check). A judgment is only offered when that category exists.
 */
const distractionCategory = computed(() =>
  props.day.categories.find((category) => categoryKey(category.name) === 'distraction')?.name ?? null,
)
const focusCategory = computed(() =>
  props.day.categories.find((category) => categoryKey(category.name) === 'focus work')?.name ?? null,
)

async function judge(kind: 'distraction' | 'neutral' | 'focus'): Promise<void> {
  const card = current.value
  if (card === null || saving.value) return
  const target = kind === 'focus' ? focusCategory.value
    : kind === 'distraction' ? distractionCategory.value
    : null
  if (kind !== 'neutral' && target === null) {
    saveFailed.value = true
    return
  }
  saving.value = true
  saveFailed.value = false

  const succeeded = target === null || (await applyCategory(card, target))
  if (!succeeded) {
    saveFailed.value = true
    saving.value = false
    return
  }
  history.value.push({ card, previousCategory: card.category, kind })
  if (kind === 'focus') focusMinutes.value += card.durationMinutes
  if (kind === 'neutral') neutralMinutes.value += card.durationMinutes
  if (kind === 'distraction') distractionMinutes.value += card.durationMinutes
  emit('judged', card.id, false)
  index.value += 1
  saving.value = false
}

async function undo(): Promise<void> {
  const last = history.value.pop()
  if (last === undefined || saving.value) return
  saving.value = true
  saveFailed.value = false
  const succeeded = await applyCategory(last.card, last.previousCategory)
  if (!succeeded) {
    history.value.push(last)
    saveFailed.value = true
    saving.value = false
    return
  }
  emit('judged', last.card.id, true)
  if (last.kind === 'focus') focusMinutes.value = Math.max(0, focusMinutes.value - last.card.durationMinutes)
  if (last.kind === 'neutral') neutralMinutes.value = Math.max(0, neutralMinutes.value - last.card.durationMinutes)
  if (last.kind === 'distraction') distractionMinutes.value = Math.max(0, distractionMinutes.value - last.card.durationMinutes)
  index.value = Math.max(0, index.value - 1)
  saving.value = false
}
</script>

<template>
  <div class="review" role="dialog" :aria-label="t('timeline.review.title')">
    <div class="review__panel">
      <button type="button" class="review__close" :aria-label="t('timeline.inspector.close')" @click="emit('close')">×</button>

      <template v-if="current !== null">
        <div class="review__stage">
          <CardVideoPlayer
            :frames="mediaFrames"
            :title="current.title"
            :time-label="timeRange"
            :time-zone="props.timeZone"
          />
        </div>

        <div class="review__body">
          <h2 class="review__title dg-reading">{{ current.title }}</h2>
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
          v-for="segment in doneSegments"
          :key="segment.kind"
          :style="{ flexGrow: segment.minutes, background: segment.color }"
        ></span>
      </div>
      <p class="review__done-focus">
        <span>✦ {{ t('timeline.review.focus') }}</span>
        <strong>{{ duration(focusMinutes) }}</strong>
      </p>
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
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 18px;
  width: min(680px, calc(100vw - 64px));
  max-height: calc(100vh - 96px);
}

/* The white card; hint and judge buttons live on the scrim below it. */
.review__panel {
  position: relative;
  display: flex;
  flex-direction: column;
  width: 100%;
  overflow-y: auto;
  padding: 18px 22px 14px;
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
  font-size: 24px;
  font-weight: 600;
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

.review__done-bar {
  width: min(560px, 100%);
  height: 42px;
  margin: 18px 0 4px;
  border-radius: 8px;
  background: linear-gradient(100deg, #35c3a2, #4fd8b8);
}

.review__done-focus {
  display: flex;
  flex-direction: column;
  gap: 2px;
  margin: 10px 0 0;
}

.review__done-focus span { color: var(--dg-text-secondary); font-size: 12px; }
.review__done-focus strong { color: var(--dg-text-primary); font-size: 16px; font-weight: 650; }

.review__done-close { margin-top: 16px; min-width: 96px; }

.review__hint {
  margin: 0;
  color: rgba(255, 255, 255, 0.85);
  font-size: 12px;
  text-align: center;
}

.review__error {
  margin: 0;
  color: #ffb3a8;
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
  color: var(--dg-text-secondary);
  font-size: 12px;
  font-weight: 550;
  cursor: pointer;
  transition: background var(--dg-motion-fast) ease, transform var(--dg-motion-fast) var(--dg-ease-out);
}

.review__judge:hover:not(:disabled) { background: var(--dg-hover-fill); }
.review__judge:active:not(:disabled) { transform: scale(0.94); }
.review__judge:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }
.review__judge:disabled { opacity: 0.45; cursor: default; }

.review__judge-icon {
  display: grid;
  width: 34px;
  height: 34px;
  place-items: center;
  border-radius: 9px;
  background: var(--dg-track-fill);
  color: var(--dg-text-primary);
  font-size: 13px;
}

.review__judge-icon--undo { color: var(--dg-accent-text); }
.review__judge-icon--distraction { background: color-mix(in srgb, var(--dg-danger) 14%, transparent); color: var(--dg-danger); }
.review__judge-icon--focus { background: color-mix(in srgb, var(--dg-success) 14%, transparent); color: var(--dg-success); }
</style>
