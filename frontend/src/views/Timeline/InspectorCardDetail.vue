<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import type { CardMediaFrameDTO, TimelineCardDTO, TimelineDayDTO } from '@/api/dto'
import type { TimelineActionAvailability } from '@/api/timeline'
import { getCardMedia } from '@/api/media'
import { clearCardRating, clearCardReview, getCardRating, getCardVerdict, saveCardRating, saveCardReview } from '@/api/review'
import type { ReviewVerdict, SummaryRating } from './review'
import AppSiteIcon from '@/components/AppSiteIcon.vue'
import CardVideoPlayer from '@/components/CardVideoPlayer.vue'
import { appSiteValues } from '@/lib/appSiteIcon'
import { categoryLabel } from '@/lib/categoryLabel'
import { useDurationFormat } from '@/lib/duration'
import { formatClockTime } from '@/lib/timeFormat'
import type { TimelineAction } from '@/stores/timeline'

import { safeCategoryColor } from './layout'

/*
 * The inspector's card pane: the selected card's summary, apps, activity
 * points, distractions and media. Each editable field (title, category,
 * summary) carries its own pencil affordance and switches to an inline
 * editor; the action row keeps edit-less delete with confirm.
 */
const props = defineProps<{
  day: TimelineDayDTO
  timeZone: string
  card: TimelineCardDTO
  canWrite: boolean
  actions: TimelineActionAvailability
  pendingAction: TimelineAction | null
  actionFailed: boolean
}>()

const emit = defineEmits<{
  close: []
  saveEdits: [cardID: number, edits: { title?: string; category?: string; summary?: string; detailedSummary?: string }]
  delete: [cardID: number]
  reprocess: [cardID: number]
  verdictChanged: []
}>()

const { t, locale } = useI18n()
const duration = useDurationFormat()
const localizedCategory = computed(() => categoryLabel(props.card.category, t))
const timeRange = computed(() => {
  return `${formatClockTime(props.card.startTs, locale.value, props.timeZone)} – ${formatClockTime(props.card.endTs, locale.value, props.timeZone)}`
})

/* Which field is open in its inline editor; null shows read-only rows. */
type Field = 'title' | 'category' | 'summary' | 'detailedSummary'
const editingField = ref<Field | null>(null)
const confirmingDelete = ref(false)
const draft = ref('')

/* A busy card's activity points run to dozens of rows; unfolded they would push
   the media and action rows off the pane, so the list starts folded. */

const titleInput = ref<HTMLTextAreaElement | null>(null)
const summaryInput = ref<HTMLTextAreaElement | null>(null)
const detailedInput = ref<HTMLTextAreaElement | null>(null)
const categoryInput = ref<HTMLSelectElement | null>(null)

const selectedColor = computed(() => {
  const category = props.day.categories.find((entry) => entry.name === props.card.category)
  return safeCategoryColor(category?.colorHex)
})

const displayedAppSites = computed(() => appSiteValues(props.card.appSites ?? null))

const videoURLs = computed(() =>
  [...new Set([props.card.videoSummaryUrl, ...props.card.otherVideoSummaryUrls])]
    .filter((url): url is string => typeof url === 'string' && url.trim() !== ''),
)

/*
 * The frames the recorder stored inside the card's timespan. Media is display
 * data: a failed listing leaves the placeholder instead of blocking the pane.
 */
const mediaFrames = ref<CardMediaFrameDTO[]>([])

watch(
  () => props.card.id,
  async (cardID) => {
    mediaFrames.value = []
    try {
      const media = await getCardMedia(cardID)
      if (props.card.id === cardID) mediaFrames.value = media.frames
    } catch {
      // Placeholder stays; no invented frames.
    }
  },
  { immediate: true },
)

/*
 * The card's focus verdict, re-review outside the sequential flow: read back
 * per card (day totals track the day-view day, which differs from the selected
 * card in week mode). Verdicts are statistics only — saving never rewrites the
 * card's category. A change re-fetches the day totals in the parent so the
 * "你的回顾" panel and the review queue stay in sync.
 */
