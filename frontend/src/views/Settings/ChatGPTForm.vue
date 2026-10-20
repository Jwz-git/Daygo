<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ProviderDTO } from '@/api/dto'
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
  <form class="chatgpt-form" @submit.prevent="save">
    <p class="hint">{{ t('chatgpt.description') }}</p>
    <div v-if="provider || failed || connection?.state === 'error' || connection?.state === 'disconnected'" class="actions">
      <button type="button" class="dg-button dg-button--primary" :disabled="working || waiting" @click="setup.start()">
        {{ t('chatgpt.reauthorize') }}
      </button>
    </div>
    <p v-if="waiting" role="status">{{ t('chatgpt.waiting') }}</p>
    <p v-else-if="connection?.state === 'connected' || connection?.state === 'ready'" class="connected" role="status">
      {{ t('chatgpt.connected') }}<span v-if="connection.email"> · {{ connection.email }}</span>
    </p>
    <p v-else-if="provider && connection?.state === 'disconnected'">{{ t('chatgpt.disconnected') }}</p>
    <p v-if="failed || connection?.state === 'error'" class="error" role="alert">{{ errorText }}</p>
    <p v-if="remoteUnconfirmed" class="error" role="alert">{{ t('chatgpt.remoteUnconfirmed') }}</p>
    <div v-if="connection?.needsConfirmation" class="notice" role="alertdialog" aria-modal="false" :aria-label="t('chatgpt.welcomeTitle')">
      <h4>{{ t('chatgpt.welcomeTitle') }}</h4>
      <p>{{ t('chatgpt.welcomeBody') }}</p>
      <button type="button" class="dg-button dg-button--primary" :disabled="working" @click="setup.confirm()">{{ t('chatgpt.acknowledge') }}</button>
    </div>
    <div v-if="connection?.paused" class="notice" role="status">
      <p>{{ t('chatgpt.limit') }}</p>
      <button type="button" class="dg-button" :disabled="working" @click="setup.resume()">{{ t('chatgpt.resume') }}</button>
    </div>
    <template v-if="connection?.state === 'ready' || connection?.state === 'connected'">
      <label class="field"><span class="dg-field-label">{{ t('chatgpt.name') }}</span><input v-model="name" class="dg-input" /></label>
      <fieldset class="models">
        <legend class="dg-field-label">{{ t('chatgpt.models') }}</legend>
        <label v-for="model in connection.models" :key="model" class="model"><input v-model="selected" type="checkbox" :value="model" /> {{ model }}</label>
        <p v-if="connection.models.length === 0" class="hint">{{ t('chatgpt.noModels') }}</p>
      </fieldset>
    </template>
    <div class="actions footer">
      <button v-if="provider?.hasSecret" type="button" class="dg-button" :disabled="working || waiting" @click="disconnect">{{ t('chatgpt.disconnect') }}</button>
      <button type="button" class="dg-button" @click="emit('done')">{{ t('common.action.cancel') }}</button>
      <button type="submit" class="dg-button dg-button--primary" :disabled="!canSave">{{ t('common.action.save') }}</button>
    </div>
  </form>
</template>

<style scoped>
.chatgpt-form { display: grid; gap: 14px; padding: 18px 2px; min-width: 0; width: 100%; font-size: 13px; }
.hint { color: var(--dg-text-secondary); line-height: 1.6; }
.actions { display: flex; flex-wrap: wrap; gap: 8px; }
.field { display: grid; gap: 6px; }
.models { display: flex; flex-wrap: wrap; gap: 10px 18px; border: none; padding: 8px 0; margin: 0; }
.model { display: flex; align-items: center; gap: 6px; overflow-wrap: anywhere; }
.model input { accent-color: var(--dg-accent); }
.notice { display: grid; gap: 10px; padding: 14px; border: 1px solid var(--dg-timeline-grid); border-radius: 10px; line-height: 1.6; }
.notice button { justify-self: start; }
.connected { color: var(--dg-success); overflow-wrap: anywhere; }
.error { color: var(--dg-danger); line-height: 1.6; }
.footer { justify-content: flex-end; }
</style>
