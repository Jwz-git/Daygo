<script setup lang="ts">
import {
  computed,
  onBeforeUnmount,
  onMounted,
  ref,
  watch,
  type CSSProperties,
} from 'vue'
import { useI18n } from 'vue-i18n'

import type { CardMediaFrameDTO, TimelineCardDTO, TimelineDayDTO } from '@/api/dto'
import { getCardMedia } from '@/api/media'
import { clearCardReview, saveCardReview } from '@/api/review'
import { appSiteValues } from '@/lib/appSiteIcon'
import { categoryLabel } from '@/lib/categoryLabel'
import { useDurationFormat } from '@/lib/duration'
import { formatClockTime } from '@/lib/timeFormat'

import AppSiteIcon from '@/components/AppSiteIcon.vue'
import CardVideoPlayer from '@/components/CardVideoPlayer.vue'
import { safeCategoryColor } from './layout'
import { type ReviewTotals } from './review'

/*
 * Sequential card review (审阅卡片): step through the day's activity cards and
 * judge each one's focus level with authentic Dayflow physics:
 * - Tinder-style stacked deck (active card on top, next card underneath).
 * - Directional exits: Distraction swipes left (◀), Focus swipes right (▶),
 *   Neutral swipes up (▲).
 * - Undo slides the previous card back in from the bottom (↺).
 * - Interactive pointer drag/swipe with live overlay rating feedback.
 * - Arrow key shortcuts (Left/Up/Right, Z/Backspace for Undo, Esc to close).
 * - Statistics-only: verdicts never rewrite the card's category on the timeline.
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
const nextCard = computed(() => queue.value[index.value + 1] ?? null)
/* Marks for the deck's two visible cards, resolved once each. */
const currentSites = computed(() => (current.value === null ? [] : cardSites(current.value)))
const nextSites = computed(() => (nextCard.value === null ? [] : cardSites(nextCard.value)))
const total = queue.value.length
const finished = computed(() => total > 0 && index.value >= total)

/*
 * The verdict split (分心 / 中性 / 专注) — the same presentation the
 * inspector's review panel renders. All three blocks always draw: a
 * zero-minute verdict keeps a small stub instead of vanishing.
 */
const verdictSegments = computed(() => [
  { label: t('timeline.review.distraction'), minutes: distractionMinutes.value, color: 'var(--dg-danger)' },
  { label: t('timeline.review.neutral'), minutes: neutralMinutes.value, color: '#e7e4ec' },
  { label: t('timeline.review.focus'), minutes: focusMinutes.value, color: '#35c3a2' },
])

function getTimeRange(card: TimelineCardDTO | null): string {
  if (card === null) return ''
  return `${formatClockTime(card.startTs, locale.value, props.timeZone)} – ${formatClockTime(card.endTs, locale.value, props.timeZone)}`
}

function getCategoryColor(catName: string | undefined): string {
  if (!catName) return safeCategoryColor(undefined)
  const category = props.day.categories.find((entry) => entry.name === catName)
  return safeCategoryColor(category?.colorHex)
}

/* App/site candidates for the mark beside the title. The review deck shows the
   same cards the track and the inspector do, so it carries the same mark:
   without it the deck is the one place a card loses the "what was on screen"
   cue. Only local rules run here — the icon resolver's network and
   installed-app lookups stay in the card component. */
function cardSites(card: TimelineCardDTO): string[] {
  return appSiteValues(card.appSites)
}

function getProgressLabel(cardIndex: number): string {
  return t('timeline.review.progress', { current: Math.min(cardIndex + 1, total), total })
}

function ratingTitle(rating: 'distraction' | 'neutral' | 'focus'): string {
  if (rating === 'distraction') return t('timeline.review.distraction')
  if (rating === 'neutral') return t('timeline.review.neutral')
  return t('timeline.review.focus')
}

function ratingIcon(rating: 'distraction' | 'neutral' | 'focus'): string {
  if (rating === 'distraction') return '◀'
  if (rating === 'neutral') return '▲'
  return '▶'
}

/* Cache media frames per card ID so the leaving card's video does not flash blank
   and the next card is already buffered underneath. */