const VERDICTS: readonly ReviewVerdict[] = ['distraction', 'neutral', 'focus']
const VERDICT_COLORS: Record<ReviewVerdict, string> = {
  distraction: 'var(--dg-danger)',
  neutral: '#e7e4ec',
  focus: '#35c3a2',
}
const verdict = ref<ReviewVerdict | null>(null)
const verdictSaving = ref(false)
const verdictFailed = ref(false)
const verdictEditable = computed(() => props.canWrite && props.pendingAction === null)

watch(
  () => props.card.id,
  async (cardID) => {
    verdict.value = null
    verdictFailed.value = false
    try {
      const current = await getCardVerdict(cardID)
      if (props.card.id === cardID) verdict.value = current
    } catch {
      // No verdict shown; the buttons still let the user set one.
    }
  },
  { immediate: true },
)

async function setVerdict(next: ReviewVerdict): Promise<void> {
  if (!verdictEditable.value || verdictSaving.value) return
  // Tapping the active verdict again clears it, mirroring 撤销 in the flow.
  const clearing = verdict.value === next
  verdictSaving.value = true
  verdictFailed.value = false
  const cardID = props.card.id
  try {
    if (clearing) {
      await clearCardReview(cardID)
      if (props.card.id === cardID) verdict.value = null
    } else {
      await saveCardReview(cardID, next)
      if (props.card.id === cardID) verdict.value = next
    }
    emit('verdictChanged')
  } catch {
    verdictFailed.value = true
  } finally {
    verdictSaving.value = false
  }
}

/*
 * The card's summary rating: thumbs up/down on the AI-written summary text.
 * Read back per card like the verdict, and tapping the active thumb again
 * clears it (撤销). Ratings are feedback only — saving never rewrites the
 * summary, so nothing outside this pane needs re-fetching.
 */
const RATINGS: readonly SummaryRating[] = ['up', 'down']
const RATING_GLYPHS: Record<SummaryRating, string> = {
  up: 'M5 7.5V13m0-5.5L7.8 3c.9 0 1.5.7 1.4 1.6L9 7h3.4c.9 0 1.5.8 1.3 1.6l-.9 3.6c-.1.5-.6.9-1.2.9H5M5 7.5H2.5V13H5',
  down: 'M5 8.5V3m0 5.5L7.8 13c.9 0 1.5-.7 1.4-1.6L9 9h3.4c.9 0 1.5-.8 1.3-1.6l-.9-3.6C12.7 3.9 12.2 3.5 11.6 3.5H5M5 8.5H2.5V3H5',
}
const RATING_COLORS: Record<SummaryRating, string> = {
  // The pane's existing good/bad language: teal for "this was right", danger
  // for "this was wrong". A thumbs-down is negative feedback, not an error.
  up: '#35c3a2',
  down: 'var(--dg-danger)',
}
const rating = ref<SummaryRating | null>(null)
const ratingSaving = ref(false)
const ratingFailed = ref(false)
const ratingEditable = computed(() => props.canWrite && props.pendingAction === null)

function ratingLabel(option: SummaryRating): string {
  return option === 'up' ? t('timeline.inspector.ratingUp') : t('timeline.inspector.ratingDown')
}

watch(
  () => props.card.id,
  async (cardID) => {
    rating.value = null
    ratingFailed.value = false
    try {
      const current = await getCardRating(cardID)
      if (props.card.id === cardID) rating.value = current
    } catch {
      // No rating shown; the thumbs still let the user set one.
    }
  },
  { immediate: true },
)

async function setRating(next: SummaryRating): Promise<void> {
  if (!ratingEditable.value || ratingSaving.value) return
  const clearing = rating.value === next
  ratingSaving.value = true
  ratingFailed.value = false
  const cardID = props.card.id
  try {
    if (clearing) {
      await clearCardRating(cardID)
      if (props.card.id === cardID) rating.value = null
    } else {
      await saveCardRating(cardID, next)
      if (props.card.id === cardID) rating.value = next
    }
  } catch {
    ratingFailed.value = true
  } finally {
    ratingSaving.value = false
  }
}

