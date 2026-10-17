<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import daygoIcon from '@/assets/favicons/daygo.png'
import DgIcon from '@/components/DgIcon.vue'
import { resolveDesktopPlatform } from '@/lib/desktopPlatform'
import { useOnboardingStore } from '@/stores/onboarding'
import { useProvidersStore } from '@/stores/providers'
import { useRecordingStore } from '@/stores/recording'

import OnboardingPermission from './OnboardingPermission.vue'
import OnboardingPreferences from './OnboardingPreferences.vue'
import OnboardingProfession from './OnboardingProfession.vue'
import OnboardingProvider from './OnboardingProvider.vue'

/*
 * First-run guide, after Dayflow's onboarding: welcome → profession
 * (categories) → AI service (skippable) → preferences → screen recording
 * (macOS only) → done. It covers the shell rather than replacing it, so the
 * recording store the shell starts is already live for the last step. Every
 * step writes through the bindings the app already uses; reaching "done", or
 * skipping, records system.onboardingCompleted.
 */
type Step = 'welcome' | 'profession' | 'provider' | 'preferences' | 'permission' | 'done'

const { t } = useI18n()
const onboarding = useOnboardingStore()
const providers = useProvidersStore()
const recording = useRecordingStore()

const isMac = ref(false)
const isWindows = document.documentElement.dataset.dgPlatform === 'windows'
const step = ref<Step>('welcome')
const direction = ref<'forward' | 'back'>('forward')
const starting = ref(false)

onMounted(async () => {
  isMac.value = (await resolveDesktopPlatform()) === 'darwin'
})

const steps = computed<Step[]>(() => [
  'welcome', 'profession', 'provider', 'preferences', ...(isMac.value ? ['permission' as const] : []), 'done',
])

/** The numbered steps, between welcome and done. */
const numbered = computed(() => steps.value.slice(1, -1))
const current = computed(() => numbered.value.indexOf(step.value) + 1)

function go(delta: 1 | -1): void {
  const index = steps.value.indexOf(step.value)
  const next = steps.value[index + delta]
  if (next === undefined) return
  direction.value = delta > 0 ? 'forward' : 'back'
  step.value = next
  if (next === 'done') void onboarding.markCompleted()
}

async function startRecording(): Promise<void> {
  starting.value = true
  try {
    await recording.perform('start')
  } finally {
    starting.value = false
    onboarding.close()
  }
}
</script>

