<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import DgIcon from '@/components/DgIcon.vue'
import { useProvidersStore } from '@/stores/providers'
import ProviderSetup from '@/views/Settings/ProviderSetup.vue'

import OnboardingStep from './OnboardingStep.vue'

/*
 * AI service step (skippable). It reuses the settings add form and the
 * providers store as they are — the first service added becomes the primary
 * route there — so onboarding opens no second configuration path.
 */
const emit = defineEmits<{ back: []; next: [] }>()
const { t } = useI18n()
const store = useProvidersStore()

const adding = ref(false)

onMounted(async () => {
  await store.hydrate()
  adding.value = store.isEmpty && !store.unavailable
})
</script>

<template>
  <OnboardingStep :title="t('onboarding.provider.title')" :body="t('onboarding.provider.body')" @back="emit('back')">
    <ul v-if="!store.isEmpty" class="ob-services">
      <li v-for="provider in store.providers" :key="provider.id" class="ob-service">
        <DgIcon name="check" :size="14" />
        {{ t('onboarding.provider.configured', { name: provider.displayName }) }}
      </li>
    </ul>

    <div v-if="adding" class="ob-form">
      <ProviderSetup :provider="null" @done="adding = false" />
    </div>
    <button v-else-if="!store.unavailable" type="button" class="dg-button ob-add" @click="adding = true">
      + {{ store.isEmpty ? t('onboarding.provider.add') : t('onboarding.provider.addAnother') }}
    </button>

    <p v-if="store.isEmpty" class="ob-note">{{ t('onboarding.provider.skipNote') }}</p>

    <template #actions>
      <button v-if="store.isEmpty" type="button" class="dg-button dg-button--lg" @click="emit('next')">{{ t('onboarding.provider.later') }}</button>
      <button type="button" class="dg-button dg-button--lg dg-button--primary ob-primary" :disabled="store.isEmpty" @click="emit('next')">{{ t('onboarding.next') }}</button>
    </template>
  </OnboardingStep>
</template>

<style scoped>
.ob-services {
  display: grid;
  gap: 6px;
  max-width: 560px;
  margin: 0 auto 12px;
  padding: 0;
  list-style: none;
}

.ob-service {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  border: 1px solid var(--dg-timeline-grid);
  border-radius: 10px;
  background: var(--dg-timeline-card-fill);
  color: var(--dg-text-primary);
  font-size: 13px;
  font-weight: 600;
}

.ob-service svg { color: var(--dg-accent-text); }

.ob-form {
  max-width: 600px;
  margin: 0 auto;
  padding: 4px 18px 14px;
  border: 1px solid var(--dg-timeline-grid);
  border-radius: 14px;
  background: var(--dg-timeline-card-fill);
}

.ob-add {
  display: flex;
  margin: 0 auto;
}

.ob-note {
  max-width: 520px;
  margin: 14px auto 0;
  color: var(--dg-text-muted);
  font-size: 12px;
  line-height: 1.55;
  text-align: center;
}
</style>
