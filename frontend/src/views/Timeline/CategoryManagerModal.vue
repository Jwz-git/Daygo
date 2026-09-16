<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { saveCategories } from '@/api/timeline'
import type { CategoryDTO } from '@/api/dto'
import { safeCategoryColor } from './layout'

/*
 * Category management wizard (自定义类别), two steps like the reference:
 * titles + descriptions, then colors. Edits stay in a local draft until
 * 完成 — one SaveCategories call writes the whole set; the backend merges
 * the built-ins back and rewrites cards under renamed categories.
 * System rows (System / Idle) never enter the editor: is_system is
 * pipeline-assigned and the binding rejects rows claiming it.
 */
const props = defineProps<{
  categories: CategoryDTO[]
  canWrite: boolean
}>()

const emit = defineEmits<{
  close: []
  saved: []
}>()

const { t } = useI18n()

interface DraftCategory {
  id: string
  name: string
  details: string
  colorHex: string
  sortOrder: number
  isIdle: boolean
  createdAtTs: number
  updatedAtTs: number
}

const step = ref<1 | 2>(1)
const draft = ref<DraftCategory[]>([])
const editingIndex = ref<number | null>(null)
const nameDraft = ref('')
const detailsDraft = ref('')
const selectedIndex = ref(0)
const saving = ref(false)
const error = ref<string | null>(null)

watch(
  () => props.categories,
  () => {
    draft.value = props.categories
      .filter((category) => !category.isSystem)
      .map((category) => ({
        id: category.id,
        name: category.name,
        details: category.details,
        colorHex: category.colorHex,
        sortOrder: category.sortOrder,
        isIdle: category.isIdle,
        createdAtTs: category.createdAtTs,
        updatedAtTs: category.updatedAtTs,
      }))
  },
  { immediate: true, deep: false },
)

const PALETTE = [
  '#a855f7', '#ec4899', '#ef4444', '#eab308',
  '#84cc16', '#22c55e', '#22d3ee', '#3b82f6',
] as const

const selectedColor = computed(() => draft.value[selectedIndex.value]?.colorHex ?? PALETTE[0])

function beginEdit(index: number): void {
  const row = draft.value[index]
  if (row === undefined) return
  editingIndex.value = index
  nameDraft.value = row.name
  detailsDraft.value = row.details
}

function commitEdit(): void {
  const index = editingIndex.value
  if (index === null) return
  const row = draft.value[index]
  if (row === undefined) return
  const name = nameDraft.value.trim()
  if (name !== '') {
    row.name = name
    row.details = detailsDraft.value.trim()
  }
  editingIndex.value = null
}

function addCategory(): void {
  const maxOrder = draft.value.reduce((max, row) => Math.max(max, row.sortOrder), 0)
  draft.value.push({
    id: '',
    name: '',
    details: '',
    colorHex: PALETTE[draft.value.length % PALETTE.length] ?? PALETTE[0],
    sortOrder: maxOrder + 1,
    isIdle: false,
    createdAtTs: 0,
    updatedAtTs: 0,
  })
  beginEdit(draft.value.length - 1)
}

function removeCategory(index: number): void {
  draft.value.splice(index, 1)
  if (editingIndex.value === index) editingIndex.value = null
  selectedIndex.value = Math.min(selectedIndex.value, Math.max(0, draft.value.length - 1))
}

function applyColor(color: string): void {
  const row = draft.value[selectedIndex.value]
  if (row === undefined) return
  row.colorHex = color
}

function selectRow(index: number): void {
  if (editingIndex.value !== null) commitEdit()
  selectedIndex.value = index
}

function toNext(): void {
  if (editingIndex.value !== null) commitEdit()
  step.value = 2
}

function toBack(): void {
  step.value = 1
}

