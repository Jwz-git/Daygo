<script setup lang="ts">
import { computed, nextTick, ref, useId, watch } from 'vue'
import { useI18n } from 'vue-i18n'

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

const { t } = useI18n()

const open = ref(false)
const query = ref('')
const filtering = ref(false)
const activeIndex = ref(-1)
const input = ref<HTMLInputElement | null>(null)
const list = ref<HTMLUListElement | null>(null)
const listId = `combo-${useId()}`

/** The text shown in the closed state: the matched option's label, or the
 * raw value (a hand-typed model name, say) when no option matches. */
const display = computed(() => {
  if (open.value && filtering.value) return query.value
  const match = props.options.find((option) => option.value === props.modelValue)
  if (match !== undefined) return match.label
  if (props.modelValue !== '' && props.fallbackLabel !== '') return props.fallbackLabel
  return props.modelValue
})

const filtered = computed(() => {
  const needle = filtering.value ? query.value.trim().toLowerCase() : ''
  if (needle === '') return props.options
  return props.options.filter((option) => {
    const haystack = `${option.value} ${option.label} ${option.detail ?? ''}`.toLowerCase()
    return haystack.includes(needle)
  })
})

watch(filtered, () => { activeIndex.value = -1 }, { flush: 'sync' })
watch(() => props.disabled, (disabled) => { if (disabled) open.value = false })

function openDropdown(): void {
  if (props.disabled || open.value) return
  filtering.value = false
  query.value = ''
  activeIndex.value = -1
  open.value = true
}

function select(value: string): void {
  if (props.disabled) return
  input.value?.focus()
  open.value = false
  filtering.value = false
  if (value !== props.modelValue) emit('update:modelValue', value)
}

function onInput(event: Event): void {
  openDropdown()
  query.value = (event.target as HTMLInputElement).value
  filtering.value = true
  // Update immediately so clicking Save cannot race a delayed blur commit.
  emit('update:modelValue', query.value)
}

function onKeydown(event: KeyboardEvent): void {
  if (props.disabled || event.isComposing || event.keyCode === 229) return
  if (event.key === 'Escape') {
    if (open.value) event.preventDefault()
    open.value = false
    return
  }
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault()
    openDropdown()
    const count = filtered.value.length
    if (count === 0) return
    activeIndex.value = activeIndex.value < 0
      ? event.key === 'ArrowDown' ? 0 : count - 1
      : (activeIndex.value + (event.key === 'ArrowDown' ? 1 : -1) + count) % count
    void nextTick(() => list.value?.children[activeIndex.value]?.scrollIntoView({ block: 'nearest' }))
  } else if (event.key === 'Enter' && open.value) {
    event.preventDefault()
    const option = filtered.value[activeIndex.value]
    if (option !== undefined) select(option.value)
    else onInputBlur()
  }
}

function onToggle(): void {
  if (props.disabled) return
  if (open.value) open.value = false
  else {
    input.value?.focus()
    openDropdown()
  }
}

function onInputBlur(): void {
  open.value = false
  if (!filtering.value) return
  const typed = query.value.trim()
  const exact = props.options.find((option) => option.value === typed || option.label === typed)
  const picked = exact?.value ?? typed
  if (picked !== props.modelValue) emit('update:modelValue', picked)
  filtering.value = false
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
      :aria-controls="listId"
      :aria-activedescendant="open && activeIndex >= 0 ? `${listId}-${activeIndex}` : undefined"
      aria-autocomplete="list"
      @input="onInput"
      @focus="openDropdown"
      @blur="onInputBlur"
      @keydown="onKeydown"
    />
    <button
      type="button"
      class="combo__toggle"
      :disabled="disabled"
      :tabindex="disabled ? -1 : 0"
      :aria-label="open ? t('common.combo.collapse') : t('common.combo.expand')"
      @mousedown.prevent
      @click="onToggle"
    >
      <span class="combo__chevron" aria-hidden="true">{{ open ? '▴' : '▾' }}</span>
    </button>
    <ul v-if="open && !disabled" :id="listId" ref="list" class="combo__list dg-popover" role="listbox">
      <li v-if="filtered.length === 0" class="combo__empty">
        <slot name="empty">{{ t('common.combo.noMatches') }}</slot>
      </li>
      <li v-for="(option, index) in filtered" :id="`${listId}-${index}`" :key="option.value" role="option" :aria-selected="option.value === modelValue">
        <button
          type="button"
          tabindex="-1"
          class="combo__option"
          :class="{ 'combo__option--active': option.value === modelValue || index === activeIndex }"
          @mousedown.prevent
          @click="select(option.value)"
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

/* Glass surface, entrance and fallbacks come from .dg-popover; only the
   anchoring geometry lives here. It drops out of the field (the default). */
.combo__list {
  position: absolute;
  top: calc(100% + 4px);
  left: 0;
  z-index: 30;
  width: 100%;
  max-height: 240px;
  margin: 0;
  padding: 4px;
  border-radius: 8px;
  list-style: none;
  overflow-y: auto;
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
