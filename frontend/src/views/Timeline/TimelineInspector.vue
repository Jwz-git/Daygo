<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import LiquidGlassSurface from '@/components/LiquidGlassSurface.vue'
import type { DayGoalDTO, TimelineCardDTO, TimelineDayDTO, TimelineFailureDTO } from '@/api/dto'
import type { TimelineActionAvailability } from '@/api/timeline'
import type { TimelineAction } from '@/stores/timeline'
import type { ReviewTotals } from './review'

import InspectorCardDetail from './InspectorCardDetail.vue'
import InspectorDayOverview from './InspectorDayOverview.vue'
import InspectorFailureDetail from './InspectorFailureDetail.vue'

/*
 * The inspector shell dispatches between three panes — day overview (no
 * selection), failure detail, card detail — and carries the visual language
 * they share (header, sections, action row) as :deep rules, so the panes stay
 * visually one component while owning their own state.
 */
const props = defineProps<{
  day: TimelineDayDTO
  timeZone: string
  card: TimelineCardDTO | null
  failure: TimelineFailureDTO | null
  canWrite: boolean
  actions: TimelineActionAvailability
  pendingAction: TimelineAction | null
  actionFailed: boolean
  goal: DayGoalDTO | null
  goalUnavailable: boolean
  goalFailed: boolean
  goalSaving: boolean
  reviewTotals: ReviewTotals
}>()

const emit = defineEmits<{
  close: []
  saveEdits: [cardID: number, edits: { title?: string; category?: string; summary?: string; detailedSummary?: string }]
  delete: [cardID: number]
  retry: [batchIDs: number[]]
  dismissFailure: [batchIDs: number[]]
  reprocess: []
  reprocessCard: [cardID: number]
  saveGoal: [goal: DayGoalDTO]
  verdictChanged: []
}>()

const { t } = useI18n()
</script>

<template>
  <LiquidGlassSurface intensity="glass" as="aside" class="inspector" :aria-label="t('timeline.inspector.title')">
    <InspectorDayOverview
      v-if="props.card === null && props.failure === null"
      :day="props.day"
      :can-write="props.canWrite"
      :actions="props.actions"
      :pending-action="props.pendingAction"
      :goal="props.goal"
      :goal-unavailable="props.goalUnavailable"
      :goal-failed="props.goalFailed"
      :goal-saving="props.goalSaving"
      :review-totals="props.reviewTotals"
      @retry="(batchIDs) => emit('retry', batchIDs)"
      @save-goal="(goal) => emit('saveGoal', goal)"
      @reprocess="emit('reprocess')"
    />

    <InspectorFailureDetail
      v-else-if="props.failure !== null"
      :failure="props.failure"
      :can-write="props.canWrite"
      :actions="props.actions"
      :pending-action="props.pendingAction"
      :action-failed="props.actionFailed"
      @close="emit('close')"
      @retry="(batchIDs) => emit('retry', batchIDs)"
      @dismiss="(batchIDs) => emit('dismissFailure', batchIDs)"
    />

    <Transition v-else-if="props.card !== null" name="card-swap" mode="out-in">
      <div :key="props.card.id" class="card-swap-wrap">
        <InspectorCardDetail
          :day="props.day"
          :time-zone="props.timeZone"
          :card="props.card"
          :can-write="props.canWrite"
          :actions="props.actions"
          :pending-action="props.pendingAction"
          :action-failed="props.actionFailed"
          @close="emit('close')"
          @save-edits="(cardID: number, edits: { title?: string; category?: string; summary?: string; detailedSummary?: string }) => emit('saveEdits', cardID, edits)"
          @delete="(cardID) => emit('delete', cardID)"
          @reprocess="(cardID) => emit('reprocessCard', cardID)"
          @verdict-changed="emit('verdictChanged')"
        />
      </div>
    </Transition>
  </LiquidGlassSurface>
</template>

<style scoped>
.inspector {
  min-height: 0;
  padding: 22px;
  overflow-y: auto;
  background: var(--dg-timeline-inspector-fill);
}

/*
 * Shared inspector language, applied into the panes with :deep. Anything used
 * by a single pane lives in that pane instead.
 */
.inspector :deep(.inspector__header) {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding-bottom: 18px;
  border-bottom: 1px solid var(--dg-timeline-grid);
}

.inspector :deep(.inspector__eyebrow) {
  margin-bottom: 5px;
  color: var(--dg-accent-text);
  font-size: 10px;
  font-weight: 600;
}

.inspector :deep(.inspector__eyebrow--danger) { color: var(--dg-danger); }

.inspector :deep(.inspector__title) {
  color: var(--dg-text-primary);
  font-size: 26px;
  line-height: 1.1;
}

.inspector :deep(.inspector__title--card) {
  color: var(--dg-text-primary);
  font-size: 20px;
  font-weight: 700;
  line-height: 1.35;
}

.inspector :deep(.inspector__close) {
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

.inspector :deep(.inspector__close:hover) { background: var(--dg-hover-fill); }

.inspector :deep(.card-time) {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 14px 0;
  color: var(--dg-text-secondary);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}

.inspector :deep(.card-time span) { width: 7px; height: 7px; border-radius: 50%; }

.inspector :deep(.inspector__section) { padding: 16px 0; border-top: 1px solid var(--dg-timeline-grid); }
.inspector :deep(.inspector__section:first-of-type) { border-top: 0; }
.inspector :deep(.inspector__section h3) { margin-bottom: 6px; color: var(--dg-text-primary); font-size: 11px; font-weight: 650; }
.inspector :deep(.inspector__section p) { color: var(--dg-text-primary); font-size: 13px; font-weight: 500; line-height: 1.65; }

.inspector :deep(.inspector__actions) { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; padding-top: 18px; border-top: 1px solid var(--dg-timeline-grid); }
.inspector :deep(.inspector__readonly) { width: 100%; color: var(--dg-text-muted); font-size: 10px; }
.inspector :deep(.inspector__confirm) { width: 100%; color: var(--dg-text-secondary); font-size: 11px; }

.inspector :deep(.inspector__failure-note) { margin-top: 8px; color: var(--dg-text-muted); font-size: 11px; }

.inspector :deep(.inspector__danger) { color: var(--dg-danger); }

.inspector :deep(.inspector__danger:not(:disabled):hover) {
  background: color-mix(in srgb, var(--dg-danger) 9%, transparent);
}

.inspector :deep(.inspector__delete) { border-color: color-mix(in srgb, var(--dg-danger) 34%, transparent); color: var(--dg-danger); }
.inspector :deep(.inspector__error) { margin: 4px 0 12px; color: var(--dg-danger); font-size: 11px; }

/* Switching between cards: the outgoing detail shrinks away, the incoming
   one grows in — same language as the review flow's card switch. */
.card-swap-wrap { display: flex; flex-direction: column; min-height: 0; }

.card-swap-enter-active {
  transition:
    opacity 220ms ease,
    transform 280ms var(--dg-ease-glide);
}

.card-swap-leave-active {
  transition:
    opacity 160ms ease,
    transform 200ms ease;
}

.card-swap-enter-from {
  opacity: 0;
  transform: scale(0.94);
}

.card-swap-leave-to {
  opacity: 0;
  transform: scale(0.94);
}
</style>
