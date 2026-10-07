<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import DgIcon from '@/components/DgIcon.vue'
import type { CategoryDTO, PlanBlockDTO, PlanBlockInputDTO } from '@/api/dto'
import { categoryLabel } from '@/lib/categoryLabel'
import { useDurationFormat } from '@/lib/duration'
import { usePlanStore } from '@/stores/plan'
import { useCapabilitiesStore } from '@/stores/capabilities'
import { safeCategoryColor } from './layout'
import PlanBlockForm, { type PlanDraft } from './PlanBlockForm.vue'
import { planPhase } from './planLayout'

/*
 * "Today's plan" in the timeline inspector (docs/modules/plan.md): the day's
 * time blocks with editable title, notes and category, a done toggle, and the
 * recorded / distracted minutes so far. Data, writes and the plan:updated
 * subscription live in the plan store, which TimelineView loads for the
 * displayed day; this component only presents and edits. Times are HH:mm;
 * Go places them in the logical day.
 */
const props = defineProps<{
  day: string
  categories: CategoryDTO[]
  canWrite: boolean
  /** The displayed day is the live day: new blocks default to the next half hour. */
  live: boolean
}>()

const { t } = useI18n()
const duration = useDurationFormat()
const plan = usePlanStore()
const capabilities = useCapabilitiesStore()

const nowTs = ref(Math.floor(Date.now() / 1000))
let clock: number | null = null
onMounted(() => {
  void capabilities.load()
  clock = window.setInterval(() => { nowTs.value = Math.floor(Date.now() / 1000) }, 30_000)
})
onBeforeUnmount(() => {
  if (clock !== null) window.clearInterval(clock)
})

function phaseOf(block: PlanBlockDTO) {
  return planPhase(block, nowTs.value)
}

const doneCount = computed(() => plan.blocks.filter((block) => block.status === 'done').length)
const progress = computed(() => (plan.blocks.length === 0 ? 0 : doneCount.value / plan.blocks.length))

function hasStarted(block: PlanBlockDTO): boolean {
  return block.startTs <= nowTs.value
}

// ---- Editor --------------------------------------------------------------

const draft = ref<PlanDraft | null>(null)
const pendingDelete = ref<number | null>(null)

function clockOf(minutes: number): string {
  const wrapped = ((minutes % 1440) + 1440) % 1440
  return `${String(Math.floor(wrapped / 60)).padStart(2, '0')}:${String(wrapped % 60).padStart(2, '0')}`
}

// A new block starts at the next half hour on the live day (local wall
// clock only), or at 09:00 on any other day.
function newDraft(): PlanDraft {
  let start = 9 * 60
  if (props.live) {
    const now = new Date()
    start = Math.ceil((now.getHours() * 60 + now.getMinutes() + 1) / 30) * 30
  }
  return { id: 0, start: clockOf(start), end: clockOf(start + 60), title: '', notes: '', categoryId: '', remind: capabilities.notificationsAvailable }
}

function openNew(): void {
  plan.writeError = null
  pendingDelete.value = null
  draft.value = newDraft()
}

function openEdit(block: PlanBlockDTO): void {
  plan.writeError = null
  pendingDelete.value = null
  draft.value = {
    id: block.id,
    start: block.start,
    end: block.end,
    title: block.title,
    notes: block.notes ?? '',
    categoryId: block.categoryId,
    remind: block.remind,
  }
}

async function submit(current: PlanDraft): Promise<void> {
  const input: PlanBlockInputDTO = {
    id: current.id,
    day: props.day,
    start: current.start,
    end: current.end,
    title: current.title,
    notes: current.notes.trim() === '' ? null : current.notes,
    categoryId: current.categoryId,
    remind: current.remind,
  }
  if (await plan.save(input)) draft.value = null
}

async function toggleDone(block: PlanBlockDTO): Promise<void> {
  await plan.setStatus(block.id, block.status === 'done' ? 'planned' : 'done')
}

async function confirmDelete(id: number): Promise<void> {
  if (await plan.remove(id)) pendingDelete.value = null
}

/*
 * A plan block clicked on the track or week grid: scroll its row into view
 * and flash it, opening the form when asked. A request for a day still
 * loading waits until its blocks arrive.
 */
const flashID = ref<number | null>(null)
let handledNonce = 0
let flashTimer: number | null = null

