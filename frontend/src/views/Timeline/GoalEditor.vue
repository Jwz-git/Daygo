<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import type { CategoryDTO, DayGoalDTO } from '@/api/dto'
import { categoryLabel } from '@/lib/categoryLabel'
import { useDurationFormat } from '@/lib/duration'

import GoalIcon from './GoalIcon.vue'
import {
  DEFAULT_DISTRACTION_LIMIT_MINUTES,
  DEFAULT_FOCUS_TARGET_MINUTES,
  defaultGoalCategories,
} from './goalProgress'
import GoalWheel from './GoalWheel.vue'
import { safeCategoryColor } from './layout'

/*
 * The day-goal form, after Dayflow's goal sheet (DayGoalSetupComponents):
 * a pool of categories that cycle untracked → focus → distraction on click
 * (or drag into a panel), then one tinted panel per goal holding its
 * categories and an hours / minutes wheel. Kept compact for the inspector:
 * no stats footer, and the panels stack instead of sitting side by side.
 */
const props = defineProps<{
  goal: DayGoalDTO | null
  categories: CategoryDTO[]
  saving: boolean
}>()

const emit = defineEmits<{
  save: [goal: DayGoalDTO]
}>()

const { t } = useI18n()
const duration = useDurationFormat()

type GoalKind = 'focus' | 'distraction'
type CategoryStatus = GoalKind | 'untracked'

interface Draft {
  focusTargetMinutes: number
  distractionLimitMinutes: number
  isSkipped: boolean
  focusCategories: string[]
  distractionCategories: string[]
}

/** Dayflow caps either goal at 12 hours. */
const MAX_MINUTES = 12 * 60

// A day with no goal yet starts from Dayflow's default plan: every category
// but Distraction is focus, 4.5 h of focus and a 2 h distraction budget.
function fromGoal(goal: DayGoalDTO | null): Draft {
  if (goal === null || !goal.exists) {
    const defaults = defaultGoalCategories(props.categories)
    return {
      focusTargetMinutes: DEFAULT_FOCUS_TARGET_MINUTES,
      distractionLimitMinutes: DEFAULT_DISTRACTION_LIMIT_MINUTES,
      isSkipped: false,
      focusCategories: defaults.focus,
      distractionCategories: defaults.distraction,
    }
  }
  return {
    focusTargetMinutes: goal.focusTargetMinutes,
    distractionLimitMinutes: goal.distractionLimitMinutes,
    isSkipped: goal.isSkipped,
    focusCategories: goal.focusCategories.map((ref) => ref.categoryId),
    distractionCategories: goal.distractionCategories.map((ref) => ref.categoryId),
  }
}

const draft = ref<Draft>(fromGoal(props.goal))
watch(() => props.goal, (next) => { draft.value = fromGoal(next) })
// Categories can arrive after the goal; an unsaved default re-derives from them.
watch(
  () => props.categories.map((category) => category.id).join(','),
  () => { if (props.goal === null || !props.goal.exists) draft.value = fromGoal(props.goal) },
)

// An unsaved goal is always saveable, so the defaults can be accepted as-is.
const dirty = computed(() =>
  props.goal === null || !props.goal.exists
    || JSON.stringify(draft.value) !== JSON.stringify(fromGoal(props.goal)),
)

// System categories are excluded: they never carry user meaning as a target.
const selectableCategories = computed(() =>
  props.categories.filter((category) => !category.isSystem),
)

function categoryById(id: string): CategoryDTO | undefined {
  return selectableCategories.value.find((category) => category.id === id)
}

function statusOf(id: string): CategoryStatus {
  if (draft.value.focusCategories.includes(id)) return 'focus'
  if (draft.value.distractionCategories.includes(id)) return 'distraction'
  return 'untracked'
}

// A category is focus, distraction, or neither — never both.
function assign(id: string, status: CategoryStatus): void {
  draft.value.focusCategories = draft.value.focusCategories.filter((item) => item !== id)
  draft.value.distractionCategories = draft.value.distractionCategories.filter((item) => item !== id)
  if (status === 'focus') draft.value.focusCategories.push(id)
  if (status === 'distraction') draft.value.distractionCategories.push(id)
}

