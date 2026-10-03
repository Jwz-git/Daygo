<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'

/*
 * One number wheel of the goal duration (Dayflow GoalNumberColumn): the value
 * sits large in the middle with its neighbours dimmed above and below. Scroll,
 * drag, a click on the upper / lower half, or the arrow keys step it; each
 * step rolls the column one row and settles. A spinbutton to assistive tech.
 */
const props = withDefaults(
  defineProps<{
    modelValue: number
    min: number
    max: number
    step: number
    label: string
    /** Two-digit display (minutes). */
    pad?: boolean
  }>(),
  { pad: false },
)

const emit = defineEmits<{ 'update:modelValue': [value: number] }>()

const ROW = 22
const SCROLL_THRESHOLD = 22

const offset = ref(0)
const settling = ref(false)
let scrollAccumulator = 0
let drag: { y: number; value: number; moved: boolean } | null = null
let settleFrame = 0

const rows = computed(() =>
  [-2, -1, 0, 1, 2].map((delta) => {
    const value = props.modelValue + delta * props.step
    return { delta, value: value < props.min || value > props.max ? null : value }
  }),
)

function format(value: number): string {
  return props.pad ? String(value).padStart(2, '0') : String(value)
}

function clamp(value: number): number {
  return Math.min(props.max, Math.max(props.min, value))
}

function roll(direction: number): void {
  if (window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return
  settling.value = false
  offset.value = direction * ROW
  cancelAnimationFrame(settleFrame)
  settleFrame = requestAnimationFrame(() => {
    settling.value = true
    offset.value = 0
  })
}

function stepBy(delta: number): boolean {
  const next = clamp(props.modelValue + delta)
  if (next === props.modelValue) return false
  emit('update:modelValue', next)
  roll(delta > 0 ? 1 : -1)
  return true
}

function onWheel(event: WheelEvent): void {
  event.preventDefault()
  // Mouse wheels send whole lines; trackpads send pixels and accumulate.
  if (event.deltaMode !== 0) {
    stepBy(event.deltaY > 0 ? props.step : -props.step)
    return
  }
  scrollAccumulator += event.deltaY
  while (Math.abs(scrollAccumulator) >= SCROLL_THRESHOLD) {
    const up = scrollAccumulator > 0
    scrollAccumulator += up ? -SCROLL_THRESHOLD : SCROLL_THRESHOLD
    if (!stepBy(up ? props.step : -props.step)) {
      scrollAccumulator = 0
      break
    }
  }
}

function onPointerDown(event: PointerEvent): void {
  if (event.button !== 0) return
  drag = { y: event.clientY, value: props.modelValue, moved: false }
  ;(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId)
}

function onPointerMove(event: PointerEvent): void {
  if (drag === null) return
  const dy = event.clientY - drag.y
  if (Math.abs(dy) > 3) drag.moved = true
  if (!drag.moved) return
  // Dragging up raises the value, like pulling a wheel.
  const steps = Math.round(-dy / ROW)
  const next = clamp(drag.value + steps * props.step)
  settling.value = false
  const applied = (next - drag.value) / props.step
  // Follow the finger between rows, with resistance at either end.
  const rest = dy + applied * ROW
  offset.value = next === props.min || next === props.max ? rest * 0.35 : rest
  if (next !== props.modelValue) emit('update:modelValue', next)
}

function onPointerUp(event: PointerEvent): void {
  if (drag === null) return
  const wasDrag = drag.moved
  drag = null
  settling.value = true
  offset.value = 0
  if (wasDrag) return
  // A click: upper half steps down, lower half steps up (Dayflow).
  const rect = (event.currentTarget as HTMLElement).getBoundingClientRect()
  stepBy(event.clientY < rect.top + rect.height / 2 ? -props.step : props.step)
}

function onKeydown(event: KeyboardEvent): void {
  const actions: Record<string, () => void> = {
    ArrowUp: () => stepBy(props.step),
    ArrowDown: () => stepBy(-props.step),
    PageUp: () => stepBy(props.step * 3),
    PageDown: () => stepBy(-props.step * 3),
    Home: () => stepBy(props.min - props.modelValue),
    End: () => stepBy(props.max - props.modelValue),
  }
  const action = actions[event.key]
  if (action === undefined) return
  event.preventDefault()
  action()
}

onBeforeUnmount(() => cancelAnimationFrame(settleFrame))
</script>

<template>
  <div
    class="wheel"
    role="spinbutton"
    tabindex="0"
    :aria-label="label"
    :aria-valuenow="modelValue"
    :aria-valuemin="min"
    :aria-valuemax="max"
    :aria-valuetext="`${modelValue} ${label}`"
    @wheel="onWheel"
    @pointerdown="onPointerDown"
    @pointermove="onPointerMove"
    @pointerup="onPointerUp"
    @pointercancel="onPointerUp"
    @keydown="onKeydown"
  >
    <div
      class="wheel__stack"
      :class="{ 'is-settling': settling }"
      :style="{ transform: `translateY(${offset}px)` }"
      aria-hidden="true"
    >
      <span
        v-for="row in rows"
        :key="row.delta"
        class="wheel__row"
        :class="`wheel__row--${Math.abs(row.delta)}`"
      >{{ row.value === null ? '' : format(row.value) }}</span>
    </div>
    <span class="wheel__label" aria-hidden="true">{{ label }}</span>
  </div>
</template>

<style scoped>
.wheel {
  position: relative;
  display: grid;
  height: 118px;
  overflow: hidden;
  border-radius: 8px;
  background: var(--dg-goal-wheel-fill);
  box-shadow: inset 0 0 0 1px var(--dg-goal-box-border);
  cursor: ns-resize;
  touch-action: none;
  user-select: none;
  place-items: center;
}

.wheel:focus-visible {
  outline: none;
  box-shadow: inset 0 0 0 1px var(--dg-goal-box-border), 0 0 0 3px var(--dg-focus-ring);
}

.wheel__stack {
  display: grid;
  justify-items: center;
  width: 100%;
  margin-left: -18px;
}

.wheel__stack.is-settling {
  transition: transform 220ms var(--dg-ease-glide);
}

.wheel__row {
  display: block;
  height: 22px;
  color: var(--dg-goal-wheel-dim);
  font-size: 15px;
  font-weight: 450;
  font-variant-numeric: tabular-nums;
  line-height: 22px;
}

.wheel__row--1 { font-size: 16px; }

.wheel__row--0 {
  color: var(--dg-text-primary);
  font-size: 20px;
  font-weight: 600;
}

/* The unit sits on the centre row, to the right of the number. */
.wheel__label {
  position: absolute;
  top: 50%;
  right: 8px;
  color: var(--dg-text-secondary);
  font-size: 11px;
  font-weight: 550;
  transform: translateY(-40%);
  pointer-events: none;
}

@media (prefers-reduced-motion: reduce) {
  .wheel__stack.is-settling { transition: none; }
}
</style>