/* The category picker lists the user-editable category names; the built-ins
   (System / Idle) are pipeline-assigned and rejected by the backend, so they
   are never offered — even for a card currently sitting in one. A legacy
   built-in card snaps to the first real choice when edited. */
const categoryOptions = computed(() => {
  const names = props.day.categories
    .filter((category) => !category.isSystem)
    .map((category) => category.name)
  if (props.card.category !== '' && !names.includes(props.card.category)
    && !props.day.categories.some((category) => category.name === props.card.category && category.isSystem)) {
    names.unshift(props.card.category)
  }
  return names
})

function fieldEditable(field: Field): boolean {
  if (!props.canWrite) return false
  if (props.pendingAction !== null) return false
  return field === 'title' ? props.actions.updateTitle
    : field === 'category' ? props.actions.updateCategory
    : field === 'summary' ? props.actions.updateSummary
    : props.actions.updateDetailedSummary
}

/* The detailed summary is a chronological log, one paragraph per phase. */
const detailedParagraphs = computed(() =>
  (props.card.detailedSummary || '')
    .split(/\n+/)
    .map((paragraph) => paragraph.trim())
    .filter((paragraph) => paragraph !== ''),
)

async function beginEditing(field: Field): Promise<void> {
  if (!fieldEditable(field)) return
  editingField.value = field
  confirmingDelete.value = false
  draft.value = field === 'title' ? props.card.title
    : field === 'category' ? props.card.category
    : field === 'summary' ? props.card.summary
    : props.card.detailedSummary
  await nextTick()
  const input = field === 'title' ? titleInput.value
    : field === 'category' ? categoryInput.value
    : field === 'summary' ? summaryInput.value
    : detailedInput.value
  input?.focus()
  if (field === 'title') {
    growTitle()
    titleInput.value?.select()
  }
}

/* Keep the borderless title editor exactly as tall as its wrapped text so the
   heading matches the displayed <h2> and the pane below stays put. */
function growTitle(): void {
  const el = titleInput.value
  if (el === null) return
  el.style.height = 'auto'
  el.style.height = `${el.scrollHeight}px`
}

function cancelEditing(): void {
  editingField.value = null
  draft.value = ''
}

function submitEditing(): void {
  const field = editingField.value
  if (field === null || props.pendingAction !== null) return
  if (field === 'title' && draft.value.trim() === '') return
  const edits = { [field]: draft.value }
  emit('saveEdits', props.card.id, edits)
  editingField.value = null
  draft.value = ''
}

function confirmDeletion(): void {
  emit('delete', props.card.id)
  confirmingDelete.value = false
}

/*
 * Regenerate one card: re-run the LLM on the batch that produced it. A card
 * with no originating batch (batchId null — a System fallback) cannot be
 * regenerated, so the control hides. It re-runs the whole batch (analysis is
 * per batch, not per card), so it is confirmed like delete before firing.
 */
const confirmingReprocess = ref(false)
const canReprocess = computed(
  () => props.canWrite && props.actions.reprocessCard && props.card.batchId !== null,
)

function confirmReprocess(): void {
  emit('reprocess', props.card.id)
  confirmingReprocess.value = false
}

watch(
  () => props.card.id,
  () => {
    editingField.value = null
    confirmingDelete.value = false
    confirmingReprocess.value = false
    draft.value = ''
  },
  { immediate: true },
)
</script>

