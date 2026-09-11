<script setup lang="ts">
/** One label + hint + control line. The shared shape of every settings field. */
defineProps<{ title: string; hint?: string }>()
</script>

<template>
  <section class="row dg-card">
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
.row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 18px;
  padding: 17px 18px;
  transition:
    border-color var(--dg-motion-base) ease;
}

.row:hover {
  border-color: var(--dg-chip-border);
}

.row__title {
  color: var(--dg-text-primary);
  font-size: 14px;
  font-weight: 600;
}

.row__hint {
  margin-top: 4px;
  color: var(--dg-text-secondary);
  font-size: 12px;
}

.row__control {
  flex: none;
}

@media (max-width: 620px) {
  .row {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>
