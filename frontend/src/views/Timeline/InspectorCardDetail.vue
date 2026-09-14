<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import type { TimelineCardDTO, TimelineDayDTO } from '@/api/dto'
import type { TimelineActionAvailability } from '@/api/timeline'
import AppSiteIcon from '@/components/AppSiteIcon.vue'
import { appSiteValues } from '@/lib/appSiteIcon'
import { useDurationFormat } from '@/lib/duration'
import { formatClockTime } from '@/lib/timeFormat'
import type { TimelineAction } from '@/stores/timeline'

import { safeCategoryColor } from './layout'

/* The inspector's card pane: the selected card's summary, apps, activity
   points, distractions and media, plus the title/category editor. */
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
  saveEdits: [cardID: number, title: string, category: string]
  delete: [cardID: number]
}>()

const { t, locale } = useI18n()
const duration = useDurationFormat()
const timeRange = computed(() => {
  return `${formatClockTime(props.card.startTs, locale.value, props.timeZone)} – ${formatClockTime(props.card.endTs, locale.value, props.timeZone)}`
})

const editing = ref(false)
const confirmingDelete = ref(false)
const draftTitle = ref('')
const draftCategory = ref('')

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

const canStartEditing = computed(
  () => props.canWrite && (props.actions.updateTitle || props.actions.updateCategory),
)

const titleChanged = computed(() => draftTitle.value.trim() !== '' && draftTitle.value.trim() !== props.card.title)
const categoryChanged = computed(() => draftCategory.value !== '' && draftCategory.value !== props.card.category)

const canSave = computed(() =>
  (titleChanged.value || categoryChanged.value) && props.pendingAction === null,
)

watch(
  () => props.card.id,
  () => {
    editing.value = false
    confirmingDelete.value = false
    draftTitle.value = props.card.title
    draftCategory.value = props.card.category
  },
  { immediate: true },
)

function beginEditing(): void {
  if (!canStartEditing.value) return
  draftTitle.value = props.card.title
  draftCategory.value = props.card.category
  confirmingDelete.value = false
  editing.value = true
}

function cancelEditing(): void {
  editing.value = false
  draftTitle.value = props.card.title
  draftCategory.value = props.card.category
}

function saveEditing(): void {
  if (!canSave.value) return
  emit('saveEdits', props.card.id, draftTitle.value.trim(), draftCategory.value)
  editing.value = false
}

function confirmDeletion(): void {
  emit('delete', props.card.id)
  confirmingDelete.value = false
}
</script>

<template>
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
    {{ timeRange }} · {{ duration(props.card.durationMinutes) }}
  </div>

  <form v-if="editing" class="editor" @submit.prevent="saveEditing" @keydown.esc="cancelEditing">
    <label>
      <span>{{ t('timeline.inspector.titleLabel') }}</span>
      <input
        v-model="draftTitle"
        class="dg-input"
        type="text"
        maxlength="160"
        :disabled="!props.actions.updateTitle || props.pendingAction !== null"
      />
    </label>
    <label>
      <span>{{ t('timeline.inspector.categoryLabel') }}</span>
      <select
        v-model="draftCategory"
        class="dg-input"
        :disabled="!props.actions.updateCategory || props.pendingAction !== null"
      >
        <option
          v-for="name in categoryOptions"
          :key="name"
          :value="name"
        >
          {{ name }}
        </option>
      </select>
    </label>
    <div class="editor__actions">
      <button type="button" class="dg-button" @click="cancelEditing">
        {{ t('common.action.cancel') }}
      </button>
      <button type="submit" class="dg-button dg-button--primary" :disabled="!canSave">
        {{ props.pendingAction === 'update-card' ? t('common.state.saving') : t('common.action.save') }}
      </button>
    </div>
  </form>

  <section class="inspector__section">
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

<style scoped>
.editor { display: grid; gap: 13px; padding: 16px 0; border-bottom: 1px solid var(--dg-timeline-grid); }

.editor label { display: grid; gap: 5px; }
.editor label > span { color: var(--dg-text-secondary); font-size: 10px; font-weight: 600; }
.editor__actions { display: flex; justify-content: flex-end; gap: 7px; }

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
</style>