<template>
  <div class="ob" :class="{ 'is-windows': isWindows }" role="dialog" aria-modal="true" :aria-label="t('onboarding.welcome.eyebrow')">
    <div class="ob__glow ob__glow--a" aria-hidden="true"></div>
    <div class="ob__glow ob__glow--b" aria-hidden="true"></div>

    <header class="ob__bar">
      <div v-if="current > 0" class="ob__progress" :aria-label="t('onboarding.stepOf', { current, total: numbered.length })">
        <span
          v-for="(item, index) in numbered"
          :key="item"
          class="ob__segment"
          :class="{ 'is-done': index < current - 1, 'is-current': index === current - 1 }"
        ></span>
        <span class="ob__count">{{ t('onboarding.stepOf', { current, total: numbered.length }) }}</span>
      </div>
      <span v-if="onboarding.preview" class="ob__preview">{{ t('onboarding.previewNote') }}</span>
      <span class="ob__spacer"></span>
      <button v-if="step !== 'done'" type="button" class="ob__skip" @click="onboarding.complete()">{{ t('onboarding.skip') }}</button>
    </header>

    <main class="ob__stage">
      <Transition :name="`ob-${direction}`" mode="out-in">
        <section v-if="step === 'welcome'" key="welcome" class="ob-hero">
          <img class="ob-hero__icon" :src="daygoIcon" alt="" width="96" height="96" />
          <p class="ob-hero__eyebrow">{{ t('onboarding.welcome.eyebrow') }}</p>
          <h1 class="ob-hero__title">{{ t('onboarding.welcome.title') }}</h1>
          <p class="ob-hero__body">{{ t('onboarding.welcome.body') }}</p>
          <p class="ob-hero__privacy"><DgIcon name="lock" :size="13" />{{ t('onboarding.welcome.privacy') }}</p>
          <button type="button" class="dg-button dg-button--lg dg-button--primary ob-hero__cta" @click="go(1)">
            {{ t('onboarding.welcome.start') }}
            <DgIcon name="arrowRight" :size="14" />
          </button>
        </section>

        <OnboardingProfession v-else-if="step === 'profession'" key="profession" @back="go(-1)" @next="go(1)" />
        <OnboardingProvider v-else-if="step === 'provider'" key="provider" @back="go(-1)" @next="go(1)" />
        <OnboardingPreferences v-else-if="step === 'preferences'" key="preferences" @back="go(-1)" @next="go(1)" />
        <OnboardingPermission v-else-if="step === 'permission'" key="permission" @back="go(-1)" @next="go(1)" />

        <section v-else key="done" class="ob-hero">
          <span class="ob-hero__check" aria-hidden="true"><DgIcon name="check" :size="30" /></span>
          <h1 class="ob-hero__title">{{ t('onboarding.done.title') }}</h1>
          <p class="ob-hero__body">{{ t('onboarding.done.body') }}</p>
          <p v-if="providers.isEmpty" class="ob-hero__privacy">{{ t('onboarding.done.noProvider') }}</p>
          <div class="ob-hero__buttons">
            <button type="button" class="dg-button dg-button--lg" :disabled="starting" @click="onboarding.close()">{{ t('onboarding.done.enter') }}</button>
            <button
              v-if="recording.canControl && recording.lifecycle !== 'capturing'"
              type="button"
              class="dg-button dg-button--lg dg-button--primary ob-hero__cta"
              :disabled="starting"
              @click="startRecording"
            >{{ t('onboarding.done.start') }}</button>
          </div>
        </section>
      </Transition>
    </main>
  </div>
</template>

<style scoped>
.ob {
  position: fixed;
  inset: 0;
  z-index: 900;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  overflow: hidden;
  background-color: var(--dg-window-bg);
  background-image: var(--dg-window-gradient);
  animation: ob-in 420ms var(--dg-ease-out) both;
}

/* Keep the custom Windows title bar (window controls) uncovered. */
.ob.is-windows { top: var(--dg-windows-titlebar-height); }

/* Two slow warm blooms behind the content, in the spirit of Dayflow's
   onboarding backdrop. */
.ob__glow {
  position: absolute;
  border-radius: 50%;
  filter: blur(60px);
  opacity: 0.55;
  pointer-events: none;
}

.ob__glow--a {
  top: -160px;
  left: -120px;
  width: 520px;
  height: 420px;
  background: color-mix(in srgb, var(--dg-accent) 30%, transparent);
  animation: ob-drift 16s ease-in-out infinite alternate;
}

.ob__glow--b {
  right: -160px;
  bottom: -200px;
  width: 560px;
  height: 460px;
  background: color-mix(in srgb, #ffb98a 40%, transparent);
  animation: ob-drift 20s ease-in-out infinite alternate-reverse;
}

.ob__bar {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  gap: 12px;
  height: 52px;
  /* Room for the macOS traffic lights; the bar doubles as the drag handle. */
  padding: 0 20px 0 88px;
  --wails-draggable: drag;
}

.ob.is-windows .ob__bar { padding-left: 20px; }

.ob__progress {
  display: flex;
  align-items: center;
  gap: 5px;
}

.ob__segment {
  width: 28px;
  height: 4px;
  border-radius: 999px;
  background: var(--dg-hover-fill-strong);
  transition: background var(--dg-motion-base) ease, width var(--dg-motion-base) var(--dg-ease-out);
}

.ob__segment.is-done { background: color-mix(in srgb, var(--dg-accent) 55%, transparent); }
.ob__segment.is-current { width: 44px; background: var(--dg-accent); }

.ob__count {
  margin-left: 8px;
  color: var(--dg-text-muted);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}

.ob__spacer { flex: 1; }

.ob__preview {
  padding: 3px 10px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--dg-accent) 14%, transparent);
  color: var(--dg-accent-text);
  font-size: 11px;
  font-weight: 600;
}

