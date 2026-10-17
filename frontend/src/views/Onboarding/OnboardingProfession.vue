<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { applyCategoryPreset } from '@/api/timeline'
import { PROFESSION_PRESETS, presetRows, type ProfessionPresetId } from '@/lib/categoryPresets'
import { useOnboardingStore } from '@/stores/onboarding'

import OnboardingStep from './OnboardingStep.vue'

/*
 * Profession step (Dayflow's role selection): picking a role replaces the
 * starter categories with that role's preset — on a first run nothing
 * references the starter set yet, so the timeline starts with exactly the
 * preset; anything a card, plan or goal does reference is kept by the
 * backend. "Keep defaults" moves on without a write.
 */
const emit = defineEmits<{ back: []; next: [] }>()
const { t } = useI18n()
const onboarding = useOnboardingStore()

const selected = ref<ProfessionPresetId | null>(null)
const applying = ref(false)
const failed = ref(false)

const preview = computed(() => {
  const preset = PROFESSION_PRESETS.find((candidate) => candidate.id === selected.value)
  return preset === undefined ? [] : presetRows(preset, t)
})

async function apply(): Promise<void> {
  const preset = PROFESSION_PRESETS.find((candidate) => candidate.id === selected.value)
  if (preset === undefined || applying.value) return
  // A preview never rewrites the categories of a user with history.
  if (onboarding.preview) {
    emit('next')
    return
  }
  applying.value = true
  failed.value = false
  try {
    await applyCategoryPreset(presetRows(preset, t))
    emit('next')
  } catch {
    failed.value = true
  } finally {
    applying.value = false
  }
}
</script>

<template>
  <OnboardingStep :title="t('onboarding.profession.title')" :body="t('onboarding.profession.body')" @back="emit('back')">
    <div class="ob-roles" role="radiogroup" :aria-label="t('onboarding.profession.title')">
      <button
        v-for="preset in PROFESSION_PRESETS"
        :key="preset.id"
        type="button"
        role="radio"
        class="ob-role"
        :class="{ 'is-selected': selected === preset.id }"
        :aria-checked="selected === preset.id"
        @click="selected = preset.id"
      >{{ t(`presets.profession.${preset.nameKey}.name`) }}</button>
    </div>

    <Transition name="ob-fade" mode="out-in">
      <div v-if="preview.length > 0" :key="selected ?? ''" class="ob-preview">
        <p class="ob-preview__label">{{ t('onboarding.profession.preview') }}</p>
        <ul class="ob-preview__list">
          <li v-for="row in preview" :key="row.name" class="ob-preview__row">
            <i :style="{ background: row.colorHex }"></i>
            <span class="ob-preview__text">
              <strong>{{ row.name }}</strong>
              <span>{{ row.details }}</span>
            </span>
            <em>{{ t(`presets.archetype.${row.archetype}`) }}</em>
          </li>
        </ul>
      </div>
    </Transition>

    <p v-if="failed" class="ob-error" role="alert">{{ t('onboarding.profession.failed') }}</p>

    <template #actions>
      <button type="button" class="dg-button dg-button--lg" :disabled="applying" @click="emit('next')">{{ t('onboarding.profession.keep') }}</button>
      <button type="button" class="dg-button dg-button--lg dg-button--primary ob-primary" :disabled="selected === null || applying" @click="apply">
        {{ applying ? t('onboarding.profession.applying') : t('onboarding.next') }}
      </button>
    </template>
  </OnboardingStep>
</template>

<style scoped>
.ob-roles {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 8px;
}

/* Dayflow's role chips: translucent capsules that warm up and glow once picked. */
.ob-role {
  padding: 9px 20px;
  border: 1px solid var(--dg-timeline-grid);
  border-radius: 999px;
  background: color-mix(in srgb, var(--dg-timeline-card-fill) 70%, transparent);
  color: var(--dg-text-primary);
  font-size: 14px;
  font-weight: 550;
  cursor: pointer;
  box-shadow: 0 0 4px rgba(175, 114, 70, 0.12);
  transition: border-color var(--dg-motion-fast) ease, background var(--dg-motion-fast) ease, box-shadow var(--dg-motion-fast) ease;
}

.ob-role:hover { border-color: color-mix(in srgb, var(--dg-accent) 40%, transparent); }

.ob-role.is-selected {
  border-color: color-mix(in srgb, var(--dg-accent) 70%, transparent);
  background: color-mix(in srgb, var(--dg-accent) 12%, var(--dg-timeline-card-fill));
  box-shadow: 0 0 8px color-mix(in srgb, var(--dg-accent) 40%, transparent);
}

.ob-role:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }

.ob-preview {
  margin: 24px auto 0;
  max-width: 560px;
}

.ob-preview__label {
  margin: 0 0 8px 4px;
  color: var(--dg-text-muted);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.02em;
}

.ob-preview__list {
  display: grid;
  gap: 6px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.ob-preview__row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 14px;
  border: 1px solid var(--dg-timeline-grid);
  border-radius: 10px;
  background: var(--dg-timeline-card-fill);
}

.ob-preview__row i {
  flex: none;
  width: 12px;
  height: 12px;
  border-radius: 4px;
  box-shadow: 0 1px 3px rgba(35, 39, 60, 0.18);
}

.ob-preview__text {
  display: grid;
  flex: 1;
  gap: 2px;
  min-width: 0;
}

.ob-preview__text strong { color: var(--dg-text-primary); font-size: 13px; font-weight: 650; }

.ob-preview__text span {
  overflow: hidden;
  color: var(--dg-text-secondary);
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ob-preview__row em {
  flex: none;
  padding: 2px 8px;
  border-radius: 999px;
  background: var(--dg-control-fill);
  color: var(--dg-text-muted);
  font-size: 10px;
  font-style: normal;
  font-weight: 600;
}

.ob-error {
  margin: 14px 0 0;
  color: var(--dg-danger);
  font-size: 12px;
  text-align: center;
}

.ob-fade-enter-active,
.ob-fade-leave-active { transition: opacity 180ms ease, transform 180ms var(--dg-ease-out); }
.ob-fade-enter-from,
.ob-fade-leave-to { opacity: 0; transform: translateY(6px); }

@media (prefers-reduced-motion: reduce) {
  .ob-fade-enter-active,
  .ob-fade-leave-active { transition: none; }
}
</style>
