<script setup lang="ts">
import { computed, ref, watch } from 'vue'

/**
 * A text input with a filtered dropdown: the user may pick one of the options
 * or keep typing freely. Options carry a value (what the model receives) and
 * a label (what the user reads); filtering matches value, label, and detail.
 */
interface ComboBoxOption {
  value: string
  label: string
  detail?: string
}

const props = withDefaults(
  defineProps<{
    modelValue: string
    options: ComboBoxOption[]
    placeholder?: string
    /** Option shown for the current value when it is not among the options. */
    fallbackLabel?: string
    disabled?: boolean
    ariaLabel?: string
  }>(),
  {
    placeholder: '',
    fallbackLabel: '',
    disabled: false,
    ariaLabel: '',
  },
)

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const open = ref(false)
const query = ref('')
const input = ref<HTMLInputElement | null>(null)

/** The text shown in the closed state: the matched option's label, or the
 * raw value (a hand-typed model name, say) when no option matches. */
const display = computed(() => {
  if (open.value) return query.value
  const match = props.options.find((option) => option.value === props.modelValue)
  if (match !== undefined) return match.label
  if (props.modelValue !== '' && props.fallbackLabel !== '') return props.fallbackLabel
  return props.modelValue
})

const filtered = computed(() => {
  const needle = query.value.trim().toLowerCase()
  if (needle === '') return props.options
  return props.options.filter((option) => {
    const haystack = `${option.value} ${option.label} ${option.detail ?? ''}`.toLowerCase()
    return haystack.includes(needle)
  })
})

watch(open, (next) => {
  if (next) query.value = ''
})

function select(value: string): void {
  open.value = false
  input.value?.focus()
  if (value !== props.modelValue) emit('update:modelValue', value)
}

function onInput(event: Event): void {
  query.value = (event.target as HTMLInputElement).value
  if (!open.value) open.value = true
}

function onKeydown(event: KeyboardEvent): void {
  if (event.isComposing || event.keyCode === 229) return
  if (event.key === 'Escape') {
    open.value = false
    return
  }
  if (event.key === 'ArrowDown' && filtered.value.length > 0) {
    event.preventDefault()
    open.value = true
  }
}

function onToggle(): void {
  if (props.disabled) return
  open.value = !open.value
  if (open.value) input.value?.focus()
}

function onInputBlur(): void {
  // Commit the typed text when it stops matching a filtered option's value or
  // label exactly; clicking an option re-focuses first, so a pick never lands
  // here as a partial commit.
  window.setTimeout(() => {
    if (!open.value) return
    open.value = false
    const typed = query.value.trim()
    if (typed === '') return
    const exact = props.options.find(
      (option) => option.value === typed || option.label === typed,
    )
    const picked = exact !== undefined ? exact.value : typed
    if (picked !== props.modelValue) emit('update:modelValue', picked)
  }, 120)
}
</script>

<template>
  <div class="combo" :class="{ 'combo--open': open, 'combo--disabled': disabled }">
    <input
      ref="input"
      class="dg-input combo__input"
      type="text"
      spellcheck="false"
      autocomplete="off"
      :value="display"
      :placeholder="placeholder"
      :disabled="disabled"
      :aria-label="ariaLabel || placeholder"
      role="combobox"
      :aria-expanded="open"
      aria-autocomplete="list"
      @input="onInput"
      @focus="open = true"
      @blur="onInputBlur"
      @keydown="onKeydown"
    />
    <button
      type="button"
      class="combo__toggle"
      :tabindex="disabled ? -1 : 0"
      :aria-label="open ? '收起选项' : '展开选项'"
      @mousedown.prevent="onToggle"
    >
      <span class="combo__chevron" aria-hidden="true">{{ open ? '▴' : '▾' }}</span>
    </button>
    <ul v-if="open && !disabled" class="combo__list" role="listbox">
      <li v-if="filtered.length === 0" class="combo__empty">
        <slot name="empty">无匹配项</slot>
      </li>
      <li v-for="option in filtered" :key="option.value" role="option" :aria-selected="option.value === modelValue">
        <button
          type="button"
          class="combo__option"
          :class="{ 'combo__option--active': option.value === modelValue }"
          @mousedown.prevent="select(option.value)"
        >
          <span class="combo__option-label">{{ option.label }}</span>
          <span v-if="option.detail !== undefined && option.detail !== ''" class="combo__option-detail">
            {{ option.detail }}
          </span>
        </button>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.combo {
  position: relative;
  display: flex;
  min-width: 0;
}

.combo--disabled {
  opacity: 0.6;
}

.combo__input {
  flex: 1;
  min-width: 0;
  padding-right: 26px;
}

.combo__toggle {
  position: absolute;
  top: 0;
  right: 0;
  display: flex;
  align-items: center;
  height: 100%;
  padding: 0 8px;
  border: none;
  background: none;
  color: var(--dg-text-muted);
  cursor: pointer;
}

.combo--disabled .combo__toggle {
  cursor: default;
}

.combo__chevron {
  font-size: 10px;
}

.combo__list {
  position: absolute;
  top: calc(100% + 4px);
  left: 0;
  z-index: 30;
  width: 100%;
  max-height: 240px;
  margin: 0;
  padding: 4px;
  border: 1px solid var(--dg-chip-border);
  border-radius: 8px;
  background: var(--dg-card-fill);
  list-style: none;
  overflow-y: auto;
  box-shadow: 0 6px 20px rgb(0 0 0 / 25%);
}

.combo__empty {
  padding: 8px 10px;
  color: var(--dg-text-muted);
  font-size: 12px;
}

.combo__option {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 1px;
  width: 100%;
  padding: 6px 10px;
  border: none;
  border-radius: 6px;
  background: none;
  text-align: left;
  cursor: pointer;
}

.combo__option:hover,
.combo__option--active {
  background: var(--dg-control-fill);
}

.combo__option-label {
  color: var(--dg-text-primary);
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 100%;
}

.combo__option-detail {
  color: var(--dg-text-muted);
  font-size: 11px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 100%;
}
</style>
