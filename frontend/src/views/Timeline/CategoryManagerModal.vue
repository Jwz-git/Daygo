<script setup lang="ts">
import DgIcon from '@/components/DgIcon.vue'
import { computed, nextTick, onMounted, ref, watch, type ComponentPublicInstance } from 'vue'
import { useI18n } from 'vue-i18n'

import { saveCategories } from '@/api/timeline'
import { CATEGORY_ARCHETYPES, type CategoryArchetype, type CategoryDTO } from '@/api/dto'
import ColorCanvasPicker from '@/components/ColorCanvasPicker.vue'
import { categoryDetails, categoryLabel } from '@/lib/categoryLabel'
import { SWATCH_COUNT, spectrumPalette } from '@/lib/colorPalette'
import { safeCategoryColor } from './layout'

/*
 * Category management wizard (自定义类别), two steps like the reference:
 * titles + descriptions, then colors. Edits stay in a local draft until
 * 完成 — one SaveCategories call writes the whole set; the backend merges
 * the built-ins back and rewrites cards under renamed categories.
 * System rows (System / Idle) never enter the editor: is_system is
 * pipeline-assigned and the binding rejects rows claiming it.
 *
 * Stored rows are English (the model matches activity against them), so the
 * rows and the edit fields show the localized copy of an untouched default.
 * Committing an edit writes that copy into the draft: from 完成 on, the
 * category is the user's own text and its cards are renamed with it.
 *
 * Closing with unsaved draft edits asks first (the close button, Esc and a
 * backdrop click through the exposed requestClose). Profession presets are
 * not offered here: they replace the whole set and belong to the first-run
 * guide only.
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
  archetype: CategoryArchetype
  createdAtTs: number
  updatedAtTs: number
}

const root = ref<HTMLElement | null>(null)
const step = ref<1 | 2>(1)
const draft = ref<DraftCategory[]>([])
const editingIndex = ref<number | null>(null)
/** The row being edited was just added: cancelling it removes the row. */
const editingIsNew = ref(false)
const nameDraft = ref('')
const detailsDraft = ref('')
const archetypeDraft = ref<CategoryArchetype>('forge')
const editError = ref<string | null>(null)
const saving = ref(false)
const error = ref<string | null>(null)
const confirmingClose = ref(false)

let nameInput: HTMLInputElement | null = null
function setNameInput(element: Element | ComponentPublicInstance | null): void {
  nameInput = element instanceof HTMLInputElement ? element : null
}

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
        archetype: category.archetype,
        createdAtTs: category.createdAtTs,
        updatedAtTs: category.updatedAtTs,
      }))
    editingIndex.value = null
  },
  { immediate: true, deep: false },
)

onMounted(() => root.value?.focus({ preventScroll: true }))

/** Unsaved edits in the draft, which closing would discard. */
const dirty = computed(() => {
  const stored = props.categories.filter((category) => !category.isSystem)
  if (editingIndex.value !== null || stored.length !== draft.value.length) return true
  return draft.value.some((row, index) => {
    const original = stored[index]
    return original === undefined || original.id !== row.id || original.name !== row.name ||
      original.details !== row.details || original.colorHex !== row.colorHex ||
      original.archetype !== row.archetype
  })
})

/*
 * Color stage, ported from the reference implementation: the canvas sets a
 * hue (drag angle) and a lightness (drag radius), and the eight spectrum
 * swatches derive from them. A color is applied by dragging a swatch onto a
 * category card, or — the keyboard- and trackpad-friendly path — by clicking
 * a swatch and then the cards it should go to.
 */
const paletteAngle = ref(-Math.PI / 2)
const paletteRadius = ref(0.7)
const draggingColor = ref(false)
const targetedIndex = ref<number | null>(null)
const pickedColor = ref<string | null>(null)
/** The reference shows a "Drag to category" hint until the first drag. */
const showDragHint = ref(true)

const spectrumColors = computed(() =>
  spectrumPalette(paletteAngle.value, paletteRadius.value),
)

const colorHint = computed(() => {
  if (draggingColor.value) return t('timeline.manage2.colorDropHint')
  if (pickedColor.value !== null) return t('timeline.manage2.colorPickedHint')
  return t('timeline.manage2.colorDragHint')
})

