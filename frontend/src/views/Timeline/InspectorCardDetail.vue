<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import type { TimelineCardDTO, TimelineDayDTO } from '@/api/dto'
import type { TimelineActionAvailability } from '@/api/timeline'
import AppSiteIcon from '@/components/AppSiteIcon.vue'
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

const titleInput = ref<HTMLInputElement | null>(null)
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

/* The category picker lists the user-editable category names. A card
   currently in a built-in category (e.g. System) keeps it as an option so the
   user can leave the card untouched instead of being forced to move it. */
const categoryOptions = computed(() => {
  const names = props.day.categories
    .filter((category) => !category.isSystem)
    .map((category) => category.name)
  if (props.card.category !== '' && !names.includes(props.card.category)) {
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
  if (input instanceof HTMLInputElement) input.select()
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

watch(
  () => props.card.id,
  () => {
    editingField.value = null
    confirmingDelete.value = false
    draft.value = ''
  },
  { immediate: true },
)
</script>

<template>
  <header class="inspector__header">
    <div class="inspector__heading">
      <p class="inspector__eyebrow">
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
        <input
          ref="titleInput"
          v-model="draft"
          class="dg-input field-editor__title"
          type="text"
          maxlength="160"
          @keydown.esc="cancelEditing"
          @keydown.enter.prevent="submitEditing"
          @blur="submitEditing"
        />
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

  <div v-if="editingField === 'category'" class="field-editor">
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

  <section v-if="props.card.activityPoints.length > 0" class="inspector__section">
    <h3>{{ t('timeline.inspector.activityPoints') }}</h3>
    <ul class="activity-points">
      <li v-for="(point, index) in props.card.activityPoints" :key="index">
        <span class="activity-points__time">{{ point.time }}</span>
        <span>{{ point.description }}</span>
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
    <div>
      <h3>{{ t('timeline.inspector.media') }}</h3>
      <p v-if="videoURLs.length === 0">{{ t('timeline.inspector.framesUnavailable') }}</p>
    </div>
    <div v-if="videoURLs.length === 0" class="frame-placeholder" aria-hidden="true">
      <span></span><span></span><span></span>
    </div>
    <div v-else class="media-list">
      <video
        v-for="(url, index) in videoURLs"
        :key="url"
        controls
        preload="metadata"
        :src="url"
        :aria-label="t('timeline.inspector.mediaLabel', { count: index + 1 })"
      ></video>
    </div>
  </section>

  <p v-if="props.actionFailed" class="inspector__error" role="alert">
    {{ t('timeline.inspector.actionFailed') }}
  </p>

  <div class="inspector__actions">
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
    <template v-else>
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

.inspector__title--card {
  display: flex;
  align-items: flex-start;
  gap: 8px;
}

.title-text { min-width: 0; overflow-wrap: anywhere; }

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

.field-editor { display: grid; gap: 6px; }

.field-editor__title { font-size: 15px; font-weight: 620; }

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

.activity-points { display: flex; flex-direction: column; gap: 6px; margin: 0; padding: 0; list-style: none; }
.activity-points li {
  display: flex;
  align-items: baseline;
  gap: 10px;
  color: var(--dg-text-secondary);
  font-size: 12px;
}
.activity-points__time {
  flex: none;
  min-width: 52px;
  color: var(--dg-text-muted);
  font-variant-numeric: tabular-nums;
}

.distraction { display: grid; gap: 2px; padding: 8px 0; }
.distraction span { color: var(--dg-text-muted); font-size: 9px; }
.distraction strong { color: var(--dg-text-secondary); font-size: 11px; font-weight: 550; }

.inspector__section--frames { display: grid; gap: 12px; }
.frame-placeholder { display: grid; grid-template-columns: repeat(3, 1fr); gap: 5px; }
.frame-placeholder span { height: 52px; border: 1px solid var(--dg-timeline-grid); border-radius: 5px; background: var(--dg-timeline-frame-fill); }
.media-list { display: grid; gap: 8px; }
.media-list video { width: 100%; border-radius: 6px; background: var(--dg-track-fill); }

@media (prefers-reduced-motion: reduce) {
  .field-pencil { transition: none; }
}
</style>
