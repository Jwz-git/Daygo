<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

/*
 * Minimal month-grid date picker for jumping the timeline to a calendar day.
 * The 04:00 logical-day boundary stays backend-owned: this picker only
 * produces a yyyy-MM-dd key that the caller sends back to GetDayContext.
 * All arithmetic runs on UTC calendar dates so daylight-saving shifts in the
 * host zone cannot move a cell off by a day.
 */
const props = defineProps<{ selected: string }>()

const emit = defineEmits<{
  select: [day: string]
  close: []
}>()

const { locale, t } = useI18n()

const rootEl = ref<HTMLElement | null>(null)

const ISO_PATTERN = /^(\d{4})-(\d{2})-(\d{2})$/

function parseIso(value: string): Date | null {
  const match = ISO_PATTERN.exec(value)
  if (match === null) return null
  const date = new Date(Date.UTC(Number(match[1]), Number(match[2]) - 1, Number(match[3])))
  return Number.isNaN(date.getTime()) ? null : date
}

function toIso(date: Date): string {
  const month = String(date.getUTCMonth() + 1).padStart(2, '0')
  const day = String(date.getUTCDate()).padStart(2, '0')
  return `${date.getUTCFullYear()}-${month}-${day}`
}

function todayIso(): string {
  const now = new Date()
  return toIso(new Date(Date.UTC(now.getFullYear(), now.getMonth(), now.getDate())))
}

/** The month the grid shows, as the UTC midnight of its 1st. */
const monthStart = ref<Date>(parseIso(props.selected) ?? new Date(Date.UTC(0, 0)))

const monthTitle = computed(() =>
  new Intl.DateTimeFormat(locale.value, { month: 'long', year: 'numeric', timeZone: 'UTC' })
    .format(monthStart.value),
)

interface DayCell {
  iso: string
  label: number
  inMonth: boolean
  isToday: boolean
  isSelected: boolean
}

const cells = computed<DayCell[]>(() => {
  const first = monthStart.value
  // Monday-first offset: getUTCDay() is 0 (Sunday)…6, so Sunday maps to 6.
  const offset = (first.getUTCDay() + 6) % 7
  const gridStart = new Date(first)
  gridStart.setUTCDate(gridStart.getUTCDate() - offset)

  const selectedDate = parseIso(props.selected)
  const today = todayIso()
  const result: DayCell[] = []
  for (let index = 0; index < 42; index += 1) {
    const date = new Date(gridStart)
    date.setUTCDate(gridStart.getUTCDate() + index)
    const iso = toIso(date)
    result.push({
      iso,
      label: date.getUTCDate(),
      inMonth: date.getUTCMonth() === first.getUTCMonth(),
      isToday: iso === today,
      isSelected: selectedDate !== null && iso === toIso(selectedDate),
    })
  }
  return result
})

const weekdayLabels = computed(() => {
  const format = new Intl.DateTimeFormat(locale.value, { weekday: 'narrow', timeZone: 'UTC' })
  // 2023-01-02 is a Monday; the following six days complete the row.
  return Array.from({ length: 7 }, (_, index) =>
    format.format(new Date(Date.UTC(2023, 0, 2 + index))),
  )
})

function shiftMonth(offset: -1 | 1): void {
  const next = new Date(monthStart.value)
  next.setUTCMonth(next.getUTCMonth() + offset, 1)
  monthStart.value = next
}

function choose(cell: DayCell): void {
  emit('select', cell.iso)
}

function onPointerDown(event: PointerEvent): void {
  if (rootEl.value !== null && event.target instanceof Node && !rootEl.value.contains(event.target)) {
    emit('close')
  }
}

function onKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape') emit('close')
}

onMounted(() => {
  document.addEventListener('pointerdown', onPointerDown)
  document.addEventListener('keydown', onKeydown)
})
onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', onPointerDown)
  document.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <div ref="rootEl" class="calendar" role="dialog" :aria-label="t('timeline.calendar.open')">
    <div class="calendar__head">
      <button type="button" class="calendar__nav" :title="t('timeline.calendar.prevMonth')" @click="shiftMonth(-1)">
        <svg viewBox="0 0 12 12" aria-hidden="true"><path d="M7.5 2.5 4 6l3.5 3.5" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" /></svg>
      </button>
      <span class="calendar__month">{{ monthTitle }}</span>
      <button type="button" class="calendar__nav" :title="t('timeline.calendar.nextMonth')" @click="shiftMonth(1)">
        <svg viewBox="0 0 12 12" aria-hidden="true"><path d="M4.5 2.5 8 6l-3.5 3.5" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" /></svg>
      </button>
    </div>
    <div class="calendar__grid">
      <span v-for="(label, index) in weekdayLabels" :key="index" class="calendar__weekday">{{ label }}</span>
      <button
        v-for="cell in cells"
        :key="cell.iso"
        type="button"
        class="calendar__day"
        :class="{
          'is-outside': !cell.inMonth,
          'is-today': cell.isToday && !cell.isSelected,
          'is-selected': cell.isSelected,
        }"
        @click="choose(cell)"
      >
        {{ cell.label }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.calendar {
  position: absolute;
  top: calc(100% + 8px);
  left: 0;
  z-index: 60;
  width: 248px;
  padding: 12px;
  border-radius: 12px;
  background: var(--dg-popover-fill, var(--dg-glass-fallback));
  box-shadow: var(--lg-shadow-dense, var(--dg-shadow-lg));
  -webkit-backdrop-filter: blur(var(--lg-blur-dense, 18px)) saturate(var(--lg-saturation-dense, 1.4));
  backdrop-filter: blur(var(--lg-blur-dense, 18px)) saturate(var(--lg-saturation-dense, 1.4));
}

.calendar__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.calendar__month {
  color: var(--dg-text-primary);
  font-size: 12px;
  font-weight: 600;
}

.calendar__nav {
  display: grid;
  width: 24px;
  height: 24px;
  place-items: center;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--dg-text-secondary);
  cursor: pointer;
}

.calendar__nav:hover { background: var(--dg-hover-fill); }
.calendar__nav:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }
.calendar__nav svg { width: 11px; height: 11px; }

.calendar__grid {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  gap: 2px;
}

.calendar__weekday {
  padding: 3px 0;
  color: var(--dg-text-muted);
  font-size: 10px;
  text-align: center;
}

.calendar__day {
  height: 28px;
  border: none;
  border-radius: 7px;
  background: transparent;
  color: var(--dg-text-primary);
  font-size: 11px;
  cursor: pointer;
}

.calendar__day:hover { background: var(--dg-hover-fill); }
.calendar__day:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }
.calendar__day.is-outside { color: var(--dg-text-muted); opacity: 0.55; }
.calendar__day.is-today { box-shadow: inset 0 0 0 1px var(--dg-chip-border); }
.calendar__day.is-selected {
  background: var(--dg-accent);
  color: #ffffff;
  font-weight: 600;
}
</style>