const CYCLE: Record<CategoryStatus, CategoryStatus> = {
  untracked: 'focus',
  focus: 'distraction',
  distraction: 'untracked',
}

function cycle(id: string): void {
  assign(id, CYCLE[statusOf(id)])
}

// ---- Drag and drop -------------------------------------------------------

const DRAG_TYPE = 'application/x-daygo-category'
const dropTarget = ref<GoalKind | 'pool' | null>(null)

function onDragStart(event: DragEvent, id: string): void {
  event.dataTransfer?.setData(DRAG_TYPE, id)
  if (event.dataTransfer !== null) event.dataTransfer.effectAllowed = 'move'
}

function onDragOver(event: DragEvent, target: GoalKind | 'pool'): void {
  if (!event.dataTransfer?.types.includes(DRAG_TYPE)) return
  event.preventDefault()
  dropTarget.value = target
}

function onDragLeave(event: DragEvent, target: GoalKind | 'pool'): void {
  const next = event.relatedTarget
  if (next instanceof Node && (event.currentTarget as HTMLElement).contains(next)) return
  if (dropTarget.value === target) dropTarget.value = null
}

function onDrop(event: DragEvent, target: GoalKind | 'pool'): void {
  dropTarget.value = null
  const id = event.dataTransfer?.getData(DRAG_TYPE) ?? ''
  if (categoryById(id) === undefined) return
  event.preventDefault()
  assign(id, target === 'pool' ? 'untracked' : target)
}

// ---- Panels --------------------------------------------------------------

interface Panel {
  kind: GoalKind
  title: string
  minutes: number
  ids: string[]
}

const panels = computed<Panel[]>(() => [
  { kind: 'focus', title: t('daily.goal.focusTarget'), minutes: draft.value.focusTargetMinutes, ids: draft.value.focusCategories },
  { kind: 'distraction', title: t('daily.goal.distractionLimit'), minutes: draft.value.distractionLimitMinutes, ids: draft.value.distractionCategories },
])

function setMinutes(kind: GoalKind, minutes: number): void {
  const clamped = Math.min(MAX_MINUTES, Math.max(0, minutes))
  if (kind === 'focus') draft.value.focusTargetMinutes = clamped
  else draft.value.distractionLimitMinutes = clamped
}

function setHours(kind: GoalKind, current: number, hours: number): void {
  setMinutes(kind, hours * 60 + (current % 60))
}

function setMins(kind: GoalKind, current: number, mins: number): void {
  setMinutes(kind, Math.floor(current / 60) * 60 + mins)
}

const canSave = computed(() => !props.saving && props.goal !== null && dirty.value)

function submit(): void {
  if (!canSave.value) return
  const toRefs = (ids: string[]) =>
    ids.map((id) => {
      const category = categoryById(id)
      return {
        categoryId: id,
        name: category?.name ?? '',
        colorHex: category?.colorHex ?? '',
        sortOrder: category?.sortOrder ?? 0,
      }
    })
  emit('save', {
    day: props.goal?.day ?? '',
    focusTargetMinutes: draft.value.focusTargetMinutes,
    distractionLimitMinutes: draft.value.distractionLimitMinutes,
    isSkipped: draft.value.isSkipped,
    focusCategories: toRefs(draft.value.focusCategories),
    distractionCategories: toRefs(draft.value.distractionCategories),
    exists: true,
  })
}
</script>

