<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import type { CategoryDTO, TimelineCardDTO, TimelineDayDTO, TimelineFailureDTO } from '@/api/dto'
import type { TimelineActionAvailability } from '@/api/timeline'
import AppSiteIcon from '@/components/AppSiteIcon.vue'
import { appSiteValues } from '@/lib/appSiteIcon'
import { useDurationFormat } from '@/lib/duration'
import type { TimelineAction } from '@/stores/timeline'

import { safeCategoryColor } from './layout'

const props = defineProps<{
  day: TimelineDayDTO
  card: TimelineCardDTO | null
  failure: TimelineFailureDTO | null
  canWrite: boolean
  actions: TimelineActionAvailability
  pendingAction: TimelineAction | null
  actionFailed: boolean
}>()

const emit = defineEmits<{
  close: []
  updateTitle: [cardID: number, title: string]
  updateCategory: [cardID: number, category: string]
  delete: [cardID: number]
  retry: [batchIDs: number[]]
  dismissFailure: [batchIDs: number[]]
}>()
const { t, locale } = useI18n()
const editing = ref(false)
const confirmingDelete = ref(false)
const confirmingFailureDelete = ref(false)
const draftTitle = ref('')
const draftCategory = ref('')

// The retryable flag describes automatic requeue behavior only — the backend
// RetryBatches deliberately ignores both the attempt cap and failure kind
// ("not to overrule the user asking for one more run"), so every failed range
// with batches gets a manual retry button regardless of that flag.
const failuresWithBatches = computed(() =>
  props.day.failures.filter((failure) => failure.batchIds.length > 0),
)

const canRetry = computed(() => props.canWrite && props.actions.retryBatches)
const canDeleteFailure = computed(() => props.canWrite && props.actions.deleteBatches)

const failureClock = computed(() => {
  const failure = props.failure
  if (failure === null) return null
  const format = new Intl.DateTimeFormat(locale.value, {
    hour: 'numeric',
    minute: '2-digit',
  })
  return `${format.format(new Date(failure.startTs * 1000))} – ${format.format(new Date(failure.endTs * 1000))}`
})

interface CategoryTotal {
  category: CategoryDTO
  minutes: number
  percentage: number
}

const categoryTotals = computed<CategoryTotal[]>(() => {
  const totals = new Map<string, number>()
  for (const card of props.day.cards) {
    if (card.isIdle || card.category === 'System') continue
    totals.set(card.category, (totals.get(card.category) ?? 0) + card.durationMinutes)
  }

  const denominator = Math.max(1, [...totals.values()].reduce((sum, value) => sum + value, 0))
  return props.day.categories
    .filter((category) => totals.has(category.name))
    .map((category) => {
      const minutes = totals.get(category.name) ?? 0
      return { category, minutes, percentage: (minutes / denominator) * 100 }
    })
    .sort((left, right) => right.minutes - left.minutes)
})

const selectedColor = computed(() => {
  const category = props.day.categories.find((entry) => entry.name === props.card?.category)
  return safeCategoryColor(category?.colorHex)
})

const displayedAppSites = computed(() => appSiteValues(props.card?.appSites ?? null))

const videoURLs = computed(() => {
  const card = props.card
  if (card === null) return []
  return [...new Set([card.videoSummaryUrl, ...card.otherVideoSummaryUrls])]
    .filter((url): url is string => typeof url === 'string' && url.trim() !== '')
})

const editableCategories = computed(() =>
  props.day.categories.filter((category) => !category.isSystem),
)

const canStartEditing = computed(
  () => props.canWrite && (props.actions.updateTitle || props.actions.updateCategory),
)

const canSaveTitle = computed(() => {
  const card = props.card
  const title = draftTitle.value.trim()
  return card !== null &&
    title !== '' &&
    title !== card.title &&
    props.actions.updateTitle &&
    props.pendingAction === null
})

const canSaveCategory = computed(() => {
  const card = props.card
  const category = draftCategory.value.trim()
  return card !== null &&
    category !== '' &&
    category !== card.category &&
    props.actions.updateCategory &&
    props.pendingAction === null
})

watch(
  () => props.card?.id ?? null,
  () => {
    editing.value = false
    confirmingDelete.value = false
    draftTitle.value = props.card?.title ?? ''
    draftCategory.value = props.card?.category ?? ''
  },
  { immediate: true },
)

watch(
  () => props.failure?.startTs ?? null,
  () => {
    confirmingFailureDelete.value = false
  },
  { immediate: true },
)

function beginEditing(): void {
  if (!canStartEditing.value || props.card === null) return
  draftTitle.value = props.card.title
  draftCategory.value = props.card.category
  confirmingDelete.value = false
  editing.value = true
}

