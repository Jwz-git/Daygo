<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ProviderDTO } from '@/api/dto'
import DgIcon from '@/components/DgIcon.vue'
import { onSettingsChanged } from '@/api/settings'
import { createChatGPTSetup } from '@/stores/chatgpt'
import { useProvidersStore } from '@/stores/providers'

const props = defineProps<{ provider: ProviderDTO | null }>()
const emit = defineEmits<{ done: [] }>()
const { t } = useI18n()
const providers = useProvidersStore()
const setup = createChatGPTSetup(undefined, props.provider?.id)
const { connection, working, waiting, failed, remoteUnconfirmed } = setup
const name = ref(props.provider?.displayName ?? t('chatgpt.subscription'))
const selected = ref<string[]>([...(props.provider?.models ?? [])])
const canSave = computed(() => !!connection.value && ['connected', 'ready'].includes(connection.value.state) && !connection.value.needsConfirmation && name.value.trim() !== '' && selected.value.length > 0 && !working.value)
const errorText = computed(() => {
  switch (connection.value?.errorCode) {
    case 'access_denied': case 'canceled': return t('chatgpt.denied')
    case 'plan_permission_required': return t('chatgpt.permission')
    case 'identity_mismatch': case 'identity_rejected': return t('chatgpt.identity')
    case 'credentials_unavailable': return t('chatgpt.credentials')
    case 'timeout': return t('chatgpt.timeout')
    default: return t('chatgpt.error')
  }
})
watch(() => connection.value?.models, (models) => {
  if (models && models.length > 0) selected.value = selected.value.filter((model) => models.includes(model))
})
let stopEvents: (() => void) | undefined
onMounted(() => {
  // A new form mounts only after the user chooses the subscription option.
  // Existing subscriptions only load status; reauthorization stays explicit.
  void (props.provider ? setup.load() : setup.start())
  stopEvents = onSettingsChanged((keys) => { if (keys.includes('providers.routing')) void setup.load() })
})
onBeforeUnmount(() => { stopEvents?.(); setup.dispose() })
async function save(): Promise<void> {
  if (await setup.save(name.value, selected.value)) { await providers.refresh(); emit('done') }
}
async function disconnect(): Promise<void> { await setup.disconnect(); await providers.refresh() }
</script>

<template>
  <form class="form" @submit.prevent="save">
    <h3 class="form__title">
      {{ provider === null ? t('settings.providers.form.addTitle') : t('settings.providers.form.editTitle') }}
    </h3>
    <p class="form__hint">{{ t('chatgpt.description') }}</p>

    <p v-if="waiting" class="form__status" role="status">{{ t('chatgpt.waiting') }}</p>
    <p v-else-if="connection?.state === 'connected' || connection?.state === 'ready'" class="form__status form__status--ok" role="status">
      <DgIcon name="lock" :size="11" />
      {{ t('chatgpt.connected') }}<span v-if="connection.email"> · {{ connection.email }}</span>
    </p>
    <p v-else-if="provider && connection?.state === 'disconnected'" class="form__hint">{{ t('chatgpt.disconnected') }}</p>
    <p v-if="failed || connection?.state === 'error'" class="form__error" role="alert">{{ errorText }}</p>
    <p v-if="remoteUnconfirmed" class="form__error" role="alert">{{ t('chatgpt.remoteUnconfirmed') }}</p>

    <div v-if="provider || failed || connection?.state === 'error' || connection?.state === 'disconnected'" class="form__inline-actions">
      <button type="button" class="link-button" :disabled="working || waiting" @click="setup.start()">
        {{ t('chatgpt.reauthorize') }}
      </button>
    </div>

    <div v-if="connection?.needsConfirmation" class="form__notice" role="alertdialog" aria-modal="false" :aria-label="t('chatgpt.welcomeTitle')">
      <h4>{{ t('chatgpt.welcomeTitle') }}</h4>
      <p>{{ t('chatgpt.welcomeBody') }}</p>
      <button type="button" class="dg-button dg-button--primary" :disabled="working" @click="setup.confirm()">{{ t('chatgpt.acknowledge') }}</button>
    </div>
    <div v-if="connection?.paused" class="form__notice" role="status">
      <p>{{ t('chatgpt.limit') }}</p>
      <button type="button" class="dg-button" :disabled="working" @click="setup.resume()">{{ t('chatgpt.resume') }}</button>
    </div>

    <div v-if="connection?.state === 'ready' || connection?.state === 'connected'" class="form__grid">
      <label class="form__cell form__cell--wide">
        <span class="dg-field-label">{{ t('chatgpt.name') }}</span>
        <input v-model="name" class="dg-input" type="text" />
      </label>
      <fieldset class="form__cell form__cell--wide models">
        <legend class="dg-field-label">{{ t('chatgpt.models') }}</legend>
        <label v-for="model in connection.models" :key="model" class="model"><input v-model="selected" type="checkbox" :value="model" /> {{ model }}</label>
        <p v-if="connection.models.length === 0" class="form__hint">{{ t('chatgpt.noModels') }}</p>
      </fieldset>
    </div>

    <footer class="form__footer">
      <button v-if="provider?.hasSecret" type="button" class="dg-button dg-button--danger" :disabled="working || waiting" @click="disconnect">{{ t('chatgpt.disconnect') }}</button>
      <span class="form__actions">
        <button type="button" class="dg-button" @click="emit('done')">{{ t('common.action.cancel') }}</button>
        <button type="submit" class="dg-button dg-button--primary" :disabled="!canSave">{{ t('common.action.save') }}</button>
      </span>
    </footer>
  </form>
