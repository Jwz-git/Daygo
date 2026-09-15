<script setup lang="ts">
/*
 * ConversationTitle — display a chat conversation title with an inline rename
 * affordance.
 *
 * The component owns only the *interaction* (display → edit → confirm/cancel);
 * persistence is the parent's job. The parent passes:
 *   - the current title (string) — drives both the read-only text and the
 *     initial value of the editor;
 *   - a `disabled` flag — the panel is locked while messages are streaming
 *     or the backend is otherwise unreachable;
 *   - a `busy` flag — disables the editor while the binding call is in flight
 *     (a rename during a turn would race the read → generate → replace
 *     sequence in the chat service);
 *   - an `errorText` prop — translated error text from the binding, if any.
 *     The input stays open and shows the message until the user retries or
 *     cancels.
 *
 * On confirm, the trimmed value is emitted via `commit`; the parent invokes
 * the binding and decides when to close the editor (by passing an updated
 * `title` that matches the draft).
 */
import { nextTick, ref, watch } from 'vue'

const props = defineProps<{
  title: string
  /** Lock the editor (loading / sending / unavailable). */
  disabled?: boolean
  /** Binding is in flight; the input shows a spinner and ignores Enter. */
  busy?: boolean
  /** Translated empty-state label shown when the conversation is untitled. */
  fallbackTitle: string
  /** Translated placeholder for the input. */
  placeholder: string
  /** Translated validation message for empty submissions. */
  emptyMessage: string
  /** Translated error from the binding; empty when none. */
  errorText?: string
}>()

const emit = defineEmits<{
  commit: [title: string]
}>()

const editing = ref(false)
const draft = ref('')
const localError = ref('')
const inputEl = ref<HTMLInputElement | null>(null)

/**
 * The store re-pulls conversations after the binding returns, which updates
 * `title`. While the editor is open we do not want the parent's update to
 * overwrite what the user typed; while it is closed we mirror the new value
 * so a successful external rename (e.g. via the drawer entry) shows up.
 */
watch(
  () => props.title,
  (next) => {
    if (!editing.value) draft.value = next
  },
)

async function beginEdit(): Promise<void> {
  if (props.disabled || props.busy || editing.value) return
  draft.value = props.title
  localError.value = ''
  editing.value = true
  await nextTick()
  const el = inputEl.value
  if (el !== null) {
    el.focus()
    el.select()
  }
}

function cancel(): void {
  editing.value = false
  draft.value = props.title
  localError.value = ''
}

function confirm(): void {
  const next = draft.value.trim()
  if (next === '') {
    localError.value = props.emptyMessage
    const el = inputEl.value
    if (el !== null) el.focus()
    return
  }
  if (next === props.title) {
    // No-op: do not round-trip the binding.
    editing.value = false
    localError.value = ''
    return
  }
  localError.value = ''
  emit('commit', next)
}

function onKeydown(e: KeyboardEvent): void {
  if (e.key === 'Enter' && !e.isComposing) {
    e.preventDefault()
    confirm()
  } else if (e.key === 'Escape') {
    e.preventDefault()
    cancel()
  }
}

function onBlur(): void {
  // Blur confirms; Escape already short-circuited to cancel(), so the draft
  // is back to the canonical title at this point and confirm() is a no-op.
  if (editing.value) confirm()
}

const editLabel = 'rename'
</script>

<template>
  <div class="ct" :class="{ 'ct--editing': editing, 'ct--busy': props.busy, 'ct--disabled': props.disabled }">
    <!-- Read-only view -->
    <div v-if="!editing" class="ct__read">
      <h2 class="ct__title">
        {{ props.title !== '' ? props.title : props.fallbackTitle }}
      </h2>
      <button
        v-if="!props.disabled"
        type="button"
        class="ct__edit"
        :aria-label="editLabel"
        :title="editLabel"
        @click="beginEdit"
      >
        <svg width="14" height="14" viewBox="0 0 16 16" fill="none" aria-hidden="true">
          <path
            d="M11.5 2.5l2 2-7.5 7.5H4v-2l7.5-7.5zM10.5 1.5l2.5 2.5 1-1a1 1 0 0 0-1.4-1.4l-1 1-.1-.1z"
            stroke="currentColor"
            stroke-width="1.2"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
        </svg>
      </button>
    </div>

    <!-- Editor -->
    <div v-else class="ct__edit-row">
      <input
        ref="inputEl"
        v-model="draft"
        type="text"
        class="ct__input dg-input"
        :placeholder="props.placeholder"
        :aria-invalid="(localError !== '') || (props.errorText ?? '') !== ''"
        :aria-busy="props.busy"
        :disabled="props.busy"
        @keydown="onKeydown"
        @blur="onBlur"
      />
      <span v-if="props.busy" class="ct__spinner" aria-hidden="true" />
    </div>
    <p
      v-if="localError !== '' || (props.errorText ?? '') !== ''"
      class="ct__error"
      role="alert"
    >
      {{ localError !== '' ? localError : props.errorText }}
    </p>
  </div>
</template>

<style scoped>
.ct {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.ct__read {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.ct__title {
  margin: 0;
  min-width: 0;
  color: var(--dg-text-primary);
  font-size: 14px;
  font-weight: 700;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ct__edit {
  flex: none;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  padding: 0;
  border: 1px solid transparent;
  border-radius: 6px;
  background: transparent;
  color: var(--dg-text-muted);
  cursor: pointer;
  opacity: 0;
  transition:
    opacity var(--dg-motion-base) ease,
    background var(--dg-motion-base) ease,
    color var(--dg-motion-base) ease,
    border-color var(--dg-motion-base) ease,
    transform var(--dg-motion-fast) ease;
}

/*
 * The pen appears on hover/focus of the read view rather than the whole
 * container — the panel head already exposes the toggle button on the right,
 * and showing it unconditionally makes the chrome too busy.
 */
.ct:hover .ct__edit,
.ct__edit:focus-visible {
  opacity: 1;
}

.ct__edit:hover {
  background: var(--dg-control-fill);
  color: var(--dg-text-primary);
  border-color: var(--dg-chip-border);
}

.ct__edit:active {
  transform: scale(0.94);
}

.ct__edit:focus-visible {
  outline: 2px solid var(--dg-accent-text);
  outline-offset: 2px;
}

.ct--disabled .ct__edit {
  display: none;
}

.ct__edit-row {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.ct__input {
  flex: 1;
  min-width: 0;
  min-height: 28px;
  padding: 4px 8px;
  font-size: 13px;
  font-weight: 600;
}

.ct__spinner {
  flex: none;
  width: 12px;
  height: 12px;
  border-radius: 50%;
  border: 1.5px solid var(--dg-chip-border);
  border-top-color: var(--dg-accent);
  animation: ct-spin 700ms linear infinite;
}

@keyframes ct-spin {
  to {
    transform: rotate(360deg);
  }
}

.ct__error {
  margin: 0;
  color: var(--dg-danger);
  font-size: 11px;
  line-height: 1.3;
}
</style>