<template>
  <header class="inspector__header">
    <div class="inspector__heading">
      <div v-if="editingField === 'category'" class="field-editor field-editor--category">
        <select
          ref="categoryInput"
          v-model="draft"
          class="dg-input"
          @keydown.esc="cancelEditing"
          @change="submitEditing"
          @blur="submitEditing"
        >
          <option
            v-for="name in categoryOptions"
            :key="name"
            :value="name"
          >
            {{ categoryLabel(name, t) }}
          </option>
        </select>
      </div>
      <p v-else class="inspector__eyebrow">
        <span>{{ localizedCategory }}</span>
        <button
          v-if="fieldEditable('category')"
          type="button"
          class="field-pencil"
          :aria-label="t('timeline.inspector.editCategory')"
          @click="beginEditing('category')"
        >
          <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M11.3 1.7a2.4 2.4 0 0 1 3.4 3.4l-8.3 8.3-4.3 1 1-4.3 8.2-8.4Z" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round"/></svg>
        </button>
      </p>

      <div v-if="editingField === 'title'" class="field-editor">
        <textarea
          ref="titleInput"
          v-model="draft"
          class="dg-input field-editor__title"
          rows="1"
          maxlength="160"
          @input="growTitle"
          @keydown.esc="cancelEditing"
          @keydown.enter.prevent="submitEditing"
          @blur="submitEditing"
        ></textarea>
      </div>
      <h2 v-else class="inspector__title inspector__title--card">
        <span class="title-text">{{ props.card.title }}</span>
        <button
          v-if="fieldEditable('title')"
          type="button"
          class="field-pencil"
          :aria-label="t('timeline.inspector.editTitle')"
          @click="beginEditing('title')"
        >
          <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M11.3 1.7a2.4 2.4 0 0 1 3.4 3.4l-8.3 8.3-4.3 1 1-4.3 8.2-8.4Z" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round"/></svg>
        </button>
      </h2>
    </div>

    <button
      type="button"
      class="inspector__close"
      :aria-label="t('timeline.inspector.close')"
      @click="emit('close')"
    >
      ×
    </button>
  </header>

  <div class="card-time">
    <span :style="{ background: selectedColor }"></span>
    {{ timeRange }} · {{ duration(props.card.durationMinutes) }}
  </div>

  <!-- Reference layout: the playback surface sits directly under the
       title/time row, before the summaries. -->
  <CardVideoPlayer
    class="card-player"
    :frames="mediaFrames"
    :title="props.card.title"
    :time-label="timeRange"
    :time-zone="timeZone"
  />

  <section class="inspector__section">
    <h3 class="section-heading">
      <span>{{ t('timeline.inspector.summary') }}</span>
      <button
        v-if="fieldEditable('summary')"
        type="button"
        class="field-pencil"
        :aria-label="t('timeline.inspector.editSummary')"
        @click="beginEditing('summary')"
      >
        <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M11.3 1.7a2.4 2.4 0 0 1 3.4 3.4l-8.3 8.3-4.3 1 1-4.3 8.2-8.4Z" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round"/></svg>
      </button>
    </h3>
    <div v-if="editingField === 'summary'" class="field-editor">
      <textarea
        ref="summaryInput"
        v-model="draft"
        class="dg-input field-editor__summary"
        rows="3"
        maxlength="240"
        @keydown.esc="cancelEditing"
        @blur="submitEditing"
      ></textarea>
      <p class="field-editor__hint">{{ t('timeline.inspector.summaryEditHint') }}</p>
    </div>
    <p v-else-if="props.card.summary" class="summary-text">{{ props.card.summary }}</p>
    <p v-else class="summary-text summary-text--empty">{{ t('timeline.inspector.noSummary') }}</p>
  </section>

  <section class="inspector__section">
    <h3 class="section-heading">
      <span>{{ t('timeline.inspector.detailedSummary') }}</span>
      <button
        v-if="fieldEditable('detailedSummary')"
        type="button"
        class="field-pencil"
        :aria-label="t('timeline.inspector.editDetailedSummary')"
        @click="beginEditing('detailedSummary')"
      >
        <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M11.3 1.7a2.4 2.4 0 0 1 3.4 3.4l-8.3 8.3-4.3 1 1-4.3 8.2-8.4Z" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round"/></svg>
      </button>
    </h3>
    <div v-if="editingField === 'detailedSummary'" class="field-editor">
      <textarea
        ref="detailedInput"
        v-model="draft"
        class="dg-input field-editor__summary"
        rows="8"
        maxlength="2000"
        @keydown.esc="cancelEditing"
        @blur="submitEditing"
      ></textarea>
      <p class="field-editor__hint">{{ t('timeline.inspector.detailedSummaryEditHint') }}</p>
    </div>
    <template v-else-if="detailedParagraphs.length > 0">
      <p
        v-for="(paragraph, index) in detailedParagraphs"
        :key="index"
        class="summary-paragraph"
      >{{ paragraph }}</p>
    </template>
    <p v-else class="summary-text summary-text--empty">{{ t('timeline.inspector.noSummary') }}</p>
  </section>

  <section v-if="displayedAppSites.length > 0" class="inspector__section">
    <h3>{{ t('timeline.inspector.apps') }}</h3>
    <ul class="app-sites">
      <li v-for="site in displayedAppSites" :key="site.toLocaleLowerCase('en-US')">
        <AppSiteIcon :site="site" :accent="selectedColor" :size="20" />
        <span>{{ site }}</span>
      </li>
    </ul>
  </section>

  <section v-if="props.card.distractions.length > 0" class="inspector__section">
    <h3>{{ t('timeline.inspector.distractions') }}</h3>
    <article
      v-for="distraction in props.card.distractions"
      :key="distraction.id"
      class="distraction"
    >
      <span>{{ distraction.startTime }} – {{ distraction.endTime }}</span>
      <strong>{{ distraction.title }}</strong>
    </article>
  </section>

  <section class="inspector__section inspector__section--frames">
    <p v-if="mediaFrames.length === 0 && videoURLs.length === 0">{{ t('timeline.inspector.framesUnavailable') }}</p>
    <template v-if="videoURLs.length > 0">
      <video
        v-for="url in videoURLs"
        :key="url"
        controls
        preload="metadata"
        :src="url"
        :aria-label="t('timeline.inspector.mediaLabel', { count: videoURLs.indexOf(url) + 1 })"
      ></video>
    </template>
    <p v-else-if="mediaFrames.length > 0" class="frames-note">
      {{ t('timeline.inspector.frameCount', { count: mediaFrames.length }) }}
    </p>
  </section>

  <!-- Focus verdict: re-review one card outside the sequential flow. The
       active verdict is highlighted; tapping it again clears it. -->
  <section class="inspector__section verdict">
    <h3>{{ t('timeline.inspector.verdictTitle') }}</h3>
    <div class="verdict__options" role="group" :aria-label="t('timeline.inspector.verdictTitle')">
      <button
        v-for="option in VERDICTS"
        :key="option"
        type="button"
        class="verdict__option"
        :class="{ 'is-active': verdict === option }"
        :style="{ '--verdict': VERDICT_COLORS[option] }"
        :disabled="!verdictEditable || verdictSaving"
        :aria-pressed="verdict === option"
        @click="setVerdict(option)"
      >
        <i></i>{{ t(`timeline.review.${option}`) }}
      </button>
    </div>
    <p v-if="verdictFailed" class="inspector__error" role="alert">{{ t('timeline.inspector.verdictSaveFailed') }}</p>
    <p v-else-if="!props.canWrite" class="verdict__hint">{{ t('timeline.inspector.verdictUnavailable') }}</p>
    <p v-else class="verdict__hint">{{ t('timeline.inspector.verdictHint') }}</p>
  </section>

  <!-- Summary rating: thumbs up/down on the AI-written summary text. Feedback
       only — a rating never rewrites the summary. Tapping the active thumb
       again clears it, mirroring 撤销 in the verdict row above. -->
  <section class="rating">
    <div class="rating-row">
      <span>{{ t('timeline.inspector.rating') }}</span>
      <button
        v-for="option in RATINGS"
        :key="option"
        type="button"
        class="rating-row__thumb"
        :class="{ 'is-active': rating === option }"
        :style="{ '--rating': RATING_COLORS[option] }"
        :disabled="!ratingEditable || ratingSaving"
        :aria-pressed="rating === option"
        :aria-label="ratingLabel(option)"
        :title="ratingLabel(option)"
        @click="setRating(option)"
      >
        <svg viewBox="0 0 16 16" aria-hidden="true">
          <path :d="RATING_GLYPHS[option]" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round" />
        </svg>
      </button>
    </div>
    <p v-if="ratingFailed" class="inspector__error" role="alert">{{ t('timeline.inspector.ratingSaveFailed') }}</p>
    <p v-else-if="!props.canWrite" class="rating__hint">{{ t('timeline.inspector.ratingUnavailable') }}</p>
  </section>

  <p v-if="props.actionFailed" class="inspector__error" role="alert">
    {{ t('timeline.inspector.actionFailed') }}
  </p>

  <div class="inspector__actions">
    <!-- Delete confirm takes over the row. -->
    <template v-if="confirmingDelete">
      <span class="inspector__confirm">{{ t('timeline.inspector.deleteConfirm') }}</span>
      <button type="button" class="dg-button" @click="confirmingDelete = false">
        {{ t('common.action.cancel') }}
      </button>
      <button
        type="button"
        class="dg-button inspector__delete"
        :disabled="props.pendingAction !== null"
        @click="confirmDeletion"
      >
        {{ t('common.action.delete') }}
      </button>
    </template>
    <!-- Regenerate confirm takes over the row: re-running the LLM overwrites
         the batch's cards, so it is confirmed like delete before firing. -->
    <template v-else-if="confirmingReprocess">
      <span class="inspector__confirm">{{ t('timeline.reprocess.cardConfirm') }}</span>
      <button type="button" class="dg-button" @click="confirmingReprocess = false">
        {{ t('common.action.cancel') }}
      </button>
      <button
        type="button"
        class="dg-button inspector__regenerate"
        :disabled="props.pendingAction !== null"
        @click="confirmReprocess"
      >
        {{ t('timeline.reprocess.cardConfirmYes') }}
      </button>
    </template>
    <!-- Default row: regenerate beside delete. -->
    <template v-else>
      <button
        v-if="props.card.batchId !== null"
        type="button"
        class="dg-button inspector__regenerate"
        :disabled="!canReprocess || props.pendingAction !== null"
        :title="canReprocess ? t('timeline.reprocess.card') : t('timeline.reprocess.cardUnavailable')"
        @click="confirmingReprocess = true"
      >
        <svg viewBox="0 0 16 16" aria-hidden="true" class="reprocess-icon"><path d="M13.65 2.35A8 8 0 1 0 16 8" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/><path d="M11 2l3 0 0 3" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>
        {{ props.pendingAction === 'reprocess-card' ? t('timeline.reprocess.cardRunning') : t('timeline.reprocess.card') }}
      </button>
      <button
        type="button"
        class="dg-button inspector__delete"
        :disabled="!props.canWrite || !props.actions.deleteCard || props.pendingAction !== null"
        :title="props.actions.deleteCard ? t('common.action.delete') : t('timeline.inspector.actionsUnavailable')"
        @click="confirmingDelete = true"
      >
        {{ t('common.action.delete') }}
      </button>
    </template>
    <span v-if="!props.canWrite" class="inspector__readonly">
      {{ t('timeline.inspector.readOnly') }}
    </span>
    <span
      v-else-if="!props.actions.updateTitle && !props.actions.updateCategory && !props.actions.updateSummary && !props.actions.updateDetailedSummary && !props.actions.deleteCard"
      class="inspector__readonly"
    >
      {{ t('timeline.inspector.actionsUnavailable') }}
    </span>
  </div>
