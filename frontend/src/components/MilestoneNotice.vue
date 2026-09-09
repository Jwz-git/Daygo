<script setup lang="ts">
/**
 * Placeholder for a section that a later milestone fills in. Every string it
 * renders comes from i18n; callers pass message keys, not copy.
 */
const props = defineProps<{
  titleKey: string
  descriptionKey?: string
  milestone: 1 | 2 | 3 | 4 | 5
}>()
</script>

<template>
  <section class="milestone-notice">
    <header class="milestone-notice__head">
      <h2 class="milestone-notice__title">{{ $t(props.titleKey) }}</h2>
      <span class="milestone-notice__tag">
        {{ $t('common.placeholder.milestoneLabel', { milestone: props.milestone }) }}
      </span>
    </header>
    <p v-if="props.descriptionKey" class="milestone-notice__description">
      {{ $t(props.descriptionKey) }}
    </p>
    <p class="milestone-notice__hint">
      {{ $t('common.placeholder.milestoneHint', { milestone: props.milestone }) }}
    </p>
  </section>
</template>

<style scoped>
.milestone-notice {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 18px;
  border: 1px dashed var(--dg-card-border);
  border-radius: var(--dg-card-radius);
  background: var(--dg-card-fill);
}

.milestone-notice__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.milestone-notice__title {
  color: var(--dg-text-primary);
  font-size: 14px;
  font-weight: 600;
}

.milestone-notice__tag {
  flex: none;
  padding: 3px 8px;
  border-radius: 999px;
  background: var(--dg-control-fill);
  color: var(--dg-text-secondary);
  font-family: var(--dg-font-mono);
  font-size: 10px;
  letter-spacing: 0.04em;
}

.milestone-notice__description {
  color: var(--dg-text-secondary);
  font-size: 13px;
}

.milestone-notice__hint {
  color: var(--dg-text-muted);
  font-size: 12px;
}
</style>
