<script setup lang="ts" generic="T extends string | number">
import { computed, nextTick, onBeforeUnmount, ref, useId, watch } from 'vue'

/**
 * The app's single-choice dropdown, replacing the native <select>: WebKit's
 * Aqua popup and WebView2's white option sheet both ignore the glass surface,
 * so the list is drawn here and teleported to <body> (an inspector or settings
 * scroll container would otherwise clip it). Focus never leaves the trigger;
 * the highlighted option is announced through aria-activedescendant.
 */
export interface DgSelectOption<V extends string | number = string> {
  value: V
  label: string
  /** Secondary line under the label (a model id, a hint). */
  detail?: string
  /** Swatch drawn before the label (a category colour). */
  color?: string
  disabled?: boolean
}

/** What a parent reaches through a template ref (a generic component has no
 * usable InstanceType). */
export interface DgSelectHandle {
  focus: () => void
  open: () => Promise<void>
}

const props = withDefaults(
  defineProps<{
    modelValue: T
    options: DgSelectOption<T>[]
    /** Trigger text when the value matches no option. */
    placeholder?: string
    disabled?: boolean
    ariaLabel?: string
    size?: 'md' | 'sm'
    /** The list may grow wider than the trigger up to this many pixels. */
    menuMaxWidth?: number
  }>(),
  { placeholder: '', disabled: false, ariaLabel: undefined, size: 'md', menuMaxWidth: 320 },
)

const emit = defineEmits<{
  'update:modelValue': [value: T]
  /** The list closed, picked or dismissed. */
  close: [picked: boolean]
}>()

const MENU_GAP = 4
const MENU_MAX_HEIGHT = 280
const VIEWPORT_MARGIN = 8

const listId = useId()
const trigger = ref<HTMLButtonElement | null>(null)
const menu = ref<HTMLUListElement | null>(null)
const open = ref(false)
const activeIndex = ref(-1)
const placement = ref<'below' | 'above'>('below')
const menuStyle = ref<Record<string, string>>({})

const selectedIndex = computed(() => props.options.findIndex((option) => option.value === props.modelValue))
const selected = computed(() => props.options[selectedIndex.value])

function optionId(index: number): string {
  return `${listId}-${index}`
}

function place(): void {
  const anchor = trigger.value
  if (anchor === null) return
  const rect = anchor.getBoundingClientRect()
  // A trigger scrolled out of its container takes its list with it.
  if (rect.bottom < 0 || rect.top > window.innerHeight) {
    close(false)
    return
  }
  const below = window.innerHeight - rect.bottom - VIEWPORT_MARGIN
  const above = rect.top - VIEWPORT_MARGIN
  const wanted = Math.min(MENU_MAX_HEIGHT, menu.value?.scrollHeight ?? MENU_MAX_HEIGHT)
  placement.value = below >= wanted || below >= above ? 'below' : 'above'
  const room = placement.value === 'below' ? below : above
  const width = Math.max(rect.width, 140)
  const left = Math.min(rect.left, window.innerWidth - VIEWPORT_MARGIN - width)
  menuStyle.value = {
    left: `${Math.max(VIEWPORT_MARGIN, left)}px`,
    minWidth: `${width}px`,
    maxWidth: `${Math.max(width, props.menuMaxWidth)}px`,
    maxHeight: `${Math.max(120, Math.min(MENU_MAX_HEIGHT, room - MENU_GAP))}px`,
    ...(placement.value === 'below'
      ? { top: `${rect.bottom + MENU_GAP}px` }
      : { bottom: `${window.innerHeight - rect.top + MENU_GAP}px` }),
  }
}

function firstEnabled(from: number, step: 1 | -1): number {
  const count = props.options.length
  for (let i = 0, index = from; i < count; i += 1, index += step) {
    const wrapped = ((index % count) + count) % count
    if (props.options[wrapped]?.disabled !== true) return wrapped
  }
  return -1
}

async function openMenu(): Promise<void> {
  if (open.value || props.disabled || props.options.length === 0) return
  activeIndex.value = selectedIndex.value >= 0 ? selectedIndex.value : firstEnabled(0, 1)
  open.value = true
  place()
  await nextTick()
  place()
  revealActive()
}

function close(picked: boolean): void {
  if (!open.value) return
  open.value = false
  emit('close', picked)
}