</template>

<style scoped>
.inspector__heading { min-width: 0; }

/* The pencil floats in the top-right corner rather than taking a flex slot, so
   the title text runs the full column width — the editor then matches at full
   width with no reserved right inset. */
.inspector__title--card {
  position: relative;
  display: block;
}

/* The floating pencil overlaps the title text, so it carries the pane's own
   background to stay legible on top of any glyphs behind it. */
.inspector__title--card .field-pencil {
  position: absolute;
  top: 0;
  right: 0;
  z-index: 1;
  background: var(--dg-timeline-inspector-fill);
}

.title-text { overflow-wrap: anywhere; }

/* Pencil affordance beside an editable field. Faint until the row is
   hovered, so read-only scanning stays clean. */
.field-pencil {
  display: inline-grid;
  flex: none;
  width: 22px;
  height: 22px;
  padding: 0;
  place-items: center;
  border-radius: 6px;
  color: var(--dg-text-muted);
  opacity: 0;
  transition: opacity var(--dg-motion-base) ease;
}

.field-pencil svg { width: 12px; height: 12px; }

.inspector__header:hover .field-pencil,
.inspector__section:hover .field-pencil,
.field-pencil:focus-visible {
  opacity: 1;
}

.section-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

/* Disclosure header. The negative margin cancels the padding that gives the
   keyboard focus ring room, so the row still aligns with the other headings. */
