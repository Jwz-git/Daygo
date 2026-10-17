<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import PageHeader from '@/components/PageHeader.vue'
import { createTrainingSession } from '@/stores/trainingSession'
import { trainingAPI } from '@/api/training'
import { useProvidersStore } from '@/stores/providers'

const { t } = useI18n()
const session=createTrainingSession(trainingAPI)
const { status,samples,sample,label,privacy,quality,busy,failed,exported }=session
const providers=useProvidersStore()
const minutes=ref(60), limit=ref(50), consent=ref(false), sendConsent=ref(false), deleteConfirm=ref(false)
const providerId=ref(''),model=ref('')
const enlarged=ref('')
const chosenProvider=computed(() => providers.providers.find(p => p.id===providerId.value))
const exportable=computed(() => samples.value.some(s => s.privacyApproved && s.verdict==='accepted'))
watch(providerId, () => {model.value='';sendConsent.value=false})
watch(model, () => {sendConsent.value=false})
watch(() => sample.value?.id, () => {sendConsent.value=false;deleteConfirm.value=false})
onMounted(() => {void session.initialize();void providers.hydrate()})
onUnmounted(() => session.dispose())
function verdictLabel(value: string): string {
 if (value==='accepted') return t('training.accepted')
 if (value==='rejected') return t('training.rejected')
 return t('training.pending')
}
async function relabel() { await session.relabel(providerId.value,model.value,sendConsent.value);sendConsent.value=false }
</script>

<template>
 <div class="page training-page">
  <PageHeader :title="t('training.title')" />
  <div v-if="enlarged" class="image-overlay" role="dialog" aria-modal="true" :aria-label="t('training.previewHint')" @keydown.esc="enlarged=''">
   <button class="dg-button" @click="enlarged=''">{{ t('common.action.close') }}</button>
   <div class="image-scroll"><img :src="enlarged" :alt="t('training.previewHint')" /></div>
  </div>
  <div class="training-content dg-scroll">
   <p>{{ t('training.description') }}</p>
   <RouterLink class="dg-button" :to="{name:'settings',query:{section:'providers'}}">{{ t('modelPlayground.back') }}</RouterLink>
   <p v-if="failed" role="alert">{{ t('training.failed') }}</p>
   <fieldset :disabled="busy || !status">
    <legend>{{ t('training.capture') }}</legend>
    <div class="controls">
     <label>{{ t('training.minutes') }}<input v-model.number="minutes" type="number" min="1" max="480" /></label>
     <label>{{ t('training.limit') }}<input v-model.number="limit" type="number" min="1" max="100" /></label>
    </div>
    <label class="check"><input v-model="consent" type="checkbox" />{{ t('training.consent') }}</label>
    <div class="controls">
     <button class="dg-button dg-button--primary" :disabled="!consent || status?.active" @click="session.start(minutes,limit,consent)">{{ t('training.start') }}</button>
     <button class="dg-button" :disabled="!status?.active" @click="session.stop()">{{ t('training.stop') }}</button>
     <button class="dg-button" @click="session.refresh()">{{ t('training.refresh') }}</button>
     <span>{{ status?.active ? t('training.active') : t('training.inactive') }}</span>
    </div>
    <p v-if="status">{{ t('training.counters',{captured:status.captured,skipped:status.skipped,failed:status.failed}) }}</p>
   </fieldset>
   <div class="controls">
    <button class="dg-button" :disabled="busy || !exportable" @click="session.exportApproved()">{{ t('training.export') }}</button>
    <button class="dg-button" :disabled="busy || !exported" @click="session.openExports()">{{ t('training.openFolder') }}</button>
   </div>
   <p v-if="exported" role="status">{{ t('training.exported') }} <code>{{ exported }}</code></p>
   <div class="workspace">
    <aside>
     <p v-if="!samples.length">{{ t('training.empty') }}</p>
     <button v-for="item in samples" :key="item.id" class="sample-row" :class="{selected:sample?.id===item.id}" :disabled="busy" @click="session.choose(item.id)">
      <strong>{{ item.purpose==='transcribe' ? t('training.transcribe') : t('training.cards') }}</strong>
      <span>{{ item.model }}</span><span>{{ verdictLabel(item.verdict) }}</span>
      <time>{{ new Date(item.createdAt*1000).toLocaleString() }}</time>
     </button>
    </aside>
    <section v-if="sample" class="review">
     <p>{{ t('training.previewHint') }}</p>
     <div class="images"><button v-for="(src,index) in sample.images" :key="index" class="image-button" @click="enlarged=src"><img :src="src" :alt="t('training.image',{index:index+1})" /></button></div>
     <details><summary>{{ t('training.prompt') }}</summary><pre>{{ sample.prompt }}</pre></details>
     <details><summary>{{ t('training.schema') }}</summary><pre>{{ sample.schema }}</pre></details>
     <details><summary>{{ t('training.candidate') }}</summary><pre>{{ sample.candidate }}</pre><pre v-for="(teacher,index) in sample.teachers" :key="index">{{ teacher.model }}{{ '\n' }}{{ teacher.answer }}</pre></details>
     <fieldset :disabled="busy">
      <legend>{{ t('training.label') }}</legend>
      <textarea v-model="label" :aria-label="t('training.label')" rows="16" spellcheck="false" />
      <label class="check"><input v-model="privacy" type="checkbox" />{{ t('training.privacy') }}</label>
      <label class="check"><input v-model="quality" type="checkbox" />{{ t('training.quality') }}</label>
      <div class="controls">
       <button class="dg-button dg-button--primary" :disabled="!quality" @click="session.review('accepted')">{{ t('training.accept') }}</button>
       <button class="dg-button" @click="session.review('pending')">{{ t('common.action.save') }}</button>
       <button class="dg-button" @click="session.review('rejected')">{{ t('training.reject') }}</button>
      </div>
     </fieldset>
     <fieldset :disabled="busy">
      <legend>{{ t('training.teacher') }}</legend>
      <div class="controls">
       <label>{{ t('modelPlayground.provider') }}<select v-model="providerId"><option value="">{{ t('modelPlayground.select') }}</option><option v-for="p in providers.providers" :key="p.id" :value="p.id">{{ p.displayName }}</option></select></label>
       <label>{{ t('modelPlayground.model') }}<select v-model="model"><option value="">{{ t('modelPlayground.select') }}</option><option v-for="m in chosenProvider?.models??[]" :key="m" :value="m">{{ m }}</option></select></label>
      </div>
      <label class="check"><input v-model="sendConsent" type="checkbox" />{{ t('training.sendConsent') }}</label>
      <button class="dg-button" :disabled="!sendConsent || !providerId || !model || sample.teachers.length>=4" @click="relabel()">{{ t('training.relabel') }}</button>
     </fieldset>
     <label class="check"><input v-model="deleteConfirm" type="checkbox" :disabled="busy" />{{ t('training.deleteConfirm') }}</label>
     <button class="dg-button" :disabled="busy || !deleteConfirm" @click="session.remove()">{{ t('common.action.delete') }}</button>
    </section>
   </div>
  </div>
 </div>