watch(
  () => [plan.focusRequest, plan.blocks] as const,
  async ([request]) => {
    if (request === null || request.nonce === handledNonce) return
    const block = plan.blocks.find((item) => item.id === request.id)
    if (block === undefined) return
    handledNonce = request.nonce
    if (request.edit && props.canWrite) openEdit(block)
    flashID.value = block.id
    if (flashTimer !== null) window.clearTimeout(flashTimer)
    flashTimer = window.setTimeout(() => { flashID.value = null }, 1400)
    await nextTick()
    document.getElementById(`plan-item-${block.id}`)?.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  if (flashTimer !== null) window.clearTimeout(flashTimer)
})

const errorText = computed(() => {
  switch (plan.writeError) {
    case 'invalid': return t('timeline.plan.errorInvalid')
    case 'readonly': return t('timeline.plan.errorReadonly')
    case 'failed': return t('timeline.plan.errorFailed')
    default: return ''
  }
})
</script>

<template>
  <section class="inspector__tile plan">
    <header class="inspector__tile-head">
      <h3 class="inspector__tile-title">{{ t('timeline.plan.title') }}</h3>
      <span v-if="plan.blocks.length > 0" class="inspector__tile-meta">
        {{ t('timeline.plan.progress', { done: doneCount, total: plan.blocks.length }) }}
      </span>
      <button
        v-if="canWrite && plan.available && draft === null"
        type="button"
        class="inspector__tile-action"
        @click="openNew"
      >
        <DgIcon name="plus" :size="11" />
        {{ t('timeline.plan.add') }}
      </button>
    </header>

    <div v-if="plan.blocks.length > 0" class="plan__progress" aria-hidden="true">
      <span :style="{ transform: `scaleX(${progress})` }"></span>
    </div>

    <p v-if="!plan.available" class="inspector__tile-note">{{ t('timeline.plan.unavailable') }}</p>
    <p v-else-if="plan.loadFailed" class="inspector__tile-note">{{ t('timeline.plan.loadFailed') }}</p>

    <template v-else>
      <PlanBlockForm
        v-if="draft !== null && draft.id === 0"
        class="plan__form"
        :initial="draft"
        :categories="categories"
        :pending="plan.pending"
        :error="errorText"
        :notifications-available="capabilities.notificationsAvailable"
        @save="submit"
        @cancel="draft = null"
      />

      <p v-if="plan.blocks.length === 0 && draft === null" class="plan__empty">{{ t('timeline.plan.empty') }}</p>

      <ol class="plan__list">
        <li
          v-for="block in plan.blocks"
          :id="`plan-item-${block.id}`"
          :key="block.id"
          class="plan-item"
          :class="[`is-${phaseOf(block)}`, { 'is-flash': flashID === block.id, 'is-editing': draft?.id === block.id }]"
          :style="{ '--plan-color': safeCategoryColor(block.colorHex || undefined) }"
        >
          <PlanBlockForm
            v-if="draft !== null && draft.id === block.id"
            class="plan__form"
            :initial="draft"
            :categories="categories"
            :pending="plan.pending"
            :error="errorText"
            :notifications-available="capabilities.notificationsAvailable"
            @save="submit"
            @cancel="draft = null"
          />

          <template v-else>
            <button
              type="button"
              class="plan-item__check"
              :disabled="!canWrite || plan.pending"
              :aria-pressed="block.status === 'done'"
              :title="block.status === 'done' ? t('timeline.plan.markPlanned') : t('timeline.plan.markDone')"
              :aria-label="block.status === 'done' ? t('timeline.plan.markPlanned') : t('timeline.plan.markDone')"
              @click="toggleDone(block)"
            >
              <DgIcon name="check" :size="11" />
            </button>
            <div class="plan-item__body">
              <p class="plan-item__line">
                <span class="plan-item__time">{{ block.start }} – {{ block.end }}</span>
                <span v-if="phaseOf(block) !== 'upcoming'" class="plan-item__badge">{{ t(`timeline.plan.phase.${phaseOf(block)}`) }}</span>
              </p>
              <p class="plan-item__title">{{ block.title }}</p>
              <p v-if="block.notes" class="plan-item__notes">{{ block.notes }}</p>
              <p v-if="block.categoryName || (hasStarted(block) && block.distractionMinutes >= 1)" class="plan-item__meta">
                <span v-if="block.categoryName" class="plan-item__category">
                  <i aria-hidden="true"></i>{{ categoryLabel(block.categoryName, t) }}
                </span>
                <span v-if="hasStarted(block) && block.categoryName">{{ t('timeline.plan.recorded', { duration: duration(block.matchedMinutes) }) }}</span>
                <span v-if="hasStarted(block) && block.distractionMinutes >= 1" class="plan-item__distracted">
                  {{ t('timeline.plan.distracted', { duration: duration(block.distractionMinutes) }) }}
                </span>
              </p>
              <div v-if="pendingDelete === block.id" class="plan-item__confirm" role="alert">
                <span>{{ t('timeline.plan.deleteConfirm') }}</span>
                <button type="button" class="dg-button" @click="pendingDelete = null">{{ t('common.action.cancel') }}</button>
                <button type="button" class="dg-button dg-button--danger" :disabled="plan.pending" @click="confirmDelete(block.id)">{{ t('common.action.delete') }}</button>
              </div>
            </div>
            <div v-if="canWrite && pendingDelete !== block.id" class="plan-item__actions">
              <button type="button" class="plan-item__icon" :title="t('common.action.edit')" :aria-label="t('common.action.edit')" @click="openEdit(block)">
                <DgIcon name="pencil" :size="12" />
              </button>
              <button
                type="button"
                class="plan-item__icon"
                :title="block.status === 'skipped' ? t('timeline.plan.restore') : t('timeline.plan.skip')"
                :aria-label="block.status === 'skipped' ? t('timeline.plan.restore') : t('timeline.plan.skip')"
                @click="plan.setStatus(block.id, block.status === 'skipped' ? 'planned' : 'skipped')"
              >
                <DgIcon :name="block.status === 'skipped' ? 'undo' : 'minus'" :size="12" />
              </button>
              <button type="button" class="plan-item__icon plan-item__icon--danger" :title="t('common.action.delete')" :aria-label="t('common.action.delete')" @click="pendingDelete = block.id">
                <DgIcon name="trash" :size="12" />
              </button>
            </div>
          </template>
        </li>
      </ol>

      <p v-if="errorText && draft === null" class="plan__error" role="alert">{{ errorText }}</p>
      <p class="plan__hint">
        <DgIcon name="sparkle" :size="11" />
        <span>{{ t('timeline.plan.agentHint') }}</span>
      </p>
    </template>
  </section>