.section-toggle {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin: -2px;
  padding: 2px;
  border-radius: 6px;
}

.section-toggle__chevron {
  flex: none;
  width: 11px;
  height: 11px;
  color: var(--dg-text-muted);
  transition: transform var(--dg-motion-base) var(--dg-ease-glide);
}

.section-toggle__chevron.is-open { transform: rotate(90deg); }

.section-toggle:hover .section-toggle__chevron { color: var(--dg-text-primary); }

.section-toggle:focus-visible {
  outline: none;
  box-shadow: 0 0 0 3px var(--dg-focus-ring);
}

.section-toggle__count {
  color: var(--dg-text-muted);
  font-variant-numeric: tabular-nums;
  font-weight: 550;
}

.field-editor { display: grid; gap: 6px; }

/* The title editor mirrors the displayed <h2>: same font and no input chrome or
   padding, so entering edit mode neither shrinks the text nor changes the box
   height. Its height tracks content via the grow handler, matching how the
   heading wraps, so the rows below never jump. */
.field-editor__title {
  min-height: 0;
  /* Full width, matching the display <h2> whose pencil is out of flow, so the
     text wraps into the same column and the line count — and therefore the
     height — matches exactly, with no empty strip on the right. */
  padding: 0;
  background: transparent;
  box-shadow: none;
  border-radius: 4px;
  font-size: 20px;
  font-weight: 700;
  line-height: 1.35;
  resize: none;
  overflow: hidden;
}