</template>

<style scoped>
/* Same card, grid, hints and footer as ProviderForm, so the two choices
   read as one form family. */
.form {
  display: flex;
  flex-direction: column;
  gap: 14px;
  min-width: 0;
  padding: 16px 18px;
  border: 1px solid var(--dg-card-border);
  border-radius: var(--dg-card-radius);
  background: var(--dg-card-fill);
  box-shadow: var(--dg-card-shadow);
}

.form__title { color: var(--dg-text-primary); font-size: 14px; font-weight: 600; }
.form__grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px 12px; }
.form__cell { display: flex; flex-direction: column; gap: 5px; min-width: 0; }
.form__cell--wide { grid-column: 1 / -1; }

.form__error, .form__hint, .form__status { font-size: 11px; line-height: 1.45; }
.form__error { color: var(--dg-danger); }
.form__hint { color: var(--dg-text-muted); }
.form__status { display: inline-flex; align-items: center; gap: 4px; color: var(--dg-text-secondary); overflow-wrap: anywhere; }
.form__status--ok { color: var(--dg-success); }

.form__inline-actions { display: inline-flex; gap: 12px; }
.link-button {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 0;
  border: none;
  background: none;
  color: var(--dg-accent-text);
  font-size: 12px;
  font-weight: 550;
  cursor: pointer;
}
.link-button:disabled { color: var(--dg-text-muted); cursor: default; }
.link-button:not(:disabled):hover { text-decoration: underline; }
.link-button:focus-visible { border-radius: 4px; outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }

.models { flex-direction: row; flex-wrap: wrap; gap: 6px 18px; border: none; padding: 0; margin: 0; }
.models legend { flex-basis: 100%; padding: 0; margin-bottom: 5px; }
.model { display: flex; align-items: center; gap: 6px; font-size: 12px; overflow-wrap: anywhere; }
.model input { accent-color: var(--dg-accent); }

.form__notice { display: grid; gap: 8px; padding: 12px 14px; border: 1px solid var(--dg-timeline-grid); border-radius: 8px; font-size: 12px; line-height: 1.6; }
.form__notice h4 { color: var(--dg-text-primary); font-size: 13px; font-weight: 600; }
.form__notice button { justify-self: start; }

.form__footer { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding-top: 12px; border-top: 1px solid var(--dg-timeline-grid); }
.form__actions { display: inline-flex; flex: none; gap: 8px; margin-left: auto; }

@media (max-width: 620px) {
  .form__grid { grid-template-columns: minmax(0, 1fr); }
  .form__footer { flex-direction: column; align-items: stretch; }
  .form__actions { justify-content: flex-end; }
}
</style>