function onPaletteChange(value: { angle: number; normalizedRadius: number }): void {
  paletteAngle.value = value.angle
  paletteRadius.value = value.normalizedRadius
  // The swatches are recomputed, so a picked color is no longer on offer.
  pickedColor.value = null
}

function toggleSwatch(hex: string): void {
  showDragHint.value = false
  pickedColor.value = pickedColor.value === hex ? null : hex
}

function onSwatchDragStart(event: DragEvent, hex: string): void {
  draggingColor.value = true
  showDragHint.value = false
  if (event.dataTransfer !== null) {
    event.dataTransfer.setData('text/plain', hex)
    event.dataTransfer.effectAllowed = 'copy'
  }
}

function onSwatchDragEnd(): void {
  draggingColor.value = false
  targetedIndex.value = null
}

function onCardDragOver(index: number, event: DragEvent): void {
  if (!draggingColor.value) return
  targetedIndex.value = index
  // dragover must be cancelled for the drop to fire at all.
  event.preventDefault()
  if (event.dataTransfer !== null) event.dataTransfer.dropEffect = 'copy'
}

function onCardDragLeave(index: number, event: DragEvent): void {
  // dragleave also fires when the pointer crosses a child of the card; only a
  // leave that exits the card entirely should drop the highlight.
  const card = event.currentTarget as HTMLElement | null
  const next = event.relatedTarget as Node | null
  if (card !== null && next !== null && card.contains(next)) return
  if (targetedIndex.value === index) targetedIndex.value = null
}

function applyColor(index: number, hex: string): void {
  const row = draft.value[index]
  if (row === undefined || !/^#[0-9a-f]{6}$/i.test(hex)) return
  row.colorHex = hex
}

function onCardDrop(index: number, event: DragEvent): void {
  const hex = event.dataTransfer?.getData('text/plain')?.trim() ?? ''
  draggingColor.value = false
  targetedIndex.value = null
  applyColor(index, hex)
}

function onCardClick(index: number): void {
  if (pickedColor.value !== null) applyColor(index, pickedColor.value)
}

function displayName(row: DraftCategory): string {
  return categoryLabel(row.name, t)
}

function displayDetails(row: DraftCategory): string {
  return categoryDetails(row.name, row.details, t)
}

/**
 * Leaves the open edit before another one starts: a just-added row that is
 * still blank is dropped, anything else must commit (false when invalid).
 */
function settleEdit(): boolean {
  if (editingIndex.value === null) return true
  if (editingIsNew.value && nameDraft.value.trim() === '' && detailsDraft.value.trim() === '') {
    cancelEdit()
    return true
  }
  return commitEdit()
}

function beginEdit(index: number, isNew = false): void {
  if (editingIndex.value === index) return
  if (!settleEdit()) return
  const row = draft.value[index]
  if (row === undefined) return
  editingIndex.value = index
  editingIsNew.value = isNew
  editError.value = null
  nameDraft.value = displayName(row)
  detailsDraft.value = displayDetails(row)
  archetypeDraft.value = row.archetype
  void nextTick(() => nameInput?.focus())
}

/** Writes the open edit into the draft; false (and stays open) when invalid. */
function commitEdit(): boolean {
  const index = editingIndex.value
  if (index === null) return true
  const row = draft.value[index]
  if (row === undefined) return true
  const name = nameDraft.value.trim()
  if (name === '') {
    editError.value = t('timeline.manage2.nameRequired')
    nameInput?.focus()
    return false
  }
  const key = name.toLocaleLowerCase('en-US')
  const taken = draft.value.some((other, otherIndex) => otherIndex !== index && (
    other.name.trim().toLocaleLowerCase('en-US') === key ||
    displayName(other).trim().toLocaleLowerCase('en-US') === key
  ))
  if (taken) {
    editError.value = t('timeline.manage2.nameDuplicate')
    nameInput?.focus()
    return false
  }
  row.name = name
  row.details = detailsDraft.value.trim()
  row.archetype = archetypeDraft.value
  editingIndex.value = null
  editingIsNew.value = false
  editError.value = null
  return true
}

function cancelEdit(): void {
  const index = editingIndex.value
  if (index === null) return
  editingIndex.value = null
  editError.value = null
  if (editingIsNew.value) draft.value.splice(index, 1)
  editingIsNew.value = false
}

function onDetailsKeydown(event: KeyboardEvent): void {
  if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) {
    event.preventDefault()
    commitEdit()
  }
}