<template>
  <div class="goal" :class="{ 'is-skipped': draft.isSkipped }">
    <div
      v-if="selectableCategories.length > 0"
      class="goal-pool"
      :class="{ 'is-drop': dropTarget === 'pool' }"
      @dragover="onDragOver($event, 'pool')"
      @dragleave="onDragLeave($event, 'pool')"
      @drop="onDrop($event, 'pool')"
    >
      <p class="goal-pool__hint">{{ t('daily.goal.poolHint') }}</p>
      <div class="goal-pool__chips">
        <button
          v-for="category in selectableCategories"
          :key="category.id"
          type="button"
          class="goal-chip"
          :class="`is-${statusOf(category.id)}`"
          :style="{ '--chip-color': safeCategoryColor(category.colorHex) }"
          draggable="true"
          :aria-label="`${categoryLabel(category.name, t)} · ${t(`daily.goal.status.${statusOf(category.id)}`)}`"
          @click="cycle(category.id)"
          @dragstart="onDragStart($event, category.id)"
        >
          <span class="goal-chip__grip" aria-hidden="true"><i></i><i></i><i></i><i></i></span>
          {{ categoryLabel(category.name, t) }}
        </button>
      </div>
    </div>
    <p v-else class="goal-empty">{{ t('daily.goal.noCategories') }}</p>

    <section
      v-for="panel in panels"
      :key="panel.kind"
      class="goal-panel"
      :class="[`goal-panel--${panel.kind}`, { 'is-drop': dropTarget === panel.kind }]"
      @dragover="onDragOver($event, panel.kind)"
      @dragleave="onDragLeave($event, panel.kind)"
      @drop="onDrop($event, panel.kind)"
    >
      <header class="goal-panel__head">
        <GoalIcon :kind="panel.kind" :size="15" />
        <h4>{{ panel.title }}</h4>
        <span class="goal-panel__total">{{ panel.minutes > 0 ? duration(panel.minutes) : '—' }}</span>
      </header>
      <div class="goal-panel__body">
        <div class="goal-panel__cats" :aria-label="t(`daily.goal.${panel.kind}Categories`)">
          <span class="goal-panel__label">{{ t('daily.goal.categories') }}</span>
          <button
            v-for="id in panel.ids.filter((item) => categoryById(item) !== undefined)"
            :key="id"
            type="button"
            class="goal-chip goal-chip--assigned"
            :style="{ '--chip-color': safeCategoryColor(categoryById(id)?.colorHex) }"
            draggable="true"
            :title="t('daily.goal.remove')"
            @click="assign(id, 'untracked')"
            @dragstart="onDragStart($event, id)"
          >
            <span class="goal-chip__grip" aria-hidden="true"><i></i><i></i><i></i><i></i></span>
            <span class="goal-chip__name">{{ categoryLabel(categoryById(id)?.name ?? '', t) }}</span>
            <span class="goal-chip__x" aria-hidden="true">×</span>
          </button>
          <span v-if="panel.ids.length === 0" class="goal-panel__drop">{{ t('daily.goal.dropHint') }}</span>
        </div>
        <div class="goal-panel__wheels">
          <GoalWheel
            :model-value="Math.floor(panel.minutes / 60)"
            :min="0"
            :max="12"
            :step="1"
            :label="t('daily.goal.hours')"
            @update:model-value="(hours) => setHours(panel.kind, panel.minutes, hours)"
          />
          <GoalWheel
            :model-value="panel.minutes % 60"
            :min="0"
            :max="59"
            :step="5"
            pad
            :label="t('daily.goal.mins')"
            @update:model-value="(mins) => setMins(panel.kind, panel.minutes, mins)"
          />
        </div>
      </div>
    </section>

    <footer class="goal-foot">
      <label class="goal-skip">
        <input v-model="draft.isSkipped" class="dg-checkbox" type="checkbox">
        <span>{{ t('daily.goal.skipped') }}</span>
      </label>
      <button
        type="button"
        class="dg-button dg-button--primary goal-save"
        :disabled="!canSave"
        @click="submit"
      >
        {{ saving ? t('daily.goal.saving') : dirty ? t('daily.goal.save') : t('daily.goal.saved') }}
      </button>
    </footer>
  </div>
</template>

<style scoped>
.goal {
  display: grid;
  gap: 10px;
}

/* ---- Category pool ---- */
.goal-pool {
  padding: 10px 11px 11px;
  border-radius: 10px;
  background: var(--dg-goal-box-fill);
  box-shadow: inset 0 0 0 1px var(--dg-goal-box-border);
  transition: box-shadow var(--dg-motion-fast) ease;
}

.goal-pool.is-drop { box-shadow: inset 0 0 0 1.5px var(--dg-accent); }

.goal-pool__hint {
  margin: 0 0 8px !important;
  color: var(--dg-text-secondary) !important;
  font-size: 11.5px !important;
  font-weight: 450 !important;
  line-height: 1.45 !important;
}

.goal-pool__chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

/* Dayflow GoalCategoryChip: 4-dot grip in the category colour, tinted fill,
   hairline border; untracked chips stay quiet, tracked ones pick up their
   goal colour as a ring. */
