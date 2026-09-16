<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import type { WeekColumn } from './weekLayout'
import AppSiteIcon from '@/components/AppSiteIcon.vue'
import GeneratingCard from '@/components/GeneratingCard.vue'

/*
 * Thin renderer for the week grid; all column math lives in weekLayout.ts so
 * it stays unit-testable. Each column's cards are placed by that day's own
 * 04:00→04:00 window from GetTimelineDay — the frontend never recomputes the
 * logical-day boundary.
 *
 * Clicking a card opens its detail in the inspector (same pane as the day
 * view); clicking a day header only re-anchors the week, it never switches
 * back to the day view — mode changes belong to the header toggle alone.
 */
const props = defineProps<{
  dayKeys: string[]
  columns: WeekColumn[]
  selectedDay: string
  selectedCardId: number | null
  weekLoading: boolean
  /** Placeholder state on the now-line: off, live capture, or paused hold. */
  generating: 'off' | 'capturing' | 'paused'
}>()

const emit = defineEmits<{
  anchorDay: [day: string]
  selectCard: [day: string, cardId: number]
}>()

const { locale } = useI18n()

/*
 * Hover stretches a card downward over its neighbours to reveal the whole
 * title. Deliberately no transform: scaling is what made the previous expand
 * jitter — height and shadow alone animate smoothly.
 */
const expandedId = ref<number | null>(null)
const expandedHeight = ref(0)

function onCardEnter(event: MouseEvent, card: { id: number; height: number }): void {
  const el = event.currentTarget as HTMLElement | null
  if (el === null) return
  expandedId.value = card.id
  // scrollHeight cannot see past -webkit-line-clamp: the clamped-away lines
  // never take part in layout. Release the clamp synchronously (style applies
  // immediately) so the measurement covers every line; the reactive binding
  // keeps it at 'none' for as long as the card stays expanded.
  const text = el.querySelector<HTMLElement>('.week__card-text')
  if (text !== null) text.style.setProperty('-webkit-line-clamp', 'none')
  expandedHeight.value = Math.max(card.height, el.scrollHeight + 8)
}

function onCardLeave(id: number): void {
  if (expandedId.value === id) expandedId.value = null
}

const headerOf = (day: string): { weekday: string; number: string } | null => {
  const parsed = /^(\d{4})-(\d{2})-(\d{2})$/.exec(day)
  if (parsed === null) return null
  const date = new Date(Date.UTC(Number(parsed[1]), Number(parsed[2]) - 1, Number(parsed[3])))
  const parts = new Intl.DateTimeFormat(locale.value, { timeZone: 'UTC' }).formatToParts(date)
  const weekday = new Intl.DateTimeFormat(locale.value, { weekday: 'short', timeZone: 'UTC' }).format(date)
  const number = parts.find((part) => part.type === 'day')?.value ?? ''
  return { weekday, number }
}

const todayKey = computed(() => {
  const now = new Date()
  const month = String(now.getMonth() + 1).padStart(2, '0')
  const day = String(now.getDate()).padStart(2, '0')
  return `${now.getFullYear()}-${month}-${day}`
})

/* Live now-marker position inside today's column. */
const nowTs = ref(Math.floor(Date.now() / 1000))
let nowTimer: number | null = null

const generatingMark = computed(() => {
  if (!props.generating) return null
  const index = props.dayKeys.indexOf(todayKey.value)
  const column = props.columns[index]
  if (index === -1 || column === undefined || !column.hasData) return null
  const span = column.windowEndTs - column.windowStartTs
  if (span <= 0 || nowTs.value < column.windowStartTs || nowTs.value >= column.windowEndTs) return null
  return {
    columnIndex: index,
    top: ((nowTs.value - column.windowStartTs) / span) * column.height,
  }
})

onMounted(() => {
  nowTimer = window.setInterval(() => { nowTs.value = Math.floor(Date.now() / 1000) }, 15_000)
})
onBeforeUnmount(() => {
  if (nowTimer !== null) window.clearInterval(nowTimer)
})
</script>