function addCategory(): void {
  if (!settleEdit()) return
  const maxOrder = draft.value.reduce((max, row) => Math.max(max, row.sortOrder), 0)
  draft.value.push({
    id: '',
    name: '',
    details: '',
    colorHex: spectrumColors.value[draft.value.length % SWATCH_COUNT]
      ?? spectrumColors.value[0]
      ?? '#7D7A84',
    sortOrder: maxOrder + 1,
    isIdle: false,
    archetype: 'forge',
    createdAtTs: 0,
    updatedAtTs: 0,
  })
  beginEdit(draft.value.length - 1, true)
}

function removeCategory(index: number): void {
  draft.value.splice(index, 1)
  if (editingIndex.value === index) {
    editingIndex.value = null
    editingIsNew.value = false
    editError.value = null
  } else if (editingIndex.value !== null && editingIndex.value > index) {
    editingIndex.value -= 1
  }
}

function goTo(next: 1 | 2): void {
  if (next === step.value) return
  if (next === 2 && !settleEdit()) return
  pickedColor.value = null
  error.value = null
  step.value = next
}

function requestClose(): void {
  if (saving.value) return
  if (dirty.value) {
    confirmingClose.value = true
    return
  }
  emit('close')
}

function onKeydown(event: KeyboardEvent): void {
  if (event.key !== 'Escape') return
  event.preventDefault()
  event.stopPropagation()
  if (confirmingClose.value) confirmingClose.value = false
  else if (editingIndex.value !== null) cancelEdit()
  else if (pickedColor.value !== null) pickedColor.value = null
  else requestClose()
}