async function finish(): Promise<void> {
  if (editingIndex.value !== null) commitEdit()
  const names = draft.value.map((row) => row.name.trim())
  if (names.some((name) => name === '')) {
    error.value = t('timeline.manage2.nameRequired')
    return
  }
  const unique = new Set(names.map((name) => name.toLocaleLowerCase('en-US')))
  if (unique.size !== names.length) {
    error.value = t('timeline.manage2.nameDuplicate')
    return
  }
  saving.value = true
  error.value = null
  try {
    await saveCategories(
      draft.value.map((row) => ({
        id: row.id,
        name: row.name.trim(),
        colorHex: row.colorHex,
        details: row.details,
        sortOrder: row.sortOrder,
        isSystem: false,
        isIdle: row.isIdle,
        createdAtTs: row.createdAtTs,
        updatedAtTs: row.updatedAtTs,
      })),
    )
    emit('saved')
  } catch {
    error.value = t('timeline.manage2.saveFailed')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="wizard" role="dialog" :aria-label="t('timeline.manage2.title')">
    <button type="button" class="wizard__close" :aria-label="t('timeline.inspector.close')" @click="emit('close')">×</button>

    <div class="wizard__side">
      <p class="wizard__step">{{ t('timeline.manage2.stepOf', { current: step, total: 2 }) }}</p>
      <h2 class="wizard__title">{{ step === 1 ? t('timeline.manage2.step1Title') : t('timeline.manage2.step2Title') }}</h2>

      <template v-if="step === 1">
        <p class="wizard__hint">
          <svg viewBox="0 0 16 16" aria-hidden="true"><circle cx="8" cy="8" r="6.5" fill="none" stroke="currentColor" stroke-width="1.4" /><path d="M5.5 8h5M8 5.5v5" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" /></svg>
          {{ t('timeline.manage2.hint1') }}
        </p>
        <p class="wizard__hint">
          <svg viewBox="0 0 16 16" aria-hidden="true"><rect x="2.5" y="2.5" width="11" height="11" rx="2" fill="none" stroke="currentColor" stroke-width="1.4" /><path d="M5.5 6h5M5.5 8.5h5M5.5 11h3" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" /></svg>
          {{ t('timeline.manage2.hint2') }}
        </p>
        <p class="wizard__note">{{ t('timeline.manage2.optionalNote') }}</p>
      </template>

      <template v-else>
        <div class="wizard__swatches" aria-hidden="true">
          <i
            v-for="color in PALETTE"
            :key="color"
            :style="{ background: color }"
            :class="{ 'is-active': color === selectedColor }"
            role="button"
            :aria-label="t('timeline.manage2.step2Title')"
            @click="applyColor(color)"
          ></i>
        </div>
        <p class="wizard__note">{{ t('timeline.manage2.colorHint') }}</p>
      </template>
    </div>

    <div class="wizard__main">
      <div v-if="step === 1" class="wizard__list">
        <div v-for="(row, index) in draft" :key="row.id || `new-${index}`" class="wizard-row" :class="{ 'is-editing': editingIndex === index }">
          <template v-if="editingIndex === index">
            <input
              v-model="nameDraft"
              class="dg-input wizard-row__input"
              type="text"
              :placeholder="t('timeline.manage2.namePlaceholder')"
              maxlength="60"
              @keydown.enter="commitEdit"
            />
            <textarea
              v-model="detailsDraft"
              class="dg-input wizard-row__input"
              rows="2"
              maxlength="400"
              :placeholder="t('timeline.manage2.detailsPlaceholder')"
            ></textarea>
            <button type="button" class="dg-button wizard-row__apply" @click="commitEdit">{{ t('common.action.save') }}</button>
          </template>
          <template v-else>
            <div class="wizard-row__text">
              <strong>{{ row.name || t('timeline.manage2.namePlaceholder') }}</strong>
              <span>{{ row.details }}</span>
            </div>
            <div class="wizard-row__actions">
              <button type="button" class="wizard-row__icon" :aria-label="t('timeline.inspector.editTitle')" @click="beginEdit(index)">
                <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M11.3 1.7a2.4 2.4 0 0 1 3.4 3.4l-8.3 8.3-4.3 1 1-4.3 8.2-8.4Z" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round" /></svg>
              </button>
              <button type="button" class="wizard-row__icon wizard-row__icon--danger" :aria-label="t('timeline.manage2.deleteTitle')" @click="removeCategory(index)">
                <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M3 4h10M6.5 4V2.8h3V4M4.5 4l.6 9h5.8l.6-9M6.7 6.5v4.5M9.3 6.5v4.5" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" /></svg>
              </button>
            </div>
          </template>
        </div>
        <button type="button" class="wizard__add" @click="addCategory">+ {{ t('timeline.manage2.add') }}</button>
      </div>

      <div v-else class="wizard__list">
        <button
          v-for="(row, index) in draft"
          :key="row.id || `new-${index}`"
          type="button"
          class="wizard-row wizard-row--color"
          :class="{ 'is-selected': selectedIndex === index }"
          @click="selectRow(index)"
        >
          <i class="wizard-row__swatch" :style="{ background: safeCategoryColor(row.colorHex) }"></i>
          <span class="wizard-row__text">
            <strong>{{ row.name || t('timeline.manage2.namePlaceholder') }}</strong>
            <span>{{ row.details }}</span>
          </span>
        </button>
        <p class="wizard__optional">{{ t('timeline.manage2.colorNote') }}</p>
      </div>
    </div>

    <p v-if="error !== null" class="wizard__error" role="alert">{{ error }}</p>

    <footer class="wizard__footer">
      <button v-if="step === 2" type="button" class="dg-button" @click="toBack">{{ t('timeline.manage2.back') }}</button>
      <button v-if="step === 1" type="button" class="wizard__primary" @click="toNext">
        {{ t('timeline.manage2.next') }}
        <svg viewBox="0 0 12 12" aria-hidden="true"><path d="M4.5 2.5 8 6l-3.5 3.5" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" /></svg>
      </button>
      <button v-else type="button" class="wizard__primary" :disabled="saving || !canWrite" @click="finish">
        {{ saving ? t('timeline.failure.retrying') : t('timeline.manage2.finish') }}
        <svg viewBox="0 0 12 12" aria-hidden="true"><path d="M4.5 2.5 8 6l-3.5 3.5" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" /></svg>
      </button>
    </footer>
  </div>
</template>

<style scoped>
.wizard {
  position: relative;
  display: grid;
  grid-template-columns: 300px minmax(0, 1fr);
  grid-template-rows: minmax(0, 1fr) auto;
  gap: 8px 36px;
  width: min(960px, calc(100vw - 96px));
  height: min(640px, calc(100vh - 96px));
  padding: 30px 34px 22px;
  border-radius: 16px;
  background: var(--dg-popover-fill, var(--dg-surface));
  box-shadow: var(--lg-shadow-dense, var(--dg-shadow-lg));
}

.wizard__close {
  position: absolute;
  top: 16px;
  right: 16px;
  display: grid;
  width: 30px;
  height: 30px;
  place-items: center;
  border: none;
  border-radius: 50%;
  background: transparent;
  color: var(--dg-text-secondary);
  font-size: 18px;
  cursor: pointer;
}

.wizard__close:hover { background: var(--dg-hover-fill); }

.wizard__side { padding-top: 26px; }

.wizard__step {
  margin: 0 0 8px;
  color: var(--dg-accent-text);
  font-size: 12px;
  font-weight: 650;
}

.wizard__title {
  margin: 0 0 20px;
  color: var(--dg-text-primary);
  font-size: 24px;
  font-weight: 700;
}

.wizard__hint {
  display: flex;
  align-items: flex-start;
  gap: 9px;
  margin: 0 0 14px;
  color: var(--dg-text-secondary);
  font-size: 12px;
  line-height: 1.55;
}

.wizard__hint svg { flex: none; width: 16px; height: 16px; margin-top: 1px; color: var(--dg-accent-text); }

.wizard__note {
  margin: 14px 0 0;
  color: var(--dg-text-muted);
  font-size: 11px;
  line-height: 1.6;
}

.wizard__swatches {
  display: grid;
  grid-template-columns: repeat(2, 20px);
  gap: 10px;
  justify-content: center;
  align-content: center;
  min-height: 220px;
}

.wizard__swatches i {
  width: 20px;
  height: 20px;
  border-radius: 50%;
  opacity: 0.5;
  cursor: pointer;
  transition: transform var(--dg-motion-fast) var(--dg-ease-out), opacity var(--dg-motion-fast) ease;
}

.wizard__swatches i.is-active {
  opacity: 1;
  transform: scale(1.5);
  box-shadow: 0 0 0 2px var(--dg-surface), 0 4px 10px rgba(45, 50, 80, 0.25);
}

.wizard__main {
  min-height: 0;
  overflow-y: auto;
  border: 1px solid var(--dg-timeline-grid);
  border-radius: 14px;
  background: color-mix(in srgb, var(--dg-track-fill) 45%, transparent);
  padding: 8px;
}

.wizard__list { display: grid; gap: 8px; }

.wizard-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  border: 1px solid var(--dg-timeline-grid);
  border-radius: 10px;
  background: var(--dg-timeline-card-fill);
  text-align: left;
}

.wizard-row.is-editing { flex-direction: column; align-items: stretch; gap: 8px; }

.wizard-row--color {
  cursor: pointer;
  transition: border-color var(--dg-motion-fast) ease, box-shadow var(--dg-motion-fast) ease;
}

.wizard-row--color:hover { border-color: color-mix(in srgb, var(--dg-accent) 40%, transparent); }

.wizard-row--color.is-selected {
  border-color: var(--dg-accent);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--dg-accent) 28%, transparent);
}

