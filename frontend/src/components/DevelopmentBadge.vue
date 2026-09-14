<template>
  <span class="development-badge"><slot /></span>
</template>

<style scoped>
/* Marks a page as rendering a development fixture rather than real data. */
.development-badge {
  padding: 3px 7px;
  border: 1px solid var(--dg-chip-border);
  border-radius: 5px;
  background: var(--dg-hover-fill);
  color: var(--dg-text-tertiary);
  font-size: 10px;
  font-weight: 600;
}
</style>
