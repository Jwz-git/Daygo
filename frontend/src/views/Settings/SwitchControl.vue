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
  background: var(--dg-input-fill);
  cursor: pointer;
  transition:
    background var(--dg-motion-fast) ease,
    border-color var(--dg-motion-fast) ease;
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
  background: var(--dg-text-secondary);
  transition:
    transform var(--dg-motion-fast) var(--dg-ease-out),
    background var(--dg-motion-fast) ease;
}

.switch__input:checked + .switch__track {
  border-color: var(--dg-accent);
  background: var(--dg-control-fill);
}

.switch__input:checked + .switch__track::after {
  transform: translateX(18px);
  background: var(--dg-accent-strong);
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
</style>
