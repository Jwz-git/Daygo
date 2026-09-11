<script setup lang="ts">
/**
 * Placeholder for a feature that is on the roadmap but not built yet.
 *
 * It says plainly that the area is planned and stops there: docs/09-roadmap.md
 * fixes no build order and retired the M0–M5 numbers, so a placeholder must not
 * imply a ship date or name an internal module as if it were a release. Every
 * string it renders comes from i18n; callers pass message keys, not copy.
 */
const props = defineProps<{
  titleKey: string
  descriptionKey?: string
}>()
</script>

<template>
  <section class="planned-notice dg-card">
    <header class="planned-notice__head">
      <h2 class="planned-notice__title">{{ $t(props.titleKey) }}</h2>
      <span class="planned-notice__status">
        {{ $t('common.placeholder.planned') }}
      </span>
    </header>
    <p v-if="props.descriptionKey" class="planned-notice__description">
      {{ $t(props.descriptionKey) }}
    </p>
  </section>
</template>

<style scoped>
.planned-notice {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-height: 112px;
  padding: 18px 20px;
  overflow: hidden;
  /* Planned sections are quiet grouped rows, not promotional cards. */
  background: color-mix(in srgb, var(--dg-card-fill) 72%, transparent);
  box-shadow: none;
}

.planned-notice__head {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.planned-notice__title {
  color: var(--dg-text-primary);
  font-size: 14px;
  font-weight: 600;
}

/*
 * A quiet status, not a data tag: the app's UI face, sentence case, no tracking
 * — so a column of these stays calm and never reads as a code label.
 */
.planned-notice__status {
  flex: none;
  padding: 3px 9px;
  border-radius: 5px;
  background: var(--dg-hover-fill);
  color: var(--dg-text-tertiary);
  font-size: 11px;
  font-weight: 550;
  white-space: nowrap;
}

.planned-notice__description {
  position: relative;
  z-index: 1;
  max-width: 42rem;
  color: var(--dg-text-secondary);
  font-size: 13px;
}
</style>
