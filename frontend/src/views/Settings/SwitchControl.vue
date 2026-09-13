<script setup lang="ts">
/** Accessible on/off switch; the single switch styling of the settings pages. */
defineProps<{
  checked: boolean
  disabled?: boolean
  label: string
}>()

const emit = defineEmits<{ toggle: [checked: boolean] }>()

function onChange(event: Event): void {
  emit('toggle', (event.target as HTMLInputElement).checked)
}
</script>

<template>
  <label class="switch">
    <input
      type="checkbox"
      role="switch"
      class="switch__input"
      :checked="checked"
      :disabled="disabled"
      :aria-label="label"
      @change="onChange"
    >
    <span class="switch__track" aria-hidden="true" />
  </label>
</template>

<style scoped>
.switch {
  display: inline-flex;
  align-items: center;
  padding: 9px 0;
}

.switch__input {
  position: absolute;
  width: 1px;
  height: 1px;
  margin: 0;
  opacity: 0;
}

.switch__track {
  position: relative;
  display: inline-block;
  box-sizing: border-box;
  width: 40px;
  height: 22px;
  border: 1px solid var(--dg-input-border);
  border-radius: 999px;
  background: var(--dg-switch-track);
  cursor: pointer;
  transition:
    background var(--dg-motion-base) ease,
    border-color var(--dg-motion-base) ease;
}

.switch__track::after {
  content: '';
  position: absolute;
  top: 2px;
  left: 2px;
  box-sizing: border-box;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: var(--dg-switch-knob);
  box-shadow: var(--dg-switch-knob-shadow);
  transition: transform var(--dg-motion-base) var(--dg-ease-out);
}

.switch__input:checked + .switch__track {
  border-color: transparent;
  background: var(--dg-accent);
}

.switch__input:checked + .switch__track::after {
  transform: translateX(18px);
}

.switch:hover .switch__input:not(:checked):not(:disabled) + .switch__track {
  background: var(--dg-switch-track-hover);
}

.switch:hover .switch__input:checked:not(:disabled) + .switch__track {
  background: var(--dg-accent-strong);
}

.switch:active .switch__input:not(:disabled) + .switch__track::after {
  transform: scale(0.9);
}

.switch:active .switch__input:not(:disabled):checked + .switch__track::after {
  transform: translateX(18px) scale(0.9);
}

.switch__input:focus-visible + .switch__track {
  box-shadow: 0 0 0 3px var(--dg-focus-ring);
}

.switch__input:disabled + .switch__track {
  opacity: 0.5;
  cursor: not-allowed;
}

@media (prefers-reduced-motion: reduce) {
  .switch__track,
  .switch__track::after {
    transition: none;
  }
}

/*
 * Forced colours flatten the white knob onto the forced Canvas track; fall
 * back to the system palette so on/off stays distinguishable.
 */
@media (forced-colors: active) {
  .switch__track {
    border: 1px solid ButtonText;
    background: Canvas;
  }

  .switch__track::after {
    border: 1px solid ButtonText;
    background: ButtonText;
  }

  .switch__input:checked + .switch__track {
    background: Highlight;
  }

  .switch__input:checked + .switch__track::after {
    background: HighlightText;
  }
}
</style>