const mediaFramesCache = ref<Record<number, CardMediaFrameDTO[]>>({})

async function fetchCardMedia(cardId: number): Promise<void> {
  if (mediaFramesCache.value[cardId] !== undefined) return
  try {
    const media = await getCardMedia(cardId)
    mediaFramesCache.value = {
      ...mediaFramesCache.value,
      [cardId]: media.frames,
    }
  } catch {
    mediaFramesCache.value = {
      ...mediaFramesCache.value,
      [cardId]: [],
    }
  }
}

watch(
  [() => current.value?.id, () => nextCard.value?.id],
  ([curId, nxtId]) => {
    if (curId) void fetchCardMedia(curId)
    if (nxtId) void fetchCardMedia(nxtId)
  },
  { immediate: true },
)

/*
 * Interactive Card Animation & Drag Mechanics:
 * - isAnimatingOut: true while card is flying offscreen
 * - exitDirection: 'distraction' (left) | 'neutral' (up) | 'focus' (right)
 * - isEnteringFromBottom / isEnteringBack: true during Undo entrance
 * - isDragging: pointer press & move tracking
 */
const isAnimatingOut = ref(false)
const exitDirection = ref<'distraction' | 'neutral' | 'focus' | null>(null)
const activeOverlayRating = ref<'distraction' | 'neutral' | 'focus' | null>(null)

const isEnteringFromBottom = ref(false)
const isEnteringBack = ref(false)

const isDragging = ref(false)
const dragOffset = ref({ x: 0, y: 0 })
const dragStart = { x: 0, y: 0 }

const activeCardStyle = computed<CSSProperties>(() => {
  if (isDragging.value) {
    const rot = dragOffset.value.x / 20
    return {
      transform: `translate(${dragOffset.value.x}px, ${dragOffset.value.y}px) rotate(${rot}deg)`,
      transition: 'none',
    }
  }

  if (isAnimatingOut.value && exitDirection.value) {
    let tx = 0
    let ty = 0
    let rot = 0
    if (exitDirection.value === 'distraction') {
      tx = -580
      rot = -16
    } else if (exitDirection.value === 'focus') {
      tx = 580
      rot = 16
    } else if (exitDirection.value === 'neutral') {
      ty = -580
      rot = 0
    }
    return {
      transform: `translate(${tx}px, ${ty}px) rotate(${rot}deg)`,
      opacity: 0,
      transition: 'transform 260ms cubic-bezier(0.2, 0.9, 0.4, 1), opacity 240ms ease-out',
      pointerEvents: 'none',
    }
  }

  if (isEnteringFromBottom.value) {
    return {
      transform: 'translateY(120%)',
      opacity: 0,
      transition: 'none',
    }
  }

  if (isEnteringBack.value) {
    return {
      transform: 'translateY(0)',
      opacity: 1,
      transition: 'transform 320ms var(--dg-ease-glide), opacity 260ms ease-out',
    }
  }

  return {
    transform: 'translate(0, 0) rotate(0deg)',
    opacity: 1,
    transition: 'transform 240ms cubic-bezier(0.2, 0.9, 0.4, 1), opacity 200ms ease',
  }
})

const underCardStyle = computed<CSSProperties>(() => {
  if (isAnimatingOut.value) {
    return {
      transform: 'scale(1) translateY(0)',
      opacity: 1,
      filter: 'brightness(1)',
      transition: 'transform 260ms cubic-bezier(0.2, 0.9, 0.4, 1), opacity 260ms ease, filter 260ms ease',
      pointerEvents: 'none',
    }
  }

  if (isDragging.value) {
    const dragDistance = Math.hypot(dragOffset.value.x, dragOffset.value.y)
    const progress = Math.min(1, dragDistance / 140)
    const scale = 0.96 + 0.04 * progress
    const ty = 8 - 8 * progress
    const op = 0.85 + 0.15 * progress
    const bri = 0.96 + 0.04 * progress
    return {
      transform: `scale(${scale}) translateY(${ty}px)`,
      opacity: op,
      filter: `brightness(${bri})`,
      transition: 'none',
      pointerEvents: 'none',
    }
  }

  return {
    transform: 'scale(0.96) translateY(8px)',
    opacity: 0.85,
    filter: 'brightness(0.96)',
    transition: 'transform 240ms cubic-bezier(0.2, 0.9, 0.4, 1), opacity 200ms ease, filter 200ms ease',
    pointerEvents: 'none',
  }
})

