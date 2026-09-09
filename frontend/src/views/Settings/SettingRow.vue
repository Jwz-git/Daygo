<script setup lang="ts">
/** One label + hint + control line. The shared shape of every settings field. */
defineProps<{ title: string; hint?: string }>()
</script>

<template>
  <section class="row">
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
  gap: 16px;
  padding: 14px 18px;
  border: 1px solid var(--dg-card-border);
  border-radius: var(--dg-card-radius);
  background: var(--dg-card-fill);
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
