<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import DgIcon from '@/components/DgIcon.vue'
import { useOnboardingStore } from '@/stores/onboarding'
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
const onboarding = useOnboardingStore()

/*
 * A preview is opened by a user who already has services. It starts as a first
 * run would — the services that exist when the step opens are left out of the
 * list and the empty state shows — and counts only services added during the
 * preview. Outside a preview nothing is left out.
 */
const existingIds = ref<ReadonlySet<string>>(new Set())
const services = computed(() => store.providers.filter((provider) => !existingIds.value.has(provider.id)))
const hasServices = computed(() => services.value.length > 0)

// With no service the two choices are listed straight away, also when the
// bindings are unavailable (plain-browser preview). Once a service exists the
// form only opens from the add button. `setupKey` remounts it after a cancel.
const adding = ref(false)
const setupKey = ref(0)
const showSetup = computed(() => adding.value || !hasServices.value)

function closeSetup(): void {
  adding.value = false
  setupKey.value += 1
}

onMounted(async () => {
  await store.hydrate()
  if (onboarding.preview) existingIds.value = new Set(store.providers.map((provider) => provider.id))
})
</script>

<template>
  <OnboardingStep :title="t('onboarding.provider.title')" :body="t('onboarding.provider.body')" @back="emit('back')">
    <ul v-if="hasServices" class="ob-services">
      <li v-for="provider in services" :key="provider.id" class="ob-service">
        <DgIcon name="check" :size="14" />
        {{ t('onboarding.provider.configured', { name: provider.displayName }) }}
      </li>
    </ul>

    <div v-if="showSetup" class="ob-form">
      <ProviderSetup :key="setupKey" :provider="null" @done="closeSetup" />
    </div>
    <button v-else type="button" class="dg-button ob-add" @click="adding = true">
      + {{ !hasServices ? t('onboarding.provider.add') : t('onboarding.provider.addAnother') }}
    </button>

    <p v-if="!hasServices" class="ob-note">{{ t('onboarding.provider.skipNote') }}</p>

    <template #actions>
      <button v-if="!hasServices" type="button" class="dg-button dg-button--lg" @click="emit('next')">{{ t('onboarding.provider.later') }}</button>
      <button type="button" class="dg-button dg-button--lg dg-button--primary ob-primary" :disabled="!hasServices" @click="emit('next')">{{ t('onboarding.next') }}</button>
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

/* ProviderSetup draws its own cards (choices, subscription and key forms), so
   this wrapper only sets the width instead of adding a second card around them. */
.ob-form {
  max-width: 600px;
  margin: 0 auto;
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
