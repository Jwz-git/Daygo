<template>
  <span class="development-badge"><slot /></span>
</template>

<style scoped>
/* Marks a page as rendering a development fixture rather than real data.
   Opt out of the header's window-drag region so the badge (and any text
   inside it) stays selectable / interactable when the slot lives in a
   drag surface. */
.development-badge {
  padding: 3px 7px;
  border: 1px solid var(--dg-chip-border);
  border-radius: 5px;
  background: var(--dg-hover-fill);
  color: var(--dg-text-tertiary);
  font-size: 10px;
  font-weight: 600;
  -webkit-app-region: no-drag;
  --wails-draggable: no-drag;
}
</style>
