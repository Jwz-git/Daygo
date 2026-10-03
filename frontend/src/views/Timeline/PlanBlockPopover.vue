<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import DgIcon from '@/components/DgIcon.vue'
import type { PlanBlockDTO, PlanBlockStatus } from '@/api/dto'
import { categoryLabel } from '@/lib/categoryLabel'
import { useDurationFormat } from '@/lib/duration'

import { safeCategoryColor } from './layout'
import { planPhase } from './planLayout'

/*
 * The plan block's detail, opened from its marker on the day track or its
 * block on the week grid. Teleported beside the clicked element, it shows the
 * block and offers the quick writes (done / skip); editing hands over to the
 * inspector's plan panel, which owns the form.
 */
const props = defineProps<{
  block: PlanBlockDTO
  anchor: DOMRect
  canWrite: boolean
  pending: boolean
}>()

const emit = defineEmits<{
  close: []
  status: [status: PlanBlockStatus]
  edit: []
}>()

const { t } = useI18n()
const duration = useDurationFormat()
const root = ref<HTMLElement | null>(null)
const position = ref<Record<string, string>>({ visibility: 'hidden' })

const WIDTH = 288
const GAP = 10
const MARGIN = 10

const nowTs = Math.floor(Date.now() / 1000)
const phase = computed(() => planPhase(props.block, nowTs))
const color = computed(() => safeCategoryColor(props.block.colorHex || undefined))
const started = computed(() => props.block.startTs <= nowTs)

function place(): void {
  const height = root.value?.offsetHeight ?? 200
  const rect = props.anchor
  // Beside the anchor, right first; a wide anchor (a week block near the
  // right edge) falls back to its left side, then to overlapping it.
  let left = rect.right + GAP
  if (left + WIDTH > window.innerWidth - MARGIN) left = rect.left - GAP - WIDTH
  if (left < MARGIN) left = Math.min(window.innerWidth - MARGIN - WIDTH, Math.max(MARGIN, rect.left + 12))
  const top = Math.min(Math.max(MARGIN, rect.top - 6), window.innerHeight - MARGIN - height)
  position.value = { left: `${left}px`, top: `${Math.max(MARGIN, top)}px`, width: `${WIDTH}px` }
}

// Plan anchors (day-track markers, week lines) are left to their own
// click, which toggles this block or switches to another in one step.
const PLAN_ANCHORS = '.plan-marker, .week__plan-rail'

function onPointerDown(event: PointerEvent): void {
  if (!(event.target instanceof Element)) return
  if (root.value?.contains(event.target) || event.target.closest(PLAN_ANCHORS) !== null) return
  emit('close')
}

function onKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape') emit('close')
}

function onScroll(event: Event): void {
  if (root.value !== null && event.target instanceof Node && root.value.contains(event.target)) return
  emit('close')
}

onMounted(async () => {
  await nextTick()
  place()
  root.value?.focus()
  document.addEventListener('pointerdown', onPointerDown, true)
  document.addEventListener('keydown', onKeydown)
  window.addEventListener('scroll', onScroll, true)
  window.addEventListener('resize', place)
})

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', onPointerDown, true)
  document.removeEventListener('keydown', onKeydown)
  window.removeEventListener('scroll', onScroll, true)
  window.removeEventListener('resize', place)
})
</script>

<template>
  <Teleport to="body">
    <div
      ref="root"
      class="plan-pop dg-popover"
      :class="`is-${phase}`"
      :style="{ ...position, '--plan-color': color }"
      role="dialog"
      :aria-label="block.title"
      tabindex="-1"
    >
      <header class="plan-pop__head">
        <span class="plan-pop__time">{{ block.start }}–{{ block.end }}</span>
        <span class="plan-pop__phase">{{ t(`timeline.plan.phase.${phase}`) }}</span>
        <button type="button" class="plan-pop__close" :aria-label="t('common.action.close')" @click="emit('close')">
          <DgIcon name="close" :size="12" />
        </button>
      </header>

      <p class="plan-pop__title">{{ block.title }}</p>
      <p v-if="block.categoryName" class="plan-pop__category">
        <i aria-hidden="true"></i>{{ categoryLabel(block.categoryName, t) }}
      </p>
      <p v-if="block.notes" class="plan-pop__notes dg-scroll">{{ block.notes }}</p>

      <div v-if="started && (block.categoryName || block.distractionMinutes >= 1)" class="plan-pop__stats">
        <span v-if="block.categoryName">
          <strong>{{ duration(block.matchedMinutes) }}</strong>{{ t('timeline.plan.recordedLabel') }}
        </span>
        <span v-if="block.distractionMinutes >= 1" class="plan-pop__distracted">
          <strong>{{ duration(block.distractionMinutes) }}</strong>{{ t('timeline.plan.distractedLabel') }}
        </span>
      </div>

      <footer v-if="canWrite" class="plan-pop__actions">
        <button
          type="button"
          class="plan-pop__action plan-pop__action--primary"
          :disabled="pending"
          @click="emit('status', block.status === 'done' ? 'planned' : 'done')"
        >
          <DgIcon :name="block.status === 'done' ? 'undo' : 'check'" :size="12" />
          {{ block.status === 'done' ? t('timeline.plan.markPlanned') : t('timeline.plan.markDone') }}
        </button>
        <button
          type="button"
          class="plan-pop__action"
          :disabled="pending"
          @click="emit('status', block.status === 'skipped' ? 'planned' : 'skipped')"
        >
          {{ block.status === 'skipped' ? t('timeline.plan.restore') : t('timeline.plan.skip') }}
        </button>
        <button type="button" class="plan-pop__action" @click="emit('edit')">
          <DgIcon name="pencil" :size="12" />
          {{ t('common.action.edit') }}
        </button>
      </footer>
    </div>
  </Teleport>