.field-editor__title:focus-visible { box-shadow: 0 0 0 3px var(--dg-focus-ring); }

/* The category picker's resting height is taller than its small label, so the
   eyebrow reserves that height — swapping in the select then leaves the title
   and everything under it in place. The label is vertically centred to sit at
   the same height as the select's own centred text, so it does not appear to
   jump between the display and edit states. */
.field-editor--category { max-width: 120px; min-height: 34px; margin-bottom: 5px; }

.inspector__heading .inspector__eyebrow {
  display: flex;
  align-items: center;
  gap: 4px;
  min-height: 34px;
}

/* The category label reads as the card's type, so it carries more weight than
   the shared eyebrow default. */
.inspector__heading .inspector__eyebrow span { font-size: 13px; }

.field-editor__summary { resize: vertical; min-height: 88px; font: inherit; line-height: 1.6; }

.field-editor__hint { margin: 0; color: var(--dg-text-muted); font-size: 10px; }

/* Phase paragraphs of the chronological log: the leading time range reads as
   tabular data, the rest as prose. */
.summary-text { margin: 0; }
.summary-text--empty { color: var(--dg-text-muted); }
.summary-paragraph { margin: 0 0 8px; }

.summary-paragraph:last-child { margin-bottom: 0; }

.app-sites { display: flex; flex-wrap: wrap; gap: 7px; }
.app-sites li {
  display: flex;
  align-items: center;
  gap: 7px;
  min-width: 0;
  padding: 4px 8px 4px 4px;
  border: 1px solid var(--dg-timeline-grid);
  border-radius: 7px;
  background: var(--dg-track-fill);
  color: var(--dg-text-secondary);
  font-size: 10px;
}

