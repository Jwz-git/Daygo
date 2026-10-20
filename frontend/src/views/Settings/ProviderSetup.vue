<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ProviderDTO } from '@/api/dto'
import ChatGPTForm from './ChatGPTForm.vue'
import ProviderForm from './ProviderForm.vue'
const props = defineProps<{ provider: ProviderDTO | null }>()
const emit = defineEmits<{ done: [] }>()
const { t } = useI18n()
// New services start at the two-choice screen. Mount a form only after a pick.
const mode = ref<'chatgpt' | 'key' | null>(props.provider === null ? null : props.provider.protocol === 'chatgpt' ? 'chatgpt' : 'key')
</script>
<template>
  <div class="provider-setup">
    <div v-if="mode === null" class="choices" role="group" :aria-label="t('settings.providers.title')">
      <button type="button" class="dg-button" @click="mode = 'chatgpt'">{{ t('chatgpt.subscription') }}</button>
      <button type="button" class="dg-button" @click="mode = 'key'">{{ t('chatgpt.apiKey') }}</button>
    </div>
    <header v-else-if="provider === null" class="setup-head">
      <button type="button" class="setup-back" @click="mode = null">{{ t('onboarding.back') }}</button>
      <h3>{{ mode === 'chatgpt' ? t('chatgpt.subscription') : t('chatgpt.apiKey') }}</h3>
    </header>
    <ChatGPTForm v-if="mode === 'chatgpt'" :provider="provider" @done="emit('done')" />
    <ProviderForm v-else-if="mode === 'key'" :provider="provider" @done="emit('done')" />
  </div>
</template>
<style scoped>
.provider-setup { flex: 1; min-width: 0; }
.choices { display: flex; flex-wrap: wrap; gap: 12px; padding: 18px 0; }
.choices > button { flex: 1; min-width: 180px; min-height: 64px; font-size: 14px; }
.setup-head { display: flex; align-items: center; gap: 16px; padding-top: 14px; }
.setup-head h3 { color: var(--dg-text-primary); font-size: 14px; font-weight: 600; }
.setup-back { padding: 4px 0; border: none; background: none; color: var(--dg-text-secondary); font: inherit; cursor: pointer; }
.setup-back:hover { color: var(--dg-accent-text); }
.setup-back:focus-visible { outline: 2px solid var(--dg-accent); outline-offset: 3px; border-radius: 3px; }
</style>