.wizard-row__swatch {
  flex: none;
  width: 18px;
  height: 18px;
  border-radius: 6px;
}

.wizard-row__text {
  display: grid;
  gap: 3px;
  flex: 1;
  min-width: 0;
}

.wizard-row__text strong { color: var(--dg-text-primary); font-size: 13px; font-weight: 650; }
.wizard-row__text span {
  overflow: hidden;
  color: var(--dg-text-secondary);
  font-size: 11px;
  line-height: 1.45;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.wizard-row__actions { display: inline-flex; gap: 4px; opacity: 0; transition: opacity var(--dg-motion-fast) ease; }

.wizard-row:hover .wizard-row__actions,
.wizard-row__actions:focus-within { opacity: 1; }

.wizard-row__icon {
  display: grid;
  width: 28px;
  height: 28px;
  place-items: center;
  border: none;
  border-radius: 7px;
  background: transparent;
  color: var(--dg-accent-text);
  cursor: pointer;
}

.wizard-row__icon:hover { background: color-mix(in srgb, var(--dg-accent) 12%, transparent); }
.wizard-row__icon:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }
.wizard-row__icon svg { width: 14px; height: 14px; }

.wizard-row__icon--danger { color: var(--dg-text-muted); }
.wizard-row__icon--danger:hover { color: var(--dg-danger); background: color-mix(in srgb, var(--dg-danger) 10%, transparent); }

