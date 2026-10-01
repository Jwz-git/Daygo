<script setup lang="ts">
/** One label + hint + control line. The shared shape of every settings field. */
defineProps<{ title: string; hint?: string }>()
</script>

<template>
  <section class="row dg-setting-row">
    <div class="row__text">
      <h2 class="row__title">{{ title }}</h2>
      <p v-if="hint" class="row__hint">{{ hint }}</p>
    </div>
    <div class="row__control">
      <slot />
    </div>
  </section>
</template>

<style scoped>
/*
 * A row is one line of a System Settings–style group. Consecutive rows join
 * into a single rounded container: the first of a run takes the top corners,
 * the last the bottom ones, and the rows between are split by an inset
 * hairline. Run detection lives in CSS so sections keep emitting plain rows.
 */
.row {
  position: relative;
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(120px, auto);
  align-items: center;
  gap: 16px;
  min-height: 44px;
  padding: 10px 14px;
  background: var(--dg-group-fill);
  box-shadow: inset 0.5px 0 0 var(--dg-group-border), inset -0.5px 0 0 var(--dg-group-border);
}

.row:not(.row + .row) {
  border-top-left-radius: var(--dg-group-radius);
  border-top-right-radius: var(--dg-group-radius);
  box-shadow:
    inset 0.5px 0 0 var(--dg-group-border),
    inset -0.5px 0 0 var(--dg-group-border),
    inset 0 0.5px 0 var(--dg-group-border);
}

.row:not(:has(+ .row)) {
  border-bottom-left-radius: var(--dg-group-radius);
  border-bottom-right-radius: var(--dg-group-radius);
  box-shadow:
    inset 0.5px 0 0 var(--dg-group-border),
    inset -0.5px 0 0 var(--dg-group-border),
    inset 0 -0.5px 0 var(--dg-group-border),
    0 1px 2px rgba(0, 0, 0, 0.03);
}

.row:not(.row + .row):not(:has(+ .row)) {
  box-shadow: var(--dg-group-shadow), inset 0 0 0 0.5px var(--dg-group-border);
}

/* Inset separator between joined rows, aligned with the text column. */
.row + .row::before {
  position: absolute;
  top: 0;
  right: 0;
  left: 14px;
  border-top: 0.5px solid var(--dg-separator);
  content: '';
}

.row__text {
  min-width: 0;
}

.row__title {
  color: var(--dg-text-primary);
  font-size: var(--dg-text-body);
  font-weight: 500;
}

.row__hint {
  margin-top: 2px;
  color: var(--dg-text-secondary);
  font-size: var(--dg-text-callout);
  line-height: 1.4;
  overflow-wrap: anywhere;
}

.row__control {
  min-width: 0;
  justify-self: end;
}

@media (max-width: 620px) {
  .row {
    grid-template-columns: minmax(0, 1fr);
    align-items: stretch;
  }

  .row__control {
    width: 100%;
    justify-self: stretch;
  }
}

@media (forced-colors: active) {
  .row {
    border: 1px solid CanvasText;
    background: Canvas;
  }
}
</style>