async function finish(): Promise<void> {
  if (!settleEdit()) {
    goTo(1)
    return
  }
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
        archetype: row.archetype,
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

defineExpose({ requestClose })
</script>

<template>
  <div
    ref="root"
    class="wizard"
    role="dialog"
    aria-modal="true"
    :aria-label="t('timeline.manage2.title')"
    tabindex="-1"
    @keydown="onKeydown"
  >
    <header class="wizard__header">
      <div class="wizard__progress">
        <span class="wizard__step">{{ t('timeline.manage2.stepOf', { current: step, total: 2 }) }}</span>
        <span class="wizard__bars">
          <button
            v-for="n in ([1, 2] as const)"
            :key="n"
            type="button"
            class="wizard__bar"
            :class="{ 'is-active': step >= n }"
            :aria-label="n === 1 ? t('timeline.manage2.step1Title') : t('timeline.manage2.step2Title')"
            :aria-current="step === n ? 'step' : undefined"
            @click="goTo(n)"
          ></button>
        </span>
      </div>
      <button type="button" class="wizard__close" :aria-label="t('timeline.inspector.close')" @click="requestClose">
        <DgIcon name="close" :size="14" />
      </button>
    </header>

    <aside class="wizard__side">
      <h2 class="wizard__title">{{ step === 1 ? t('timeline.manage2.step1Title') : t('timeline.manage2.step2Title') }}</h2>

      <template v-if="step === 1">
        <p class="wizard__hint">
          <DgIcon name="plusCircle" :size="16" />
          {{ t('timeline.manage2.hint1') }}
        </p>
        <p class="wizard__hint">
          <DgIcon name="list" :size="16" />
          {{ t('timeline.manage2.hint2') }}
        </p>
        <p class="wizard__note">{{ t('timeline.manage2.optionalNote') }}</p>
      </template>

      <template v-else>
        <ColorCanvasPicker class="wizard__canvas" @change="onPaletteChange" />

        <div class="wizard__swatches" role="list" :aria-label="t('timeline.manage2.step2Title')">
          <button
            v-for="(color, index) in spectrumColors"
            :key="color"
            type="button"
            role="listitem"
            class="wizard__swatch"
            :class="{ 'is-picked': pickedColor === color }"
            :style="{ background: color }"
            :draggable="true"
            :aria-label="color"
            :aria-pressed="pickedColor === color"
            @click="toggleSwatch(color)"
            @dragstart="onSwatchDragStart($event, color)"
            @dragend="onSwatchDragEnd"
          >
            <span v-if="showDragHint && index === 0" class="wizard__swatch-hint">
              {{ t('timeline.manage2.swatchDragHint') }}
            </span>
          </button>
        </div>

        <p class="wizard__note" aria-live="polite">{{ colorHint }}</p>
      </template>
    </aside>

    <section class="wizard__main">
      <div v-if="step === 1" class="wizard__list">
        <div
          v-for="(row, index) in draft"
          :key="row.id || `new-${index}`"
          class="wizard-row"
          :class="{ 'is-editing': editingIndex === index }"
        >
          <template v-if="editingIndex === index">
            <div class="wizard-row__edit-head">
              <i class="wizard-row__dot" :style="{ background: safeCategoryColor(row.colorHex) }" aria-hidden="true"></i>
              <input
                :ref="setNameInput"
                v-model="nameDraft"
                class="dg-input wizard-row__input"
                :class="{ 'is-invalid': editError !== null }"
                type="text"
                :placeholder="t('timeline.manage2.namePlaceholder')"
                :aria-invalid="editError !== null"
                maxlength="60"
                @input="editError = null"
                @keydown.enter.prevent="commitEdit"
              />
            </div>
            <p v-if="editError !== null" class="wizard-row__error" role="alert">{{ editError }}</p>
            <textarea
              v-model="detailsDraft"
              class="dg-input wizard-row__input"
              rows="3"
              maxlength="400"
              :placeholder="t('timeline.manage2.detailsPlaceholder')"
              @keydown="onDetailsKeydown"
            ></textarea>
            <div class="wizard-row__archetype-field">
              <span class="wizard-row__archetype-label">{{ t('presets.archetype.label') }}</span>
              <div class="wizard-row__archetypes" role="radiogroup" :aria-label="t('presets.archetype.label')">
                <button
                  v-for="archetype in CATEGORY_ARCHETYPES"
                  :key="archetype"
                  type="button"
                  role="radio"
                  class="wizard-row__archetype"
                  :class="{ 'is-selected': archetypeDraft === archetype }"
                  :aria-checked="archetypeDraft === archetype"
                  @click="archetypeDraft = archetype"
                >{{ t(`presets.archetype.${archetype}`) }}</button>
              </div>
              <span class="wizard-row__archetype-hint">{{ t('presets.archetype.hint') }}</span>
            </div>
            <div class="wizard-row__edit-actions">
              <button type="button" class="dg-button dg-button--sm" @click="cancelEdit">{{ t('common.action.cancel') }}</button>
              <button type="button" class="dg-button dg-button--sm dg-button--primary" @click="commitEdit">{{ t('common.action.save') }}</button>
            </div>
          </template>
          <template v-else>
            <button type="button" class="wizard-row__body" @click="beginEdit(index)">
              <i class="wizard-row__dot" :style="{ background: safeCategoryColor(row.colorHex) }" aria-hidden="true"></i>
              <span class="wizard-row__text">
                <strong>
                  <span class="wizard-row__name">{{ displayName(row) || t('timeline.manage2.namePlaceholder') }}</span>
                  <em class="wizard-row__tag">{{ t(`presets.archetype.${row.archetype}`) }}</em>
                </strong>
                <span v-if="displayDetails(row) !== ''" class="wizard-row__details">{{ displayDetails(row) }}</span>
              </span>
            </button>
            <div class="wizard-row__actions">
              <button type="button" class="wizard-row__icon" :aria-label="t('timeline.inspector.editTitle')" @click="beginEdit(index)">
                <DgIcon name="pencil" :size="14" />
              </button>
              <button type="button" class="wizard-row__icon wizard-row__icon--danger" :aria-label="t('timeline.manage2.deleteTitle')" @click="removeCategory(index)">
                <DgIcon name="trash" :size="14" />
              </button>
            </div>
          </template>
        </div>
        <p v-if="draft.length === 0" class="wizard__optional">{{ t('timeline.manage2.empty') }}</p>
        <button type="button" class="dg-button dg-button--block wizard__add" @click="addCategory">
          <DgIcon name="plusCircle" :size="14" />
          {{ t('timeline.manage2.add') }}
        </button>
      </div>

      <div v-else class="wizard__list">
        <button
          v-for="(row, index) in draft"
          :key="row.id || `new-${index}`"
          type="button"
          class="wizard-drop-card"
          :class="{ 'is-targeted': targetedIndex === index, 'is-pickable': pickedColor !== null }"
          :tabindex="pickedColor !== null ? 0 : -1"
          @click="onCardClick(index)"
          @dragover="onCardDragOver(index, $event)"
          @dragleave="onCardDragLeave(index, $event)"
          @drop="onCardDrop(index, $event)"
        >
          <i class="wizard-drop-card__swatch" :style="{ background: safeCategoryColor(row.colorHex) }"></i>
          <span class="wizard__row-text">
            <strong>{{ displayName(row) || t('timeline.manage2.namePlaceholder') }}</strong>
            <span v-if="displayDetails(row) !== ''">{{ displayDetails(row) }}</span>
          </span>
        </button>
        <p v-if="draft.length === 0" class="wizard__optional">{{ t('timeline.manage2.empty') }}</p>
        <p v-else class="wizard__optional">{{ t('timeline.manage2.colorNote') }}</p>
      </div>
    </section>

    <footer class="wizard__footer">
      <p v-if="error !== null" class="wizard__error" role="alert">{{ error }}</p>
      <div class="wizard__footer-actions">
        <button v-if="step === 2" type="button" class="dg-button" @click="goTo(1)">{{ t('timeline.manage2.back') }}</button>
        <button v-if="step === 1" type="button" class="dg-button dg-button--primary wizard__primary" @click="goTo(2)">
          {{ t('timeline.manage2.next') }}
          <DgIcon name="chevronRight" :size="11" />
        </button>
        <button v-else type="button" class="dg-button dg-button--primary wizard__primary" :disabled="saving || !canWrite" @click="finish">
          {{ saving ? t('timeline.failure.retrying') : t('timeline.manage2.finish') }}
          <DgIcon name="chevronRight" :size="11" />
        </button>
      </div>
    </footer>

    <Transition name="wizard-confirm">
      <div v-if="confirmingClose" class="wizard__confirm-layer" @click.self="confirmingClose = false">
        <div class="wizard__confirm" role="alertdialog" :aria-label="t('timeline.manage2.discardTitle')">
          <p class="wizard__confirm-title">{{ t('timeline.manage2.discardTitle') }}</p>
          <div class="wizard__confirm-actions">
            <button type="button" class="dg-button dg-button--sm" @click="confirmingClose = false">{{ t('timeline.manage2.keepEditing') }}</button>
            <button type="button" class="dg-button dg-button--sm dg-button--danger" @click="emit('close')">{{ t('timeline.manage2.discard') }}</button>
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
/* Header (progress + close) / side + main / footer. The close button sits in
   the header row, so it never overlaps the category list beside it. */
.wizard {
  position: relative;
  display: grid;
  grid-template-areas:
    'header header'
    'side main'
    'footer footer';
  grid-template-columns: 272px minmax(0, 1fr);
  grid-template-rows: auto minmax(0, 1fr) auto;
  column-gap: 32px;
  width: min(960px, calc(100vw - 64px));
  height: min(660px, calc(100vh - 64px));
  padding: 16px 20px 18px 32px;
  border-radius: 16px;
  background: var(--dg-popover-fill, var(--dg-surface));
  box-shadow: var(--lg-shadow-dense, var(--dg-shadow-lg));
  outline: none;
}

/* ---- Header ---- */
.wizard__header {
  grid-area: header;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 14px;
}

.wizard__progress {
  display: inline-flex;
  align-items: center;
  gap: 12px;
}

.wizard__step {
  color: var(--dg-accent-text);
  font-size: 12px;
  font-weight: 650;
}

.wizard__bars { display: inline-flex; gap: 2px; }

/* A 4px bar inside a taller hit area. */
.wizard__bar {
  position: relative;
  width: 32px;
  height: 16px;
  padding: 0;
  border: none;
  background: transparent;
  cursor: pointer;
}

.wizard__bar::before {
  position: absolute;
  top: 6px;
  right: 2px;
  left: 2px;
  height: 4px;
  border-radius: 999px;
  background: var(--dg-control-fill);
  content: '';
  transition: background var(--dg-motion-base) var(--dg-ease-out);
}

.wizard__bar.is-active::before { background: var(--dg-accent); }
.wizard__bar:hover:not(.is-active)::before { background: color-mix(in srgb, var(--dg-accent) 35%, var(--dg-control-fill)); }
.wizard__bar:focus-visible { outline: none; }
.wizard__bar:focus-visible::before { box-shadow: 0 0 0 3px var(--dg-focus-ring); }

.wizard__close {
  display: grid;
  flex: none;
  width: 30px;
  height: 30px;
  place-items: center;
  border: none;
  border-radius: 50%;
  background: transparent;
  color: var(--dg-text-secondary);
  cursor: pointer;
  transition: background var(--dg-motion-fast) ease, color var(--dg-motion-fast) ease;
}

.wizard__close:hover { background: var(--dg-hover-fill); color: var(--dg-text-primary); }
.wizard__close:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }

/* ---- Side ---- */
.wizard__side {
  grid-area: side;
  display: flex;
  flex-direction: column;
  min-height: 0;
  padding: 4px 4px 4px 0;
  overflow-y: auto;
}

.wizard__title {
  margin: 0 0 18px;
  color: var(--dg-text-primary);
  font-size: 22px;
  font-weight: 700;
  line-height: 1.25;
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

/* Canvas + swatch grid, mirroring the reference color stage: the canvas sets
   the palette, the eight swatches below it are dragged (or clicked, then the
   card clicked) onto a category. */
.wizard__canvas { margin: 0 auto 14px; }

.wizard__swatches {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
}

.wizard__swatch {
  position: relative;
  height: 34px;
  padding: 0;
  border: none;
  border-radius: 8px;
  box-shadow: inset 0 0 0 2px rgba(255, 255, 255, 0.9), 0 1px 3px rgba(35, 39, 60, 0.18);
  cursor: grab;
  transition: transform var(--dg-motion-fast) var(--dg-ease-out), box-shadow var(--dg-motion-fast) ease;
}

.wizard__swatch:hover { transform: translateY(-2px); }
.wizard__swatch:active { cursor: grabbing; }
.wizard__swatch.is-picked {
  transform: translateY(-2px);
  box-shadow: inset 0 0 0 2px rgba(255, 255, 255, 0.9), 0 0 0 2px var(--dg-accent), 0 4px 10px rgba(35, 39, 60, 0.18);
}
.wizard__swatch:focus-visible { outline: none; box-shadow: inset 0 0 0 2px #ffffff, 0 0 0 3px var(--dg-focus-ring); }

.wizard__swatch-hint {
  position: absolute;
  bottom: calc(100% + 6px);
  left: 50%;
  padding: 4px 8px;
  border-radius: 4px;
  background: rgba(15, 15, 20, 0.8);
  color: #ffffff;
  font-size: 11px;
  white-space: nowrap;
  opacity: 0;
  transform: translateX(-50%);
  transition: opacity var(--dg-motion-fast) ease;
  pointer-events: none;
}

.wizard__swatch:hover .wizard__swatch-hint { opacity: 1; }

/* ---- Main list ---- */
.wizard__main {
  grid-area: main;
  min-height: 0;
  padding: 8px;
  overflow-y: auto;
  border: 1px solid var(--dg-timeline-grid);
  border-radius: 14px;
  background: color-mix(in srgb, var(--dg-track-fill) 45%, transparent);
  scrollbar-gutter: stable;
}

.wizard__list { display: grid; gap: 6px; }

.wizard-row {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 4px 6px 4px 4px;
  border: 1px solid var(--dg-timeline-grid);
  border-radius: 10px;
  background: var(--dg-timeline-card-fill);
  transition: border-color var(--dg-motion-fast) ease, box-shadow var(--dg-motion-fast) ease;
}

.wizard-row:not(.is-editing):hover { border-color: color-mix(in srgb, var(--dg-accent) 28%, var(--dg-timeline-grid)); }

.wizard-row.is-editing {
  flex-direction: column;
  align-items: stretch;
  gap: 10px;
  padding: 14px 16px;
  border-color: color-mix(in srgb, var(--dg-accent) 45%, var(--dg-timeline-grid));
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--dg-accent) 12%, transparent), 0 6px 18px rgba(35, 39, 60, 0.08);
}