.wizard-row__input { width: 100%; }
.wizard-row__apply { align-self: flex-start; }

.wizard__add {
  justify-self: start;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  border: none;
  border-radius: 999px;
  background: var(--dg-control-fill);
  color: var(--dg-accent-text);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}

.wizard__add:hover { background: var(--dg-control-fill-hover); }
.wizard__add:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }

.wizard__optional {
  margin: 6px 4px 2px;
  color: var(--dg-text-muted);
  font-size: 11px;
}

.wizard__error {
  grid-column: 1 / -1;
  margin: 0;
  color: var(--dg-danger);
  font-size: 12px;
  text-align: right;
}

.wizard__footer {
  grid-column: 1 / -1;
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 12px;
}

.wizard__primary {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  height: 38px;
  padding: 0 22px;
  border: none;
  border-radius: 999px;
  background: var(--dg-accent);
  color: #ffffff;
  font-size: 13px;
  font-weight: 650;
  cursor: pointer;
  box-shadow: var(--dg-shadow-sm);
  transition: transform var(--dg-motion-fast) var(--dg-ease-out), filter var(--dg-motion-fast) ease;
}

.wizard__primary:hover:not(:disabled) { transform: scale(1.03); background: var(--dg-accent-strong); }
.wizard__primary:active:not(:disabled) { transform: scale(0.97); }
.wizard__primary:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }
.wizard__primary:disabled { opacity: 0.55; cursor: default; }
.wizard__primary svg { width: 11px; height: 11px; }
</style>