.goal-chip {
  --chip-color: var(--dg-accent);
  display: inline-flex;
  align-items: center;
  gap: 3px;
  max-width: 100%;
  padding: 3px 8px 3px 4px;
  border: none;
  border-radius: 7px;
  background: color-mix(in srgb, var(--chip-color) 22%, transparent);
  box-shadow: inset 0 0 0 0.5px color-mix(in srgb, var(--chip-color) 80%, transparent);
  color: var(--dg-text-primary);
  font-size: 12px;
  line-height: 18px;
  cursor: grab;
  transition: box-shadow var(--dg-motion-fast) ease, opacity var(--dg-motion-fast) ease, transform var(--dg-motion-fast) ease;
}

.goal-chip:active { cursor: grabbing; transform: scale(0.97); }
.goal-chip:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }

.goal-pool .goal-chip.is-untracked { opacity: 0.72; }
.goal-pool .goal-chip.is-focus { box-shadow: inset 0 0 0 0.5px var(--chip-color), 0 0 0 2px var(--dg-goal-focus); }
.goal-pool .goal-chip.is-distraction { box-shadow: inset 0 0 0 0.5px var(--chip-color), 0 0 0 2px var(--dg-goal-distraction); }

.goal-chip__grip {
  display: grid;
  flex: none;
  grid-template-columns: repeat(2, 3px);
  gap: 2px;
  padding: 3px;
}

.goal-chip__grip i {
  width: 3px;
  height: 3px;
  border-radius: 50%;
  background: var(--chip-color);
}

.goal-chip__name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.goal-chip__x {
  margin-left: 2px;
  color: var(--dg-text-muted);
  font-size: 13px;
  line-height: 1;
}

.goal-chip--assigned:hover .goal-chip__x { color: var(--dg-text-primary); }

/* ---- Goal panels ---- */
.goal-panel {
  --goal-color: var(--dg-goal-focus);
  --goal-text: var(--dg-goal-focus-text);
  overflow: hidden;
  border-radius: 10px;
  background: var(--dg-goal-box-fill);
  box-shadow: inset 0 0 0 1px var(--dg-goal-box-border);
  transition: box-shadow var(--dg-motion-fast) ease, opacity var(--dg-motion-base) ease;
}

.goal-panel--distraction {
  --goal-color: var(--dg-goal-distraction);
  --goal-text: var(--dg-goal-distraction-text);
}

.goal-panel.is-drop { box-shadow: inset 0 0 0 1.5px var(--goal-color), 0 4px 14px color-mix(in srgb, var(--goal-color) 22%, transparent); }

.goal.is-skipped .goal-panel,
.goal.is-skipped .goal-pool { opacity: 0.45; }

.goal-panel__head {
  display: flex;
  align-items: center;
  gap: 6px;
  height: 30px;
  padding: 0 11px;
  background: var(--goal-color);
  color: #ffffff;
}

.goal-panel__head h4 {
  margin: 0;
  font-size: 13px;
  font-weight: 600;
}

.goal-panel__total {
  margin-left: auto;
  font-size: 12px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  opacity: 0.92;
}

.goal-panel__body {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 150px;
  gap: 8px;
  padding: 10px;
}

.goal-panel__cats {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 5px;
  min-width: 0;
  padding: 8px;
  border-radius: 7px;
  background: var(--dg-goal-box-fill);
  box-shadow: inset 0 0 0 1px var(--dg-goal-box-border);
}

.goal-panel__label {
  color: var(--dg-text-secondary);
  font-size: 11px;
  font-weight: 550;
}

.goal-panel__drop {
  color: var(--dg-text-muted);
  font-size: 11px;
  line-height: 1.45;
}

.goal-panel__wheels {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 5px;
}

/* ---- Footer ---- */
.goal-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding-top: 2px;
}

.goal-skip {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  color: var(--dg-text-secondary);
  font-size: 12px;
  cursor: pointer;
}

.goal-save {
  min-height: 30px;
  padding: 5px 14px;
  font-size: 12px;
}

.goal-empty {
  margin: 0;
  color: var(--dg-text-muted) !important;
  font-size: 12px !important;
}

@media (prefers-reduced-motion: reduce) {
  .goal-chip,
  .goal-panel { transition: none; }
}
</style>
