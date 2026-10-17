<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import DgIcon from '@/components/DgIcon.vue'
import {
  getPermissionState,
  openSystemSettings,
  relaunchForPermission,
  requestScreenRecordingPermission,
  setPermissionRestartArmed,
} from '@/api/system'
import { useOnboardingStore } from '@/stores/onboarding'

import OnboardingStep from './OnboardingStep.vue'

/*
 * macOS screen-recording step (the flow leaves it out elsewhere). The grant
 * only applies after a relaunch, so leaving through any path that can restart
 * the app — the system prompt, System Settings' "Quit & Reopen", our own
 * restart button — first records the guide as completed; the restarted app
 * then opens straight into Daygo instead of the guide's first page. While the
 * grant is missing the permission restart is armed, as the recording control
 * does, so macOS's reopen fully relaunches the resident agent.
 */
const emit = defineEmits<{ back: []; next: [] }>()
const { t } = useI18n()
const onboarding = useOnboardingStore()

type PermissionView = 'checking' | 'granted' | 'requestable' | 'denied'
const view = ref<PermissionView>('checking')
const requested = ref(false)

async function refresh(): Promise<void> {
  try {
    const state = await getPermissionState()
    view.value = state.screenRecording === 'granted' ? 'granted' : state.canRequest ? 'requestable' : 'denied'
  } catch {
    // No query on this host: nothing to grant here.
    view.value = 'granted'
  }
  setPermissionRestartArmed(view.value !== 'granted').catch(() => undefined)
}

async function request(): Promise<void> {
  await onboarding.markCompleted()
  requested.value = true
  try {
    await requestScreenRecordingPermission()
  } catch {
    // The prompt could not be shown; System Settings remains the way in.
  }
  await refresh()
}

async function openSettings(): Promise<void> {
  await onboarding.markCompleted()
  requested.value = true
  await openSystemSettings('screen_recording').catch(() => undefined)
}

async function relaunch(): Promise<void> {
  await onboarding.markCompleted()
  await relaunchForPermission().catch(() => undefined)
}

function onFocus(): void {
  if (view.value !== 'granted') void refresh()
}

onMounted(() => {
  void refresh()
  window.addEventListener('focus', onFocus)
})

onBeforeUnmount(() => {
  window.removeEventListener('focus', onFocus)
})
</script>

<template>
  <OnboardingStep :title="t('onboarding.permission.title')" :body="t('onboarding.permission.body')" @back="emit('back')">
    <div class="ob-perm" :class="`is-${view}`" role="status">
      <span class="ob-perm__badge" aria-hidden="true">
        <DgIcon :name="view === 'granted' ? 'check' : 'camera'" :size="18" />
      </span>
      <strong v-if="view === 'checking'">{{ t('onboarding.permission.checking') }}</strong>
      <strong v-else-if="view === 'granted'">{{ t('onboarding.permission.granted') }}</strong>
      <strong v-else>{{ t('onboarding.permission.missing') }}</strong>
    </div>

    <div v-if="view === 'requestable' || view === 'denied'" class="ob-perm__actions">
      <button v-if="view === 'requestable'" type="button" class="dg-button dg-button--primary" @click="request">
        {{ t('onboarding.permission.request') }}
      </button>
      <button type="button" class="dg-button" @click="openSettings">{{ t('onboarding.permission.openSettings') }}</button>
      <template v-if="view === 'denied' || requested">
        <p class="ob-perm__hint">{{ t('onboarding.permission.relaunchHint') }}</p>
        <button type="button" class="dg-button" @click="relaunch">{{ t('onboarding.permission.relaunch') }}</button>
      </template>
    </div>

    <template #actions>
      <button type="button" class="dg-button dg-button--lg dg-button--primary ob-primary" :disabled="view === 'checking'" @click="emit('next')">{{ t('onboarding.next') }}</button>
    </template>
  </OnboardingStep>
</template>

<style scoped>
.ob-perm {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  max-width: 420px;
  margin: 0 auto;
  padding: 14px 18px;
  border: 1px solid var(--dg-timeline-grid);
  border-radius: 14px;
  background: var(--dg-timeline-card-fill);
  color: var(--dg-text-primary);
  font-size: 14px;
}

.ob-perm__badge {
  display: grid;
  width: 30px;
  height: 30px;
  border-radius: 50%;
  background: var(--dg-control-fill);
  color: var(--dg-text-muted);
  place-items: center;
}

.ob-perm.is-granted .ob-perm__badge {
  background: color-mix(in srgb, var(--dg-success) 16%, transparent);
  color: var(--dg-success);
}

.ob-perm.is-requestable .ob-perm__badge,
.ob-perm.is-denied .ob-perm__badge {
  background: color-mix(in srgb, var(--dg-danger) 12%, transparent);
  color: var(--dg-danger);
}

.ob-perm__actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: center;
  gap: 8px;
  max-width: 480px;
  margin: 18px auto 0;
}

.ob-perm__hint {
  flex-basis: 100%;
  margin: 6px 0 0;
  color: var(--dg-text-muted);
  font-size: 12px;
  line-height: 1.55;
  text-align: center;
}
</style>
