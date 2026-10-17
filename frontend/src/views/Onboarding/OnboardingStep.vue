<script setup lang="ts">
import { useI18n } from 'vue-i18n'

/*
 * Shared frame of one onboarding step: serif title, lead paragraph, the
 * step's own content, and an action row whose primary buttons the step
 * supplies (it alone knows when continuing is allowed or busy).
 */
withDefaults(defineProps<{ title: string; body?: string; showBack?: boolean }>(), { body: '', showBack: true })
const emit = defineEmits<{ back: [] }>()
const { t } = useI18n()
</script>

<template>
  <section class="ob-step">
    <h1 class="ob-step__title">{{ title }}</h1>
    <p v-if="body !== ''" class="ob-step__body">{{ body }}</p>
    <div class="ob-step__content"><slot /></div>
    <footer class="ob-step__actions">
      <button v-if="showBack" type="button" class="dg-button dg-button--lg" @click="emit('back')">{{ t('onboarding.back') }}</button>
      <span class="ob-step__spacer"></span>
      <slot name="actions" />
    </footer>
  </section>
</template>

<style scoped>
.ob-step {
  display: grid;
  grid-template-rows: auto auto minmax(0, 1fr) auto;
  width: min(680px, 100%);
  height: 100%;
  min-height: 0;
}

.ob-step__title {
  margin: 0;
  color: var(--dg-text-primary);
  font-family: var(--dg-font-page-title);
  font-size: 36px;
  font-weight: var(--dg-page-title-weight);
  letter-spacing: -0.5px;
  line-height: 1.15;
  text-align: center;
}

.ob-step__body {
  max-width: 540px;
  margin: 12px auto 0;
  color: var(--dg-text-secondary);
  font-size: 14px;
  line-height: 1.6;
  text-align: center;
}

.ob-step__content {
  min-height: 0;
  margin-top: 28px;
  overflow-y: auto;
  padding: 4px;
}

.ob-step__actions {
  display: flex;
  align-items: center;
  gap: 10px;
  padding-top: 20px;
}

.ob-step__spacer { flex: 1; }

/* Steps slot in .dg-button--lg buttons; the primary one keeps a floor width. */
.ob-step__actions :slotted(.ob-primary) { min-width: 160px; }
</style>
