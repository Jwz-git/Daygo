<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ProviderDTO } from '@/api/dto'
import ChatGPTForm from './ChatGPTForm.vue'
import ProviderForm from './ProviderForm.vue'
const props = defineProps<{ provider: ProviderDTO | null }>()
const emit = defineEmits<{ done: [] }>()
const { t } = useI18n()
const mode = ref<'chatgpt' | 'key'>(props.provider && props.provider.protocol !== 'chatgpt' ? 'key' : 'chatgpt')
</script>
<template>
  <div class="provider-setup">
    <div v-if="provider === null" class="choices" role="group">
      <button type="button" class="dg-button" :class="{ 'dg-button--primary': mode === 'chatgpt' }" :aria-pressed="mode === 'chatgpt'" @click="mode = 'chatgpt'">{{ t('chatgpt.subscription') }}</button>
      <button type="button" class="dg-button" :class="{ 'dg-button--primary': mode === 'key' }" :aria-pressed="mode === 'key'" @click="mode = 'key'">{{ t('chatgpt.apiKey') }}</button>
    </div>
    <ChatGPTForm v-if="mode === 'chatgpt'" :provider="provider" @done="emit('done')" />
    <ProviderForm v-else :provider="provider" @done="emit('done')" />
  </div>
</template>
<style scoped>
.provider-setup { flex: 1; min-width: 0; }
.choices { display: flex; flex-wrap: wrap; gap: 8px; padding-top: 12px; }
</style>