</template>

<style scoped>
.plan-pop {
  position: fixed;
  z-index: 1000;
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 12px 14px 12px 16px;
  border-radius: 12px;
  outline: none;
  --dg-popover-origin: 0 0;
  --dg-popover-from: translateX(-4px);
}

/* The block's colour as a rail down the left edge. */
.plan-pop::before {
  position: absolute;
  top: 12px;
  bottom: 12px;
  left: 6px;
  width: 3px;
  border-radius: 2px;
  background: var(--plan-color);
  content: '';
}

.plan-pop.is-done::before { background: var(--dg-plan-done); }
.plan-pop.is-skipped::before { background: var(--dg-timeline-grid-strong); }

.plan-pop__head {
  display: flex;
  align-items: center;
  gap: 8px;
}

.plan-pop__time {
  color: var(--dg-text-secondary);
  font-size: 12px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.plan-pop__phase {
  padding: 1px 7px;
  border-radius: 999px;
  background: var(--dg-hover-fill);
  color: var(--dg-text-secondary);
  font-size: 10.5px;
  font-weight: 600;
}

.is-active .plan-pop__phase { background: var(--dg-accent); color: #ffffff; }
.is-done .plan-pop__phase { background: color-mix(in srgb, var(--dg-plan-done) 16%, transparent); color: var(--dg-plan-done); }
.is-missed .plan-pop__phase { background: color-mix(in srgb, var(--dg-warning) 14%, transparent); color: var(--dg-warning); }

.plan-pop__close {
  display: grid;
  width: 22px;
  height: 22px;
  margin-left: auto;
  padding: 0;
  border: none;
  border-radius: 50%;
  background: none;
  color: var(--dg-text-muted);
  cursor: pointer;
  place-items: center;
}

.plan-pop__close:hover { background: var(--dg-hover-fill); color: var(--dg-text-primary); }

.plan-pop__title {
  margin: 0;
  color: var(--dg-text-primary);
  font-size: 15px;
  font-weight: 650;
  line-height: 1.35;
  overflow-wrap: anywhere;
}

.is-skipped .plan-pop__title,
.is-done .plan-pop__title {
  color: var(--dg-text-secondary);
  text-decoration: line-through;
  text-decoration-color: color-mix(in srgb, currentColor 45%, transparent);
}

.plan-pop__category {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin: 0;
  color: var(--dg-text-secondary);
  font-size: 12px;
}

.plan-pop__category i {
  width: 8px;
  height: 8px;
  border-radius: 3px;
  background: var(--plan-color);
}

.plan-pop__notes {
  max-height: 120px;
  margin: 2px 0 0;
  padding: 8px 10px;
  border-radius: 8px;
  background: var(--dg-hover-fill);
  color: var(--dg-text-secondary);
  font-size: 12px;
  line-height: 1.55;
  white-space: pre-line;
  overflow-wrap: anywhere;
}

.plan-pop__stats {
  display: flex;
  gap: 14px;
  color: var(--dg-text-muted);
  font-size: 11px;
}

.plan-pop__stats strong {
  margin-right: 4px;
  color: var(--dg-text-primary);
  font-size: 13px;
  font-weight: 650;
}

.plan-pop__distracted strong { color: var(--dg-danger); }

.plan-pop__actions {
  display: flex;
  gap: 6px;
  margin-top: 6px;
  padding-top: 10px;
  border-top: 1px solid var(--dg-timeline-grid);
}

.plan-pop__action {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  min-height: 28px;
  padding: 4px 10px;
  border: none;
  border-radius: 7px;
  background: var(--dg-hover-fill);
  color: var(--dg-text-primary);
  font-size: 12px;
  font-weight: 550;
  cursor: pointer;
  transition: background-color var(--dg-motion-fast) ease;
}

.plan-pop__action:hover:not(:disabled) { background: var(--dg-hover-fill-strong); }
.plan-pop__action:disabled { opacity: 0.55; cursor: default; }
.plan-pop__action:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }

.plan-pop__action--primary {
  background: var(--dg-button-primary-fill);
  color: var(--dg-button-primary-text);
}

.plan-pop__action--primary:hover:not(:disabled) { background: var(--dg-button-primary-hover); }
</style>