</template>

<style scoped>
.plan__progress {
  height: 4px;
  margin: -2px 0 10px;
  overflow: hidden;
  border-radius: 2px;
  background: var(--dg-hover-fill);
}

.plan__progress span {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: var(--dg-plan-done);
  transform-origin: left;
  transition: transform var(--dg-motion-slow) var(--dg-ease-glide);
}

.plan__empty {
  margin: 0;
  padding: 14px 12px;
  border: 1px dashed var(--dg-timeline-grid-strong);
  border-radius: 10px;
  color: var(--dg-text-muted);
  font-size: 12px;
  line-height: 1.55;
  text-align: center;
}

.plan__hint {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  margin: 10px 0 0;
  color: var(--dg-text-muted);
  font-size: 11px;
  line-height: 1.5;
}

.plan__hint :deep(svg) { flex: none; margin-top: 2px; }

.plan__error {
  margin: 6px 0 0;
  color: var(--dg-danger);
  font-size: 12px;
  line-height: 1.45;
}

.plan__list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  margin: 0;
  padding: 0;
  list-style: none;
}

/* One row: done circle, body, hover actions; a 3px rail in the block's
   colour marks its left edge, as on the track. */
.plan-item {
  position: relative;
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 9px 8px 9px 14px;
  border-radius: 10px;
  transition: background-color var(--dg-motion-fast) ease;
}

.plan-item::before {
  position: absolute;
  top: 10px;
  bottom: 10px;
  left: 4px;
  width: 3px;
  border-radius: 2px;
  background: var(--plan-color);
  opacity: 0.75;
  content: '';
}

.plan-item:hover { background: var(--dg-hover-fill); }

.plan-item.is-active { background: color-mix(in srgb, var(--dg-accent) 9%, transparent); }
.plan-item.is-active::before { opacity: 1; }
.plan-item.is-done::before { background: var(--dg-plan-done); }
.plan-item.is-skipped::before { background: var(--dg-timeline-grid-strong); }

.plan-item.is-editing {
  padding: 4px 6px 6px 14px;
  background: var(--dg-hover-fill);
}

.plan-item.is-flash { animation: plan-flash 1.4s var(--dg-ease-out); }

@keyframes plan-flash {
  0%, 30% { background: color-mix(in srgb, var(--plan-color) 22%, transparent); }
}

.plan__form { flex: 1; }