.app-sites li > span {
  max-width: 180px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}


.distraction { display: grid; gap: 2px; padding: 8px 0; }
.distraction span { color: var(--dg-text-muted); font-size: 9px; }
.distraction strong { color: var(--dg-text-secondary); font-size: 11px; font-weight: 550; }

.inspector__section--frames { display: grid; gap: 12px; }
.frames-note { margin: 0; color: var(--dg-text-muted); font-size: 10px; }
.media-list { display: grid; gap: 8px; }
.media-list video { width: 100%; border-radius: 6px; background: var(--dg-track-fill); }

.card-player {
  margin: 10px 0 2px;
}

/* Focus verdict: three segmented toggles on the platform surface, the active
   one filled with its verdict colour. Solid controls, not glass (chrome only). */
.verdict__options {
  display: flex;
  gap: 8px;
  margin: 4px 0 0;
}

.verdict__option {
  display: inline-flex;
  flex: 1;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 8px 6px;
  border: 1px solid var(--dg-timeline-grid);
  border-radius: 9px;
  background: var(--dg-track-fill);
  color: var(--dg-text-secondary);
  font-size: 12px;
  font-weight: 550;
  cursor: pointer;
  transition:
    background var(--dg-motion-fast) ease,
    border-color var(--dg-motion-fast) ease,
    color var(--dg-motion-fast) ease;
}

.verdict__option i {
  width: 12px;
  height: 9px;
  border-radius: 3px;
  background: var(--verdict);
}

.verdict__option:hover:not(:disabled) { border-color: color-mix(in srgb, var(--verdict) 55%, var(--dg-timeline-grid)); }

.verdict__option.is-active {
  border-color: var(--verdict);
  background: color-mix(in srgb, var(--verdict) 16%, transparent);
  color: var(--dg-text-primary);
}

.verdict__option:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }
.verdict__option:disabled { opacity: 0.5; cursor: default; }

.verdict__hint { margin: 8px 0 0; color: var(--dg-text-muted); font-size: 10px; line-height: 1.5; }

/* Regenerate: sits in the action row beside delete, re-running the LLM on the
   card's batch. Accent-tinted on hover to read as the constructive counterpart
   to the destructive delete. */
.inspector__regenerate {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.inspector__regenerate:hover:not(:disabled) {
  border-color: var(--dg-accent);
  color: var(--dg-accent);
  background: var(--dg-accent-subtle);
}

.inspector__regenerate:disabled { opacity: 0.5; cursor: not-allowed; }

.reprocess-icon { width: 14px; height: 14px; flex-shrink: 0; }

.rating {
  padding: 14px 0 4px;
  border-top: 1px solid var(--dg-timeline-grid);
}

.rating-row {
  display: flex;
  align-items: center;
  gap: 10px;
  color: var(--dg-text-secondary);
  font-size: 12px;
}

.rating-row__thumb {
  display: grid;
  width: 28px;
  height: 28px;
  place-items: center;
  border: 1px solid var(--dg-timeline-grid);
  border-radius: 7px;
  background: var(--dg-track-fill);
  color: var(--dg-text-secondary);
  cursor: pointer;
}

.rating-row__thumb svg { width: 14px; height: 14px; }
.rating-row__thumb:disabled { opacity: 0.5; cursor: default; }
.rating-row__thumb:hover:not(:disabled) { border-color: color-mix(in srgb, var(--rating) 55%, var(--dg-timeline-grid)); }
.rating-row__thumb.is-active {
  border-color: var(--rating);
  background: color-mix(in srgb, var(--rating) 16%, transparent);
  color: var(--rating);
}
.rating-row__thumb:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }

.rating__hint { margin: 8px 0 0; color: var(--dg-text-muted); font-size: 10px; line-height: 1.5; }

@media (prefers-reduced-motion: reduce) {
  .field-pencil { transition: none; }
  .section-toggle__chevron { transition: none; }
}
</style>