function cancelEditing(): void {
  editing.value = false
  draftTitle.value = props.card?.title ?? ''
  draftCategory.value = props.card?.category ?? ''
}

function saveTitle(): void {
  if (!canSaveTitle.value || props.card === null) return
  emit('updateTitle', props.card.id, draftTitle.value.trim())
  editing.value = false
}

function saveCategory(): void {
  if (!canSaveCategory.value || props.card === null) return
  emit('updateCategory', props.card.id, draftCategory.value.trim())
  editing.value = false
}

function confirmDeletion(): void {
  if (props.card === null) return
  emit('delete', props.card.id)
  confirmingDelete.value = false
}

const duration = useDurationFormat()
</script>

<template>
  <aside class="inspector dg-card" :aria-label="t('timeline.inspector.title')">
    <template v-if="props.card === null && props.failure === null">
      <header class="inspector__header">
        <div>
          <p class="inspector__eyebrow">{{ t('timeline.overview.eyebrow') }}</p>
          <h2 class="inspector__title dg-display">{{ t('timeline.overview.title') }}</h2>
        </div>
      </header>

      <div class="totals">
        <div class="total total--primary">
          <strong>{{ duration(props.day.trackedMinutes) }}</strong>
          <span>{{ t('timeline.overview.tracked') }}</span>
        </div>
        <div class="total">
          <strong>{{ duration(props.day.idleMinutes) }}</strong>
          <span>{{ t('timeline.overview.idle') }}</span>
        </div>
      </div>

      <div class="category-list">
        <p v-if="categoryTotals.length === 0" class="category-list__empty">
          {{ t('timeline.overview.noCategories') }}
        </p>
        <div v-for="item in categoryTotals" :key="item.category.id" class="category-total">
          <div class="category-total__meta">
            <span>
              <i :style="{ background: safeCategoryColor(item.category.colorHex) }"></i>
              {{ item.category.name }}
            </span>
            <strong>{{ duration(item.minutes) }}</strong>
          </div>
          <div class="category-total__track" aria-hidden="true">
            <span
              :style="{
                width: `${item.percentage}%`,
                background: safeCategoryColor(item.category.colorHex),
              }"
            ></span>
          </div>
        </div>
      </div>

      <section v-if="failuresWithBatches.length > 0" class="inspector__section inspector__failures">
        <h3>{{ t('timeline.failure.title') }}</h3>
        <p class="inspector__failure-note">{{ t('timeline.failure.retryHint') }}</p>
        <button
          v-for="failure in failuresWithBatches"
          :key="`${failure.startTs}-${failure.endTs}`"
          type="button"
          class="dg-button inspector__retry"
          :disabled="!canRetry || props.pendingAction !== null"
          :title="canRetry ? t('timeline.failure.retry') : t('timeline.failure.retryUnavailable')"
          @click="emit('retry', failure.batchIds)"
        >
          {{ props.pendingAction === 'retry-batches' ? t('timeline.failure.retrying') : t('timeline.failure.retry') }}
        </button>
      </section>
    </template>

    <template v-else-if="props.failure !== null">
      <header class="inspector__header">
        <div>
          <p class="inspector__eyebrow inspector__eyebrow--danger">{{ t('timeline.failure.title') }}</p>
          <h2 class="inspector__title inspector__title--card">{{ t('timeline.failure.detailTitle') }}</h2>
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

      <div class="card-time card-time--failure">
        <span :style="{ background: 'var(--dg-danger)' }"></span>
        {{ failureClock }}
      </div>

      <section class="inspector__section">
        <h3>{{ t('timeline.failure.detail.kind') }}</h3>
        <p class="inspector__failure-kind">{{ props.failure.kind }}</p>
      </section>

      <section class="inspector__section">
        <h3>{{ t('timeline.failure.detail.message') }}</h3>
        <p class="inspector__failure-message">{{ props.failure.message }}</p>
      </section>

      <section class="inspector__section">
        <h3>{{ t('timeline.failure.detail.batches') }}</h3>
        <ul class="failure-batches">
          <li v-for="id in props.failure.batchIds" :key="id">#{{ id }}</li>
        </ul>
        <p class="inspector__failure-note">
          {{ props.failure.retryable ? t('timeline.failure.detail.retryable') : t('timeline.failure.detail.notRetryable') }}
        </p>
      </section>

      <p v-if="props.actionFailed" class="inspector__error" role="alert">
        {{ t('timeline.inspector.actionFailed') }}
      </p>

      <div class="inspector__actions">
        <template v-if="confirmingFailureDelete">
          <span class="inspector__confirm">{{ t('timeline.failure.deleteConfirm') }}</span>
          <button type="button" class="dg-button" :disabled="props.pendingAction !== null" @click="confirmingFailureDelete = false">
            {{ t('common.action.cancel') }}
          </button>
          <button
            type="button"
            class="dg-button inspector__delete"
            :disabled="props.pendingAction !== null"
            @click="emit('dismissFailure', props.failure.batchIds); confirmingFailureDelete = false"
          >
            {{ t('common.action.delete') }}
          </button>
        </template>
        <template v-else>
          <button
            type="button"
            class="dg-button"
            :disabled="!canRetry || props.pendingAction !== null"
            :title="canRetry ? t('timeline.failure.retry') : t('timeline.failure.retryUnavailable')"
            @click="emit('retry', props.failure.batchIds)"
          >
            {{ props.pendingAction === 'retry-batches' ? t('timeline.failure.retrying') : t('common.action.retry') }}
          </button>
          <button
            type="button"
            class="dg-button inspector__danger"
            :disabled="!canDeleteFailure || props.pendingAction !== null"
            :title="canDeleteFailure ? t('timeline.failure.delete') : t('timeline.failure.deleteUnavailable')"
            @click="confirmingFailureDelete = true"
          >
            {{ t('common.action.delete') }}
          </button>
        </template>
        <span v-if="!props.canWrite" class="inspector__readonly">
          {{ t('timeline.inspector.readOnly') }}
        </span>
      </div>
    </template>

    <template v-else-if="props.card !== null">
      <header class="inspector__header">
        <div>
          <p class="inspector__eyebrow">{{ props.card.category }}</p>
          <h2 class="inspector__title inspector__title--card">{{ props.card.title }}</h2>
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
        {{ props.card.start }} – {{ props.card.end }} · {{ duration(props.card.durationMinutes) }}
      </div>

      <div v-if="editing" class="editor">
        <label>
          <span>{{ t('timeline.inspector.titleLabel') }}</span>
          <span class="editor__field">
            <input
              v-model="draftTitle"
              class="dg-input"
              type="text"
              maxlength="160"
              :disabled="!props.actions.updateTitle || props.pendingAction !== null"
              @keydown.enter.prevent="saveTitle"
            />
            <button type="button" class="dg-button" :disabled="!canSaveTitle" @click="saveTitle">
              {{ props.pendingAction === 'update-title' ? t('common.state.saving') : t('common.action.save') }}
            </button>
          </span>
        </label>
        <label>
          <span>{{ t('timeline.inspector.categoryLabel') }}</span>
          <span class="editor__field">
            <select
              v-model="draftCategory"
              class="dg-input"
              :disabled="!props.actions.updateCategory || props.pendingAction !== null"
            >
              <option
                v-for="category in editableCategories"
                :key="category.id"
                :value="category.name"
              >
                {{ category.name }}
              </option>
            </select>
            <button type="button" class="dg-button" :disabled="!canSaveCategory" @click="saveCategory">
              {{ props.pendingAction === 'update-category' ? t('common.state.saving') : t('common.action.save') }}
            </button>
          </span>
        </label>
        <div class="editor__actions">
          <button type="button" class="dg-button" @click="cancelEditing">
            {{ t('common.action.cancel') }}
          </button>
        </div>
      </div>

      <section v-else class="inspector__section">
        <h3>{{ t('timeline.inspector.summary') }}</h3>
        <p>{{ props.card.detailedSummary || props.card.summary || t('timeline.inspector.noSummary') }}</p>
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
            class="dg-button"
            :disabled="!canStartEditing || props.pendingAction !== null"
            :title="canStartEditing ? t('common.action.edit') : t('timeline.inspector.actionsUnavailable')"
            @click="beginEditing"
          >
            {{ t('common.action.edit') }}
          </button>
          <button
            type="button"
            class="dg-button"
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
        <span v-else-if="!canStartEditing && !props.actions.deleteCard" class="inspector__readonly">
          {{ t('timeline.inspector.actionsUnavailable') }}
        </span>
      </div>
    </template>
  </aside>
