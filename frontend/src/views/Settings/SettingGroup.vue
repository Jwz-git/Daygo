<script setup lang="ts">
/** An open settings group: bold title + hint, then its rows. No card — the
 * rows carry their own hairline separators, matching the reference layout. */
defineProps<{ title: string; hint?: string }>()
</script>

<template>
  <section class="group">
    <div class="group__head">
      <h2 class="group__title">{{ title }}</h2>
      <p v-if="hint" class="group__hint">{{ hint }}</p>
    </div>
    <div class="group__rows">
      <slot />
    </div>
  </section>
</template>

<style scoped>
.group {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.group__head {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.group__title {
  color: var(--dg-text-primary);
  font-size: 15px;
  font-weight: 650;
}

.group__hint {
  color: var(--dg-text-secondary);
  font-size: 12px;
  max-width: 56ch;
}

.group__rows :deep(.row:last-child) {
  border-bottom: none;
}
</style>