.wizard-row__body {
  display: flex;
  flex: 1;
  align-items: center;
  gap: 12px;
  min-width: 0;
  padding: 10px 12px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.wizard-row__body:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }

.wizard-row__dot {
  flex: none;
  width: 10px;
  height: 10px;
  border-radius: 50%;
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--dg-timeline-card-fill) 70%, transparent);
}

.wizard-row__text {
  display: grid;
  flex: 1;
  gap: 3px;
  min-width: 0;
}

.wizard-row__text strong {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  color: var(--dg-text-primary);
  font-size: 13px;
  font-weight: 650;
}

.wizard-row__name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.wizard-row__details {
  display: -webkit-box;
  overflow: hidden;
  color: var(--dg-text-secondary);
  font-size: 11px;
  line-height: 1.45;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.wizard-row__tag {
  flex: none;
  padding: 1px 6px;
  border-radius: 999px;
  background: var(--dg-control-fill);
  color: var(--dg-text-muted);
  font-size: 10px;
  font-style: normal;
  font-weight: 550;
}

/* Muted until the row is hovered, never hidden: delete stays discoverable. */
.wizard-row__actions {
  display: inline-flex;
  flex: none;
  gap: 2px;
  opacity: 0.45;
  transition: opacity var(--dg-motion-fast) ease;
}

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
  color: var(--dg-text-secondary);
  cursor: pointer;
  transition: background var(--dg-motion-fast) ease, color var(--dg-motion-fast) ease;
}