</template>

<style scoped>
.inspector {
  min-height: 0;
  padding: 22px;
  overflow-y: auto;
  background: var(--dg-timeline-inspector-fill);
}

.inspector__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding-bottom: 18px;
  border-bottom: 1px solid var(--dg-timeline-grid);
}

.inspector__eyebrow {
  margin-bottom: 5px;
  color: var(--dg-accent-text);
  font-size: 10px;
  font-weight: 600;
}

.inspector__eyebrow--danger { color: var(--dg-danger); }

.inspector__failure-kind {
  padding: 6px 10px;
  border: 1px solid color-mix(in srgb, var(--dg-danger) 30%, transparent);
  border-radius: 7px;
  background: var(--dg-danger-fill);
  color: var(--dg-danger);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
}

.inspector__failure-message {
  padding: 10px 12px;
  border-radius: 7px;
  background: var(--dg-track-fill);
  color: var(--dg-text-secondary);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
  line-height: 1.6;
  overflow-wrap: anywhere;
  white-space: pre-wrap;
}

.failure-batches {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.failure-batches li {
  padding: 3px 8px;
  border: 1px solid var(--dg-timeline-grid);
  border-radius: 6px;
  background: var(--dg-track-fill);
  color: var(--dg-text-muted);
  font-size: 10px;
  font-variant-numeric: tabular-nums;
}

.inspector__failure-note { margin-top: 8px; }

.card-time--failure { color: var(--dg-danger); }

.inspector__title {
  color: var(--dg-text-primary);
  font-size: 26px;
  line-height: 1.1;
}

.inspector__title--card {
  font-size: 18px;
  font-weight: 650;
  line-height: 1.3;
}

.inspector__close {
  display: grid;
  flex: none;
  width: 28px;
  height: 28px;
  border-radius: 50%;
  color: var(--dg-text-secondary);
  font-size: 20px;
  line-height: 1;
  place-items: center;
}

.inspector__close:hover { background: var(--dg-hover-fill); }

.totals {
  display: grid;
  grid-template-columns: 1.35fr 1fr;
  gap: 8px;
  padding: 18px 0 22px;
}

.total {
  display: flex;
  flex-direction: column;
  min-width: 0;
  padding: 13px;
  border: 1px solid var(--dg-timeline-grid);
  border-radius: 8px;
  background: var(--dg-track-fill);
}

.total strong {
  overflow: hidden;
  color: var(--dg-text-primary);
  font-size: 16px;
  font-weight: 650;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.total span,
.category-list__empty {
  color: var(--dg-text-muted);
  font-size: 10px;
}

.total--primary {
  border-color: color-mix(in srgb, var(--dg-accent) 22%, transparent);
  background: var(--dg-control-fill);
}

.category-list { display: grid; gap: 15px; }
.category-total__meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 6px;
  color: var(--dg-text-secondary);
  font-size: 11px;
}

.category-total__meta span { display: flex; align-items: center; gap: 7px; min-width: 0; }
.category-total__meta i { width: 7px; height: 7px; border-radius: 50%; }
.category-total__meta strong { color: var(--dg-text-primary); font-weight: 600; }
.category-total__track { height: 4px; overflow: hidden; border-radius: 99px; background: var(--dg-track-fill); }
.category-total__track span { display: block; height: 100%; border-radius: inherit; }

.card-time {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 14px 0;
  color: var(--dg-text-secondary);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}

.card-time span { width: 7px; height: 7px; border-radius: 50%; }

.editor {
  display: grid;
  gap: 13px;
  padding: 16px 0;
  border-bottom: 1px solid var(--dg-timeline-grid);
}

.editor label { display: grid; gap: 5px; }
.editor label > span { color: var(--dg-text-secondary); font-size: 10px; font-weight: 600; }
.editor__field { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 7px; }
.editor__actions { display: flex; justify-content: flex-end; gap: 7px; }

.inspector__section { padding: 16px 0; border-top: 1px solid var(--dg-timeline-grid); }
.inspector__section:first-of-type { border-top: 0; }
.inspector__section h3 { margin-bottom: 6px; color: var(--dg-text-primary); font-size: 11px; font-weight: 650; }
.inspector__section p { color: var(--dg-text-secondary); font-size: 11px; line-height: 1.6; }
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
.frame-placeholder { display: grid; grid-template-columns: repeat(3, 1fr); gap: 5px; }
.frame-placeholder span { height: 52px; border: 1px solid var(--dg-timeline-grid); border-radius: 5px; background: var(--dg-timeline-frame-fill); }
.media-list { display: grid; gap: 8px; }
.media-list video { width: 100%; border-radius: 6px; background: var(--dg-track-fill); }
.inspector__actions { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; padding-top: 18px; border-top: 1px solid var(--dg-timeline-grid); }
.inspector__readonly { width: 100%; color: var(--dg-text-muted); font-size: 10px; }
.inspector__confirm { width: 100%; color: var(--dg-text-secondary); font-size: 11px; }

.inspector__failures {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 8px;
  padding-top: 18px;
  border-top: 1px solid var(--dg-timeline-grid);
}

.inspector__failure-note { color: var(--dg-text-muted); font-size: 11px; }

.inspector__retry { color: var(--dg-text-secondary); }

.inspector__danger { color: var(--dg-danger); }

.inspector__danger:not(:disabled):hover {
  background: color-mix(in srgb, var(--dg-danger) 9%, transparent);
}

.inspector__delete { border-color: color-mix(in srgb, var(--dg-danger) 34%, transparent); color: var(--dg-danger); }
.inspector__error { margin: 4px 0 12px; color: var(--dg-danger); font-size: 11px; }
</style>