.plan-item__check {
  display: inline-grid;
  flex: none;
  width: 18px;
  height: 18px;
  margin-top: 1px;
  padding: 0;
  border: 1.5px solid color-mix(in srgb, var(--dg-text-muted) 70%, transparent);
  border-radius: 50%;
  background: transparent;
  color: transparent;
  cursor: pointer;
  place-items: center;
  transition: background-color var(--dg-motion-fast) ease, border-color var(--dg-motion-fast) ease, color var(--dg-motion-fast) ease;
}

.plan-item__check:hover:not(:disabled) { border-color: var(--dg-plan-done); color: color-mix(in srgb, var(--dg-plan-done) 60%, transparent); }
.plan-item__check:disabled { cursor: default; opacity: 0.6; }
.plan-item__check:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }

.plan-item.is-done .plan-item__check {
  border-color: var(--dg-plan-done);
  background: var(--dg-plan-done);
  color: #ffffff;
}

.plan-item__body {
  flex: 1;
  min-width: 0;
}

.plan-item__body p { margin: 0; }

.plan-item__line {
  display: flex;
  align-items: center;
  gap: 6px;
}

.plan-item__time {
  color: var(--dg-text-secondary);
  font-size: 11.5px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  letter-spacing: 0.01em;
}

.plan-item__badge {
  padding: 0 6px;
  border-radius: 999px;
  background: var(--dg-hover-fill-strong);
  color: var(--dg-text-secondary);
  font-size: 10px;
  font-weight: 650;
  line-height: 16px;
}

.plan-item.is-active .plan-item__badge { background: var(--dg-accent); color: #ffffff; }
.plan-item.is-done .plan-item__badge { background: color-mix(in srgb, var(--dg-plan-done) 15%, transparent); color: var(--dg-plan-done); }
.plan-item.is-missed .plan-item__badge { background: color-mix(in srgb, var(--dg-warning) 14%, transparent); color: var(--dg-warning); }

.plan-item__body .plan-item__title {
  margin-top: 2px;
  color: var(--dg-text-primary);
  font-size: 13.5px;
  font-weight: 600;
  line-height: 1.4;
  overflow-wrap: anywhere;
}

.plan-item.is-done .plan-item__title,
.plan-item.is-skipped .plan-item__title {
  color: var(--dg-text-muted);
  text-decoration: line-through;
  text-decoration-color: color-mix(in srgb, currentColor 50%, transparent);
}

.plan-item__body .plan-item__notes {
  display: -webkit-box;
  margin-top: 3px;
  overflow: hidden;
  color: var(--dg-text-secondary);
  font-size: 12px;
  line-height: 1.5;
  white-space: pre-line;
  overflow-wrap: anywhere;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
}

.plan-item__body .plan-item__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 2px 10px;
  margin-top: 5px;
  color: var(--dg-text-muted);
  font-size: 11px;
  line-height: 1.5;
}

.plan-item__category {
  display: inline-flex;
  align-items: center;
  gap: 5px;
}

.plan-item__category i {
  width: 7px;
  height: 7px;
  border-radius: 2px;
  background: var(--plan-color);
}

.plan-item__distracted { color: var(--dg-danger); }

.plan-item__actions {
  display: flex;
  flex: none;
  gap: 1px;
  margin: -3px -2px 0 0;
  opacity: 0;
  transition: opacity var(--dg-motion-fast) ease;
}

.plan-item:hover .plan-item__actions,
.plan-item:focus-within .plan-item__actions {
  opacity: 1;
}

.plan-item__icon {
  display: inline-grid;
  width: 24px;
  height: 24px;
  padding: 0;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--dg-text-muted);
  cursor: pointer;
  place-items: center;
}

.plan-item__icon:hover { background: var(--dg-hover-fill-strong); color: var(--dg-text-primary); }
.plan-item__icon--danger:hover { background: var(--dg-danger-fill); color: var(--dg-danger); }
.plan-item__icon:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }

.plan-item__confirm {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  margin-top: 8px;
  color: var(--dg-danger);
  font-size: 12px;
}

.plan-item__confirm span { flex: 1; min-width: 8em; }
.plan-item__confirm .dg-button { min-height: 26px; padding: 3px 10px; font-size: 12px; }

.dg-button--danger {
  background: var(--dg-danger);
  box-shadow: none;
  color: #ffffff;
}

@media (prefers-reduced-motion: reduce) {
  .plan-item,
  .plan-item__check,
  .plan-item__actions,
  .plan__progress span { transition: none; }
  .plan-item.is-flash { animation: none; }
}
</style>