.ob__skip {
  padding: 6px 12px;
  border: none;
  border-radius: 999px;
  background: transparent;
  color: var(--dg-text-muted);
  font-size: 12px;
  font-weight: 550;
  cursor: pointer;
  --wails-draggable: no-drag;
}

.ob__skip:hover { background: var(--dg-hover-fill); color: var(--dg-text-primary); }
.ob__skip:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }

.ob__stage {
  position: relative;
  z-index: 1;
  display: grid;
  justify-items: center;
  min-height: 0;
  padding: 12px 32px 32px;
}

/* ---- Welcome / done ---- */
.ob-hero {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  max-width: 560px;
  height: 100%;
  text-align: center;
}

.ob-hero__icon {
  width: 96px;
  height: 96px;
  margin-bottom: 18px;
  filter: drop-shadow(0 10px 24px rgba(99, 71, 56, 0.25));
}

.ob-hero__eyebrow {
  margin: 0 0 6px;
  color: var(--dg-accent-text);
  font-size: 13px;
  font-weight: 650;
}

.ob-hero__title {
  margin: 0;
  color: var(--dg-text-primary);
  font-family: var(--dg-font-page-title);
  font-size: 44px;
  font-weight: var(--dg-page-title-weight);
  letter-spacing: -1px;
  line-height: 1.12;
}

.ob-hero__body {
  margin: 16px 0 0;
  color: var(--dg-text-secondary);
  font-size: 15px;
  line-height: 1.65;
}

.ob-hero__privacy {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin: 14px 0 0;
  color: var(--dg-text-muted);
  font-size: 12px;
  line-height: 1.55;
}

.ob-hero__privacy svg { flex: none; }

.ob-hero__check {
  display: grid;
  width: 72px;
  height: 72px;
  margin-bottom: 20px;
  border-radius: 50%;
  background: var(--dg-accent);
  color: #ffffff;
  place-items: center;
  box-shadow: 0 10px 28px color-mix(in srgb, var(--dg-accent) 40%, transparent);
  animation: ob-pop 520ms cubic-bezier(0.2, 1.4, 0.4, 1) both;
}

.ob-hero__buttons {
  display: flex;
  gap: 10px;
  margin-top: 32px;
}

.ob-hero__cta {
  min-width: 200px;
  margin-top: 32px;
}

.ob-hero__buttons .ob-hero__cta { margin-top: 0; }

/* ---- Step transitions: slide with the direction of travel ---- */
.ob-forward-enter-active,
.ob-forward-leave-active,
.ob-back-enter-active,
.ob-back-leave-active { transition: opacity 240ms ease, transform 320ms var(--dg-ease-out); }

.ob-forward-enter-from { opacity: 0; transform: translateX(28px); }
.ob-forward-leave-to { opacity: 0; transform: translateX(-28px); }
.ob-back-enter-from { opacity: 0; transform: translateX(-28px); }
.ob-back-leave-to { opacity: 0; transform: translateX(28px); }

@keyframes ob-in {
  from { opacity: 0; }
  to { opacity: 1; }
}

@keyframes ob-drift {
  from { transform: translate(0, 0) scale(1); }
  to { transform: translate(40px, 30px) scale(1.08); }
}

@keyframes ob-pop {
  from { opacity: 0; transform: scale(0.6); }
  to { opacity: 1; transform: scale(1); }
}

@media (prefers-reduced-motion: reduce) {
  .ob,
  .ob__glow,
  .ob-hero__check { animation: none; }
  .ob-forward-enter-active,
  .ob-forward-leave-active,
  .ob-back-enter-active,
  .ob-back-leave-active { transition: opacity 120ms ease; }
  .ob-forward-enter-from,
  .ob-forward-leave-to,
  .ob-back-enter-from,
  .ob-back-leave-to { transform: none; }
}
</style>