<template>
  <div class="week-scroll dg-scroll" :aria-label="$t('timeline.week.aria')">
    <div class="week">
      <div class="week__corner" aria-hidden="true"></div>
      <button
        v-for="(key, index) in dayKeys"
        :key="key || index"
        type="button"
        class="week__head"
        :class="{ 'is-today': key === todayKey, 'is-selected': key === selectedDay }"
        :disabled="key === ''"
        @click="emit('anchorDay', key)"
      >
        <template v-if="headerOf(key) !== null">
          <span>{{ headerOf(key)!.weekday }}</span>
          <strong>{{ headerOf(key)!.number }}</strong>
        </template>
      </button>

      <div class="week__gutter" aria-hidden="true">
        <template v-for="(column, index) in columns" :key="index">
          <span
            v-for="mark in column.hourMarks"
            :key="`${index}-${mark.top}`"
            :style="{ top: `${mark.top}px` }"
          >{{ mark.label }}</span>
        </template>
      </div>

      <div
        v-for="(column, index) in columns"
        :key="index"
        class="week__col"
        :style="{ height: `${column.height}px` }"
      >
        <i
          v-for="mark in column.hourMarks"
          :key="mark.top"
          class="week__line"
          :style="{ top: `${mark.top}px` }"
          aria-hidden="true"
        ></i>

        <div
          v-for="range in column.processing"
          :key="`p-${range.top}`"
          class="week__generating"
          :style="{ top: `${range.top}px`, height: `${range.height}px` }"
          role="status"
        >
          {{ $t('timeline.generating') }}
        </div>

        <GeneratingCard
          v-if="generatingMark?.columnIndex === index"
          :state="generating"
          :style="{
            top: `${generatingMark.top + 4}px`,
            right: '4px',
            left: '4px',
            position: 'absolute',
          }"
        />

        <p v-if="weekLoading && !column.hasData" class="week__pending">{{ $t('timeline.week.loading') }}</p>

        <button
          v-for="card in column.cards"
          :key="`${index}-${card.id}`"
          type="button"
          class="week__card"
          :class="{ 'is-selected': card.id === selectedCardId, 'is-expanded': expandedId === card.id }"
          :style="{
            top: `${card.top}px`,
            height: `${expandedId === card.id ? expandedHeight : card.height}px`,
            background: `color-mix(in srgb, ${card.color} 16%, var(--dg-timeline-card-fill))`,
            borderLeftColor: card.color,
          }"
          :title="card.title"
          @mouseenter="onCardEnter($event, card)"
          @mouseleave="onCardLeave(card.id)"
          @click="emit('selectCard', column.day, card.id)"
        >
          <span class="week__card-head">
            <AppSiteIcon
              v-if="card.site !== null"
              :site="card.site"
              :accent="card.color"
              :size="13"
            />
            <span
              class="week__card-text"
              :style="{ '-webkit-line-clamp': expandedId === card.id ? 'none' : card.clampLines }"
            >{{ card.title }}</span>
          </span>
        </button>      </div>
    </div>
  </div>
</template>

<style scoped>
.week-scroll {
  min-height: 0;
  border-radius: var(--dg-panel-radius);
  background: var(--dg-card-fill);
  overflow-y: auto;
}

.week {
  display: grid;
  grid-template-columns: 44px repeat(7, minmax(0, 1fr));
}

.week__corner {
  position: sticky;
  top: 0;
  z-index: 3;
  border-bottom: 1px solid var(--dg-timeline-grid);
  background: var(--dg-card-fill);
}

.week__head {
  position: sticky;
  top: 0;
  z-index: 3;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  padding: 9px 4px;
  border: none;
  border-bottom: 1px solid var(--dg-timeline-grid);
  background: var(--dg-card-fill);
  color: var(--dg-text-secondary);
  font-size: 11px;
  cursor: pointer;
}