</template>

<style scoped>
.training-page{height:100%;display:flex;flex-direction:column;min-height:0}
.training-content{overflow:auto;padding:20px 28px;min-height:0;max-width:1400px;width:100%;box-sizing:border-box}
fieldset{border:1px solid var(--border-subtle,#8884);border-radius:10px;padding:16px;margin:18px 0;min-width:0}
legend{font-weight:600}.controls{display:flex;gap:12px;align-items:center;flex-wrap:wrap;margin:12px 0}
label{display:flex;gap:8px;align-items:center}.check{align-items:flex-start;margin:12px 0;line-height:1.6}
input[type=number]{width:90px}input,select,textarea{font:inherit;color:inherit;background:transparent;border:1px solid #8886;border-radius:5px;padding:6px}
.workspace{display:grid;grid-template-columns:240px minmax(0,1fr);gap:24px;margin-top:24px}.sample-row{display:flex;flex-direction:column;gap:6px;text-align:left;border:1px solid #8884;background:transparent;color:inherit;padding:12px;width:100%;margin-bottom:8px;border-radius:8px;overflow-wrap:anywhere;cursor:pointer}.sample-row.selected{border-color:currentColor}.sample-row time,.sample-row span{font-size:12px}
.review{min-width:0}.images{display:grid;grid-template-columns:repeat(auto-fit,minmax(240px,1fr));gap:8px}.images img{width:100%;height:180px;object-fit:contain}.image-button{padding:0;border:0;background:transparent;cursor:zoom-in}.image-overlay{position:fixed;inset:0;z-index:1000;background:var(--surface-primary,#202024);padding:20px;display:flex;flex-direction:column;gap:10px}.image-overlay>button{align-self:flex-start}.image-scroll{overflow:auto;flex:1}.image-scroll img{max-width:none;display:block}
pre{white-space:pre-wrap;overflow-wrap:anywhere;font-size:12px;max-height:400px;overflow:auto;background:#8881;padding:12px}textarea{width:100%;box-sizing:border-box;font-family:monospace;resize:vertical}details{margin:12px 0}code{overflow-wrap:anywhere}button:disabled{opacity:.45;cursor:default}
@media(max-width:800px){.workspace{grid-template-columns:1fr}.training-content{padding:16px}.workspace aside{max-height:220px;overflow:auto}}
</style>