function pick(index: number): void {
  const option = props.options[index]
  if (option === undefined || option.disabled === true) return
  // Update before close, so a parent committing on close sees the new value.
  if (option.value !== props.modelValue) emit('update:modelValue', option.value)
  close(true)
  trigger.value?.focus()
}

function revealActive(): void {
  if (activeIndex.value < 0) return
  void nextTick(() => {
    document.getElementById(optionId(activeIndex.value))?.scrollIntoView({ block: 'nearest' })
  })
}

function move(step: 1 | -1): void {
  const next = firstEnabled(activeIndex.value + step, step)
  if (next >= 0) {
    activeIndex.value = next
    revealActive()
  }
}

// Typeahead: a printable key jumps to the next option starting with it.
function jumpTo(char: string): void {
  const needle = char.toLowerCase()
  const count = props.options.length
  for (let i = 1; i <= count; i += 1) {
    const index = (Math.max(activeIndex.value, 0) + i) % count
    const option = props.options[index]
    if (option !== undefined && option.disabled !== true && option.label.toLowerCase().startsWith(needle)) {
      if (open.value) {
        activeIndex.value = index
        revealActive()
      } else {
        pick(index)
      }
      return
    }
  }
}

function onKeydown(event: KeyboardEvent): void {
  if (event.isComposing || props.disabled) return
  switch (event.key) {
    case 'ArrowDown':
    case 'ArrowUp':
      event.preventDefault()
      if (!open.value) void openMenu()
      else move(event.key === 'ArrowDown' ? 1 : -1)
      return
    case 'Home':
    case 'End':
      if (!open.value) return
      event.preventDefault()
      activeIndex.value = event.key === 'Home' ? firstEnabled(0, 1) : firstEnabled(props.options.length - 1, -1)
      revealActive()
      return
    case 'Enter':
    case ' ':
      event.preventDefault()
      if (open.value) pick(activeIndex.value)
      else void openMenu()
      return
    case 'Escape':
      if (!open.value) return
      event.preventDefault()
      event.stopPropagation()
      close(false)
      return
    case 'Tab':
      close(false)
      return
    default:
      if (event.key.length === 1 && !event.metaKey && !event.ctrlKey && !event.altKey) jumpTo(event.key)
  }
}

function onTriggerClick(): void {
  if (open.value) close(false)
  else void openMenu()
}

function onPointerDownOutside(event: PointerEvent): void {
  const target = event.target as Node | null
  if (target === null) return
  if (trigger.value?.contains(target) || menu.value?.contains(target)) return
  close(false)
}

function onScroll(event: Event): void {
  if (menu.value !== null && event.target instanceof Node && menu.value.contains(event.target)) return
  place()
}

function listen(on: boolean): void {
  if (on) {
    document.addEventListener('pointerdown', onPointerDownOutside, true)
    window.addEventListener('scroll', onScroll, true)
    window.addEventListener('resize', place)
  } else {
    document.removeEventListener('pointerdown', onPointerDownOutside, true)
    window.removeEventListener('scroll', onScroll, true)
    window.removeEventListener('resize', place)
  }
}

watch(open, (next) => listen(next))
watch(() => props.disabled, (next) => { if (next) close(false) })
onBeforeUnmount(() => listen(false))

defineExpose<DgSelectHandle>({
  focus: () => trigger.value?.focus(),
  open: openMenu,
})
</script>