function onPointerDown(e: PointerEvent): void {
  if (isAnimatingOut.value || isEnteringFromBottom.value || isEnteringBack.value) return
  if (e.button !== 0) return
  const target = e.target as HTMLElement | null
  if (!target) return
  if (target.closest('button') || target.closest('.player__scrubber')) {
    return
  }

  isDragging.value = true
  dragStart.x = e.clientX
  dragStart.y = e.clientY
  dragOffset.value = { x: 0, y: 0 }
  window.addEventListener('pointermove', onPointerMove)
  window.addEventListener('pointerup', onPointerUp)
  window.addEventListener('pointercancel', onPointerUp)
}

function onPointerMove(e: PointerEvent): void {
  if (!isDragging.value) return
  const dx = e.clientX - dragStart.x
  const dy = e.clientY - dragStart.y
  dragOffset.value = { x: dx, y: dy }

  if (dx < -40) {
    activeOverlayRating.value = 'distraction'
  } else if (dx > 40) {
    activeOverlayRating.value = 'focus'
  } else if (dy < -40 && Math.abs(dx) < 30) {
    activeOverlayRating.value = 'neutral'
  } else {
    activeOverlayRating.value = null
  }
}

function onPointerUp(): void {
  if (!isDragging.value) return
  isDragging.value = false
  window.removeEventListener('pointermove', onPointerMove)
  window.removeEventListener('pointerup', onPointerUp)
  window.removeEventListener('pointercancel', onPointerUp)

  const dx = dragOffset.value.x
  const dy = dragOffset.value.y

  if (dx < -90) {
    void judge('distraction')
  } else if (dx > 90) {
    void judge('focus')
  } else if (dy < -80 && Math.abs(dx) < 60) {
    void judge('neutral')
  } else {
    activeOverlayRating.value = null
    dragOffset.value = { x: 0, y: 0 }
  }
}

/*
 * Every verdict is statistics-only: judgments never rewrite the card's
 * category — the timeline keeps the LLM's classification, and the session
 * totals feed the review panel and the completion bar.
 */
async function judge(kind: 'distraction' | 'neutral' | 'focus'): Promise<void> {
  const card = current.value
  if (card === null || saving.value || isAnimatingOut.value) return
  saving.value = true
  isAnimatingOut.value = true
  exitDirection.value = kind
  activeOverlayRating.value = kind

  const savePromise = saveCardReview(card.id, kind)
    .then(() => {
      history.value.push({ card, kind })
      if (kind === 'focus') focusMinutes.value += card.durationMinutes
      if (kind === 'neutral') neutralMinutes.value += card.durationMinutes
      if (kind === 'distraction') distractionMinutes.value += card.durationMinutes
      emit('totals', totalsSnapshot())
      emit('judged', card.id, false)
    })
    .catch(() => {
      saveFailed.value = true
    })

  setTimeout(async () => {
    await savePromise
    if (saveFailed.value) {
      isAnimatingOut.value = false
      exitDirection.value = null
      activeOverlayRating.value = null
      saving.value = false
      return
    }
    index.value += 1
    isAnimatingOut.value = false
    exitDirection.value = null
    activeOverlayRating.value = null
    dragOffset.value = { x: 0, y: 0 }
    saving.value = false
  }, 260)
}

async function undo(): Promise<void> {
  const last = history.value.pop()
  if (last === undefined || saving.value || isAnimatingOut.value) return
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

  isEnteringFromBottom.value = true
  requestAnimationFrame(() => {
    requestAnimationFrame(() => {
      isEnteringFromBottom.value = false
      isEnteringBack.value = true
      setTimeout(() => {
        isEnteringBack.value = false
      }, 340)
    })
  })
}

