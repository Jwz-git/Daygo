<script setup lang="ts">
/*
 * Card shell shared by the weekly charts, after Dayflow's weekly sections:
 * 4px corners, a warm 1px border, the translucent weekly card fill, an
 * Instrument Serif title in the weekly accent, and an optional footer strip.
 */
defineProps<{ title: string }>()
</script>

<template>
  <section class="wk-card">
    <h3 class="wk-card__title">{{ title }}</h3>
    <div class="wk-card__body">
      <slot />
    </div>
    <footer v-if="$slots.footer" class="wk-card__footer">
      <slot name="footer" />
    </footer>
  </section>
</template>

<style scoped>
.wk-card {
  display: flex;
  flex-direction: column;
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--dg-wk-card-border);
  border-radius: 4px;
  background: var(--dg-wk-card-fill);
  color: var(--dg-wk-text);
}

.wk-card__title {
  margin: 0;
  padding: 16px 18px 0;
  color: var(--dg-wk-title);
  font-family: var(--dg-font-serif);
  font-size: 20px;
  font-weight: 400;
  line-height: 1.25;
}

.wk-card__body {
  flex: 1;
  min-width: 0;
  padding: 14px 18px 18px;
}

.wk-card__footer {
  padding: 12px 16px;
  border-top: 1px solid var(--dg-wk-divider);
  background: var(--dg-wk-footer);
  color: var(--dg-wk-text-secondary);
  font-size: 12px;
  line-height: 1.5;
}
</style>