<template>
  <!-- One root, so a parent's class and scoped styles land on the field. -->
  <div class="dg-select-field">
  <button
    ref="trigger"
    type="button"
    class="dg-select"
    :class="[`dg-select--${size}`, { 'dg-select--open': open, 'dg-select--placeholder': selected === undefined }]"
    role="combobox"
    aria-haspopup="listbox"
    :aria-expanded="open"
    :aria-controls="open ? listId : undefined"
    :aria-activedescendant="open && activeIndex >= 0 ? optionId(activeIndex) : undefined"
    :aria-label="ariaLabel"
    :disabled="disabled"
    @click="onTriggerClick"
    @keydown="onKeydown"
  >
    <i v-if="selected?.color" class="dg-select__swatch" :style="{ background: selected.color }"></i>
    <span class="dg-select__value">{{ selected?.label ?? placeholder }}</span>
    <span class="dg-select__chevron" aria-hidden="true"></span>
  </button>
  <Teleport to="body">
    <ul
      v-if="open"
      :id="listId"
      ref="menu"
      class="dg-select-menu dg-popover dg-scroll"
      :class="`dg-select-menu--${placement}`"
      role="listbox"
      :aria-label="ariaLabel"
      :style="menuStyle"
    >
      <li
        v-for="(option, index) in options"
        :id="optionId(index)"
        :key="String(option.value)"
        class="dg-select-menu__option"
        :class="{
          'is-active': index === activeIndex,
          'is-selected': index === selectedIndex,
          'is-disabled': option.disabled === true,
        }"
        role="option"
        :aria-selected="index === selectedIndex"
        :aria-disabled="option.disabled === true || undefined"
        @pointerenter="option.disabled !== true && (activeIndex = index)"
        @pointerdown.prevent
        @click="pick(index)"
      >
        <i v-if="option.color" class="dg-select__swatch" :style="{ background: option.color }"></i>
        <span class="dg-select-menu__text">
          <span class="dg-select-menu__label">{{ option.label }}</span>
          <span v-if="option.detail" class="dg-select-menu__detail">{{ option.detail }}</span>
        </span>
        <svg v-if="index === selectedIndex" class="dg-select-menu__check" viewBox="0 0 12 12" aria-hidden="true">
          <path d="M2.5 6.2 5 8.6l4.5-5.2" />
        </svg>
      </li>
    </ul>
  </Teleport>
  </div>
</template>

<style scoped>
.dg-select-field {
  display: flex;
  min-width: 0;
}

.dg-select {
  display: flex;
  align-items: center;
  gap: 7px;
  width: 100%;
  min-width: 0;
  min-height: 34px;
  padding: 7px 10px;
  border: none;
  border-radius: 7px;
  background: var(--dg-input-fill);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
  color: var(--dg-text-primary);
  font-size: 13px;
  text-align: left;
  cursor: pointer;
  transition:
    background-color var(--dg-motion-fast) ease,
    box-shadow var(--dg-motion-base) var(--dg-ease-out);
}

.dg-select--sm {
  min-height: 28px;
  padding: 4px 8px;
  border-radius: 6px;
  font-size: 12px;
}

.dg-select:hover:not(:disabled),
.dg-select--open {
  background: var(--dg-input-fill-hover);
}

.dg-select:focus-visible,
.dg-select--open {
  outline: none;
  box-shadow: 0 0 0 3px var(--dg-focus-ring);
}

.dg-select:disabled {
  opacity: 0.55;
  cursor: default;
}

.dg-select__value {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dg-select--placeholder .dg-select__value {
  color: var(--dg-text-muted);
}

.dg-select__chevron {
  flex: none;
  width: 12px;
  height: 12px;
  background: var(--dg-select-chevron) center / 12px 12px no-repeat;
  transition: transform var(--dg-motion-base) var(--dg-ease-glide);
}

.dg-select--open .dg-select__chevron {
  transform: rotate(180deg);
}

.dg-select__swatch {
  flex: none;
  width: 9px;
  height: 9px;
  border-radius: 3px;
  box-shadow: inset 0 0 0 0.5px rgba(0, 0, 0, 0.12);
}

/* Teleported: positioned against the viewport by place(). Surface, blur
   and entrance come from .dg-popover. */
.dg-select-menu {
  position: fixed;
  z-index: 1000;
  margin: 0;
  padding: 4px;
  border-radius: 10px;
  list-style: none;
  overflow-y: auto;
}

.dg-select-menu--above {
  --dg-popover-origin: 50% 100%;
  --dg-popover-from: translateY(4px);
}

.dg-select-menu__option {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 30px;
  padding: 5px 8px 5px 10px;
  border-radius: 6px;
  color: var(--dg-text-primary);
  font-size: 12.5px;
  cursor: pointer;
  user-select: none;
}

.dg-select-menu__option.is-active {
  background: var(--dg-hover-fill);
}

.dg-select-menu__option.is-selected {
  font-weight: 600;
}

.dg-select-menu__option.is-disabled {
  color: var(--dg-text-muted);
  cursor: default;
}

.dg-select-menu__text {
  display: grid;
  flex: 1;
  min-width: 0;
  gap: 1px;
}

.dg-select-menu__label,
.dg-select-menu__detail {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dg-select-menu__detail {
  color: var(--dg-text-muted);
  font-size: 11px;
  font-weight: 400;
}

.dg-select-menu__check {
  flex: none;
  width: 12px;
  height: 12px;
  fill: none;
  stroke: var(--dg-accent);
  stroke-width: 1.6;
  stroke-linecap: round;
  stroke-linejoin: round;
}
</style>