function handleKeydown(e: KeyboardEvent): void {
  const target = e.target as HTMLElement | null
  const tag = target?.tagName
  if (tag === 'INPUT' || tag === 'TEXTAREA') return

  if (e.key === 'ArrowLeft') {
    e.preventDefault()
    void judge('distraction')
  } else if (e.key === 'ArrowRight') {
    e.preventDefault()
    void judge('focus')
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    void judge('neutral')
  } else if (e.key === 'z' || e.key === 'Z' || e.key === 'Backspace' || e.key === 'ArrowDown') {
    e.preventDefault()
    void undo()
  } else if (e.key === 'Escape') {
    e.preventDefault()
    emit('close')
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleKeydown)
  window.removeEventListener('pointermove', onPointerMove)
  window.removeEventListener('pointerup', onPointerUp)
  window.removeEventListener('pointercancel', onPointerUp)
})

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

    <div class="review__stack">
      <!-- Underneath card (next in queue) -->
      <div
        v-if="nextCard !== null"
        class="review__card review__card--under"
        :style="underCardStyle"
      >
        <div class="review__stage">
          <CardVideoPlayer
            :frames="mediaFramesCache[nextCard.id] ?? []"
            :title="nextCard.title"
            :time-label="getTimeRange(nextCard)"
            :time-zone="props.timeZone"
            :autoplay="false"
          />
        </div>

        <div class="review__body">
          <div class="review__head">
            <AppSiteIcon
              v-if="nextSites.length > 0"
              :sites="nextSites"
              :accent="getCategoryColor(nextCard.category)"
              :size="30"
            />
            <h2 class="review__title">{{ nextCard.title }}</h2>
          </div>
          <div class="review__meta">
            <span class="review__category" :style="{ borderColor: getCategoryColor(nextCard.category), color: getCategoryColor(nextCard.category) }">
              <i :style="{ background: getCategoryColor(nextCard.category) }"></i>{{ categoryLabel(nextCard.category, t) }}
            </span>
            <span class="review__time">{{ getTimeRange(nextCard) }}</span>
          </div>
          <p v-if="nextCard.summary !== ''" class="review__summary">{{ nextCard.summary }}</p>
        </div>
        <span class="review__progress">{{ getProgressLabel(index + 1) }}</span>
      </div>

      <!-- Active top card -->
      <div
        v-if="current !== null"
        class="review__card review__card--active"
        :style="activeCardStyle"
        @pointerdown="onPointerDown"
      >
        <div class="review__stage">
          <CardVideoPlayer
            :frames="mediaFramesCache[current.id] ?? []"
            :title="current.title"
            :time-label="getTimeRange(current)"
            :time-zone="props.timeZone"
            autoplay
          />
        </div>

        <div class="review__body">
          <div class="review__head">
            <AppSiteIcon
              v-if="currentSites.length > 0"
              :sites="currentSites"
              :accent="getCategoryColor(current.category)"
              :size="30"
            />
            <h2 class="review__title">{{ current.title }}</h2>
          </div>
          <div class="review__meta">
            <span class="review__category" :style="{ borderColor: getCategoryColor(current.category), color: getCategoryColor(current.category) }">
              <i :style="{ background: getCategoryColor(current.category) }"></i>{{ categoryLabel(current.category, t) }}
            </span>
            <span class="review__time">{{ getTimeRange(current) }}</span>
          </div>
          <p v-if="current.summary !== ''" class="review__summary">{{ current.summary }}</p>
        </div>
        <span class="review__progress">{{ getProgressLabel(index) }}</span>

        <!-- Rating overlay badge -->
        <div
          v-if="activeOverlayRating"
          class="review__badge"
          :class="`review__badge--${activeOverlayRating}`"
        >
          <div class="review__badge-content">
            <span class="review__badge-icon">{{ ratingIcon(activeOverlayRating) }}</span>
            <span class="review__badge-title">{{ ratingTitle(activeOverlayRating) }}</span>
          </div>
        </div>
      </div>

      <!-- Completion screen (已全部处理完毕) -->
      <div v-else-if="finished" class="review__card review__done">
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

      <!-- Empty screen -->
      <div v-else class="review__card review__empty">
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
        <button type="button" class="review__judge" :disabled="saving || isAnimatingOut" @click="judge('distraction')">
          <span class="review__judge-icon review__judge-icon--distraction">◀</span>
          {{ t('timeline.review.distraction') }}
        </button>
        <button type="button" class="review__judge" :disabled="saving || isAnimatingOut" @click="judge('neutral')">
          <span class="review__judge-icon review__judge-icon--neutral">▲</span>
          {{ t('timeline.review.neutral') }}
        </button>
        <button type="button" class="review__judge" :disabled="saving || isAnimatingOut" @click="judge('focus')">
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
  transition: background var(--dg-motion-fast) ease;
}