.wizard-row__icon:hover { background: color-mix(in srgb, var(--dg-accent) 12%, transparent); color: var(--dg-accent-text); }
.wizard-row__icon:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }
.wizard-row__icon svg { width: 14px; height: 14px; }
.wizard-row__icon--danger:hover { background: color-mix(in srgb, var(--dg-danger) 10%, transparent); color: var(--dg-danger); }

/* Edit mode. */
.wizard-row__edit-head {
  display: flex;
  align-items: center;
  gap: 10px;
}

.wizard-row__input { width: 100%; }
textarea.wizard-row__input { resize: vertical; min-height: 64px; line-height: 1.5; }
.wizard-row__input.is-invalid { box-shadow: inset 0 0 0 1px var(--dg-danger); }

.wizard-row__error {
  margin: -4px 0 0 20px;
  color: var(--dg-danger);
  font-size: 11px;
}

.wizard-row__edit-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.wizard-row__archetype-field {
  display: grid;
  gap: 6px;
}

.wizard-row__archetype-label {
  color: var(--dg-text-secondary);
  font-size: 11px;
  font-weight: 600;
}

.wizard-row__archetypes {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 4px;
  justify-self: start;
  padding: 3px;
  border-radius: 999px;
  background: var(--dg-control-fill);
}

.wizard-row__archetype {
  padding: 4px 12px;
  border: none;
  border-radius: 999px;
  background: transparent;
  color: var(--dg-text-secondary);
  font-size: 12px;
  font-weight: 550;
  cursor: pointer;
  transition: background var(--dg-motion-fast) ease, color var(--dg-motion-fast) ease, box-shadow var(--dg-motion-fast) ease;
}

.wizard-row__archetype:hover { color: var(--dg-text-primary); }
.wizard-row__archetype:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }

.wizard-row__archetype.is-selected {
  background: var(--dg-surface);
  color: var(--dg-accent-text);
  box-shadow: 0 1px 3px rgba(35, 39, 60, 0.14);
}

.wizard-row__archetype-hint {
  color: var(--dg-text-muted);
  font-size: 11px;
}