.week__head:hover:not(:disabled) { background: var(--dg-hover-fill); }
.week__head:focus-visible { outline: none; box-shadow: inset 0 0 0 3px var(--dg-focus-ring); }

.week__head strong {
  display: grid;
  width: 20px;
  height: 20px;
  place-items: center;
  border-radius: 50%;
  font-size: 11px;
  font-weight: 600;
}

.week__head.is-today strong { background: #e8804a; color: #ffffff; }
.week__head.is-selected { color: var(--dg-text-primary); }

.week__gutter {
  position: relative;
  border-right: 1px solid var(--dg-timeline-grid);
}

.week__gutter span {
  position: absolute;
  right: 8px;
  transform: translateY(-50%);
  color: var(--dg-text-muted);
  font-size: 10px;
  white-space: nowrap;
}

.week__col {
  position: relative;
  min-width: 0;
  border-right: 1px solid var(--dg-timeline-grid);
}

.week__col:last-child { border-right: none; }

.week__line {
  position: absolute;
  left: 0;
  right: 0;
  border-top: 1px dashed var(--dg-timeline-grid);
}

.week__pending {
  position: sticky;
  top: 120px;
  margin: 0;
  color: var(--dg-text-muted);
  font-size: 10px;
  text-align: center;
}

.week__generating {
  position: absolute;
  right: 4px;
  left: 4px;
  display: flex;
  align-items: center;
  overflow: hidden;
  padding: 3px 7px;
  border-radius: 6px;
  background: linear-gradient(100deg, color-mix(in srgb, var(--dg-accent) 26%, transparent), color-mix(in srgb, #e8804a 20%, transparent));
  color: var(--dg-text-primary);
  font-size: 10px;
  white-space: nowrap;
}

.week__card {
  position: absolute;
  right: 2px;
  left: 2px;
  /* Flex column kills the <button>'s built-in vertical centering: content
     hangs from the card top. */
  display: flex;
  flex-direction: column;
  align-items: stretch;
  justify-content: flex-start;
  overflow: hidden;
  min-height: 34px;
  padding: 8px 10px;
  border: 1px solid var(--dg-timeline-card-border);
  border-left: 3px solid;
  border-radius: 4px;
  background: var(--dg-timeline-card-fill);
  box-shadow: var(--dg-timeline-card-shadow);
  cursor: pointer;
  text-align: left;
  /* Only height and shadow animate. left/right must NOT: the hover state
     widens the box, and an animated width re-wraps the title on every frame
     of the transition — that continuous re-breaking is what read as jitter. */
  transition:
    height 320ms var(--dg-ease-glide),
    box-shadow var(--dg-motion-base) ease;
}

/* Hovered: a gentle downward stretch floating above the neighbours. No
   width change, no transform — both jittered or felt exaggerated. */
.week__card.is-expanded {
  z-index: 6;
  box-shadow: var(--dg-timeline-card-shadow-hover);
}

.week__card:hover {
  background: var(--dg-timeline-card-hover);
}

.week__card:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring), var(--dg-timeline-card-shadow); }

.week__card.is-selected { background: var(--dg-timeline-card-selected); }

.week__card-head {
  display: flex;
  align-items: flex-start;
  gap: 5px;
}

/* Freeze the leading icon only. Matching :first-child hit the title text on
   icon-less cards: flex:none refused to shrink, so a long title rendered as
   one unbreakable line spilling across the whole grid. */
.week__card-head > .app-site-icon {
  flex: none;
  margin-top: 1px;
}

.week__card-text {
  /* Flex children default to min-width:auto and refuse to shrink below the
     one-line content width, so a long title rendered as a single 700px line
     spilling across the whole grid. Zero it so the column width wins and the
     clamp can wrap and ellipsize. */
  min-width: 0;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  overflow: hidden;
  color: var(--dg-text-primary);
  font-size: 11px;
  font-weight: 550;
  line-height: 1.4;
  overflow-wrap: anywhere;
}
</style>
