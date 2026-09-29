<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import PageHeader from '@/components/PageHeader.vue'
import { useModelPlayground } from '@/stores/modelPlayground'
import ModelReply from './ModelReply.vue'

const { t, te } = useI18n()
const route = useRoute()
const state = useModelPlayground()
const { providerId, provider, model, text, imageURL, imageError, reading, accessError, textTooLong,
  canSend, busy, result, error } = state
const copyStatus = ref('')
void state.initialize(route.query.providerId, route.query.model)

const failure = computed(() => {
  if (error.value) return t('modelPlayground.failed')
  if (!result.value || result.value.ok) return ''
  if (result.value.errorCode === 'invalid_output') return t('modelPlayground.invalidOutput')
  if (result.value.errorCode === 'timeout') return t('modelPlayground.timeout')
  const key = `settings.providers.test.error.${result.value.errorCode}`
  return te(key) ? t(key) : t('modelPlayground.failed')
})

function chooseImage(event: Event): void {
  if (!(event.target instanceof HTMLInputElement)) return
  void state.selectImage(event.target.files?.[0])
  event.target.value = ''
}
function dropImage(event: DragEvent): void {
  void state.selectImage(event.dataTransfer?.files[0])
}
function pasteImage(event: ClipboardEvent): void {
  const image = Array.from(event.clipboardData?.files ?? []).find((file) => file.type.startsWith('image/'))
  if (image) { event.preventDefault(); void state.selectImage(image) }
}
async function send(): Promise<void> { copyStatus.value = ''; await state.sendInput() }
async function copy(): Promise<void> {
  try { await navigator.clipboard.writeText(result.value?.text ?? ''); copyStatus.value = 'copied' }
  catch { copyStatus.value = 'copyFailed' }
}
</script>

<template>
  <div class="page playground-page">
    <PageHeader :title="t('modelPlayground.title')" />
    <div class="playground-content dg-scroll" @paste="pasteImage">
      <header class="playground-intro">
        <p>{{ t('modelPlayground.description') }}</p>
        <RouterLink class="dg-button" :to="{ name: 'settings', query: { section: 'providers' } }">{{ t('modelPlayground.back') }}</RouterLink>
      </header>
      <p v-if="accessError" role="alert">{{ t(`modelPlayground.${accessError}`) }}</p>
      <div class="playground-selectors">
        <label>{{ t('modelPlayground.provider') }}
          <select v-model="providerId" class="dg-input" :disabled="busy">
            <option value="" disabled>{{ t('modelPlayground.select') }}</option>
            <option v-for="item in state.providers.providers" :key="item.id" :value="item.id">{{ item.displayName }}</option>
          </select>
        </label>
        <label>{{ t('modelPlayground.model') }}
          <select v-model="model" class="dg-input" :disabled="busy || !provider">
            <option value="" disabled>{{ t('modelPlayground.select') }}</option>
            <option v-for="item in provider?.models ?? []" :key="item" :value="item">{{ item }}</option>
          </select>
        </label>
      </div>
      <p v-if="provider && !provider.hasSecret" role="status">{{ t('modelPlayground.missingSecret') }}</p>
      <div class="playground-columns">
        <section class="playground-panel dg-card" @dragover.prevent @drop.prevent="dropImage">
          <h2>{{ t('modelPlayground.input') }}</h2>
          <label class="image-picker">{{ t('modelPlayground.image') }}
            <input type="file" accept="image/png,image/jpeg" :disabled="busy" @change="chooseImage">
          </label>
          <p class="playground-hint">{{ t('modelPlayground.imageHint') }}</p>
          <img v-if="imageURL" :src="imageURL" :alt="t('modelPlayground.image')" class="image-preview">
          <button v-if="imageURL || imageError || reading" type="button" class="dg-button" :disabled="busy" @click="state.removeImage">{{ t('modelPlayground.remove') }}</button>
          <p v-if="imageError" class="playground-error" role="alert">{{ t('modelPlayground.imageInvalid') }}</p>
          <label for="playground-text">{{ t('modelPlayground.prompt') }}</label>
          <textarea id="playground-text" v-model="text" class="dg-input" rows="7" maxlength="32000" :disabled="busy" :placeholder="t('modelPlayground.placeholder')" />
          <p v-if="textTooLong" class="playground-error" role="alert">{{ t('modelPlayground.textLimit') }}</p>
          <p class="playground-hint">{{ t('modelPlayground.notice') }}</p>
          <button type="button" class="dg-button dg-button--primary" :disabled="!canSend" @click="send">{{ busy ? t('modelPlayground.sending') : t('modelPlayground.send') }}</button>
        </section>
        <section class="playground-panel dg-card" :aria-busy="busy">
          <h2>{{ t('modelPlayground.output') }}</h2>
          <p v-if="busy" role="status">{{ t('modelPlayground.sending') }}</p>
          <p v-else-if="failure" class="playground-error" role="alert">{{ failure }}</p>
          <template v-else-if="result?.ok">
            <p role="status">{{ t('modelPlayground.success') }}</p>
            <p class="playground-hint" role="status">{{ t('modelPlayground.resultMeta', { model: result.model, latency: result.latencyMs }) }}</p>
            <ModelReply :text="result.text" />
            <button type="button" class="dg-button" @click="copy">{{ t('modelPlayground.copy') }}</button>
            <p v-if="copyStatus" role="status">{{ t(`modelPlayground.${copyStatus}`) }}</p>
          </template>
          <p v-else class="playground-hint">{{ t('modelPlayground.empty') }}</p>
        </section>
      </div>
    </div>
  </div>
</template>

<style scoped>
.playground-content { padding: 0 var(--dg-page-padding) var(--dg-page-padding); display: flex; flex-direction: column; gap: 20px; }
.playground-intro { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.playground-selectors, .playground-columns { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 20px; }
.playground-selectors label { display: flex; flex-direction: column; gap: 8px; min-width: 0; }
.playground-panel { display: flex; flex-direction: column; align-items: stretch; gap: 12px; padding: 20px; min-width: 0; }
.playground-panel h2 { font-size: 16px; margin: 0; }
.playground-panel button { align-self: flex-start; }
.image-picker { display: flex; flex-direction: column; gap: 8px; }
.image-picker input { max-width: 100%; }
.image-preview { width: 100%; max-height: 280px; object-fit: contain; border-radius: 8px; }
.playground-hint { color: var(--dg-text-secondary); font-size: 12px; line-height: 1.6; margin: 0; overflow-wrap: anywhere; }
.playground-error { color: var(--dg-danger); }
textarea { resize: vertical; width: 100%; min-height: 150px; }
select { width: 100%; min-width: 0; }
@media (max-width: 800px) { .playground-selectors, .playground-columns { grid-template-columns: minmax(0, 1fr); } }
</style>