.review__close:hover { background: var(--dg-accent-strong); }

/* Stacked card container: allows the active card to fly out without clipping. */
.review__stack {
  position: relative;
  display: flex;
  justify-content: center;
  width: 100%;
}

/* The card surface: white card with elevation shadow and border radius. */
.review__card {
  position: relative;
  display: flex;
  flex-direction: column;
  width: 100%;
  max-height: calc(100vh - 220px);
  overflow-y: auto;
  padding: 10px 12px 12px;
  border-radius: 14px;
  background: var(--dg-popover-fill, var(--dg-surface));
  box-shadow: var(--lg-shadow-dense, var(--dg-shadow-lg));
  will-change: transform, opacity;
  user-select: none;
  -webkit-user-select: none;
  touch-action: none;
}

.review__card--active {
  z-index: 2;
  cursor: grab;
}

.review__card--active:active {
  cursor: grabbing;
}

.review__card--under {
  position: absolute;
  inset: 0;
  z-index: 1;
  pointer-events: none;
  overflow: hidden;
}

/* Rating overlay badge displayed while swiping or animating out */
.review__badge {
  position: absolute;
  inset: 0;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 14px;
  backdrop-filter: blur(3px);
  pointer-events: none;
  animation: badge-pop 160ms cubic-bezier(0.2, 0.9, 0.4, 1) both;
}

@keyframes badge-pop {
  from {
    opacity: 0;
    transform: scale(0.92);
  }
  to {
    opacity: 1;
    transform: scale(1);
  }
}

.review__badge--distraction {
  background: color-mix(in srgb, var(--dg-danger) 88%, transparent);
  color: #ffffff;
}

.review__badge--neutral {
  background: rgba(221, 215, 232, 0.92);
  color: #333333;
}

.review__badge--focus {
  background: rgba(53, 195, 162, 0.9);
  color: #ffffff;
}

.review__badge-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}

.review__badge-icon {
  font-size: 38px;
  line-height: 1;
}

.review__badge-title {
  font-size: 22px;
  font-weight: 750;
  letter-spacing: -0.01em;
}

.review__stage { border-radius: 12px; overflow: hidden; }

.review__body { padding: 16px 2px 0; }

/* Mark + title on one line. min-width: 0 lets a long title wrap beside the
   mark instead of overflowing; the mark itself is flex: none. */
.review__head {
  display: flex;
  align-items: center;
  gap: 14px;
  margin: 0 0 12px;
}

.review__title {
  min-width: 0;
  margin: 0;
  color: var(--dg-text-primary);
  font-size: 30px;
  font-weight: 800;
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
  font-size: 13px;
  font-weight: 650;
}

.review__category i { width: 7px; height: 7px; border-radius: 50%; background: currentColor; }

.review__time {
  padding: 4px 12px;
  border-radius: 999px;
  background: var(--dg-track-fill);
  color: var(--dg-text-secondary);
  font-size: 13px;
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
  color: var(--dg-text-primary);
  font-size: 15px;
  line-height: 1.65;
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
  padding: 36px 16px 24px;
  text-align: center;
  animation: done-enter 300ms var(--dg-ease-glide) both;
}

@keyframes done-enter {
  from {
    opacity: 0;
    transform: scale(0.95);
  }
  to {
    opacity: 1;
    transform: scale(1);
  }
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
.review__judge-icon--neutral,
.review__judge-icon--focus { background: var(--dg-accent); }

@media (prefers-reduced-motion: reduce) {
  .review__card,
  .review__badge,
  .review__done {
    transition: none !important;
    animation: none !important;
  }
}
</style>