.wizard__add { margin-top: 2px; }
.wizard__add svg { color: var(--dg-accent-text); }

/* Step 2 cards: drop targets for a dragged swatch, click targets once a
   swatch is picked. */
.wizard-drop-card {
  display: flex;
  align-items: center;
  gap: 14px;
  width: 100%;
  padding: 14px 18px;
  border: 1px solid var(--dg-timeline-grid);
  border-radius: 10px;
  background: var(--dg-timeline-card-fill);
  box-shadow: 0 1px 2px rgba(35, 39, 60, 0.06);
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: default;
  transition: border-color var(--dg-motion-fast) ease, box-shadow var(--dg-motion-fast) ease;
}

.wizard-drop-card.is-pickable { cursor: copy; }
.wizard-drop-card.is-pickable:hover { border-color: color-mix(in srgb, var(--dg-accent) 50%, var(--dg-timeline-grid)); }

.wizard-drop-card.is-targeted {
  border-color: var(--dg-accent);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--dg-accent) 26%, transparent);
}

.wizard-drop-card:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }

.wizard-drop-card__swatch {
  flex: none;
  width: 18px;
  height: 18px;
  border-radius: 6px;
  box-shadow: inset 0 0 0 1.5px rgba(255, 255, 255, 0.9), 0 1px 3px rgba(35, 39, 60, 0.18);
  transition: background var(--dg-motion-base) var(--dg-ease-out);
}

.wizard__row-text {
  display: grid;
  flex: 1;
  gap: 3px;
  min-width: 0;
}

.wizard__row-text strong { color: var(--dg-text-primary); font-size: 13px; font-weight: 650; }
.wizard__row-text span {
  display: -webkit-box;
  overflow: hidden;
  color: var(--dg-text-secondary);
  font-size: 11px;
  line-height: 1.45;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.wizard__optional {
  margin: 6px 4px 2px;
  color: var(--dg-text-muted);
  font-size: 11px;
}

/* ---- Footer ---- */
.wizard__footer {
  grid-area: footer;
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 16px;
}

.wizard__error {
  flex: 1;
  min-width: 0;
  margin: 0;
  color: var(--dg-danger);
  font-size: 12px;
}

.wizard__footer-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-left: auto;
}

.wizard__primary { min-width: 104px; }
.wizard__primary svg { width: 11px; height: 11px; }

/* ---- Discard confirmation ---- */
.wizard__confirm-layer {
  position: absolute;
  inset: 0;
  z-index: 2;
  display: grid;
  place-items: center;
  border-radius: inherit;
  background: color-mix(in srgb, var(--dg-popover-fill, var(--dg-surface)) 55%, transparent);
  backdrop-filter: blur(2px);
}

.wizard__confirm {
  display: grid;
  gap: 14px;
  min-width: 280px;
  padding: 18px 20px;
  border: 1px solid var(--dg-timeline-grid);
  border-radius: 14px;
  background: var(--dg-surface);
  box-shadow: var(--dg-shadow-lg);
}

.wizard__confirm-title {
  margin: 0;
  color: var(--dg-text-primary);
  font-size: 14px;
  font-weight: 650;
}

.wizard__confirm-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.wizard-confirm-enter-active,
.wizard-confirm-leave-active { transition: opacity var(--dg-motion-fast) ease; }
.wizard-confirm-enter-from,
.wizard-confirm-leave-to { opacity: 0; }

/* Narrow windows: the side column stacks above the list. */
@media (max-width: 760px) {
  .wizard {
    grid-template-areas:
      'header'
      'side'
      'main'
      'footer';
    grid-template-columns: minmax(0, 1fr);
    grid-template-rows: auto auto minmax(0, 1fr) auto;
    width: calc(100vw - 32px);
    height: calc(100vh - 32px);
    padding: 14px 16px 16px;
  }

  .wizard__side { max-height: 46vh; margin-bottom: 14px; }
}

@media (prefers-reduced-motion: reduce) {
  .wizard__swatch,
  .wizard__swatch:hover,
  .wizard__swatch.is-picked { transform: none; transition: box-shadow var(--dg-motion-fast) ease; }
}
</style>
