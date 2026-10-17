import { ref } from 'vue'

export interface TrainingStatus { active: boolean; until: number; limit: number; captured: number; skipped: number; failed: number }
export interface TrainingSummary { id: string; purpose: string; createdAt: number; model: string; verdict: string; privacyApproved: boolean; revision: number }
export interface TrainingSample { id: string; revision: number; purpose: string; model: string; prompt: string; schema: string; images: string[]; candidate: string; teachers: { model: string; answer: string }[]; label: string; verdict: string; privacyApproved: boolean; group: string }
export interface TrainingAPI {
 status(): Promise<TrainingStatus>
 list(): Promise<TrainingSummary[]>
 get(id: string): Promise<TrainingSample>
 start(minutes: number, limit: number, consent: boolean): Promise<void>
 stop(): Promise<void>
 review(id: string, revision: number, label: string, privacyApproved: boolean, verdict: string): Promise<void>
 remove(id: string): Promise<void>
 export(ids: string[]): Promise<string>
 relabel(id: string, revision: number, providerId: string, model: string, consent: boolean): Promise<void>
 openExports(): Promise<void>
 subscribe(callback: () => void): () => void
}

export function createTrainingSession(api: TrainingAPI) {
 const status = ref<TrainingStatus | null>(null)
 const samples = ref<TrainingSummary[]>([])
 const sample = ref<TrainingSample | null>(null)
 const label = ref('')
 const privacy = ref(false)
 const quality = ref(false)
 const busy = ref(false)
 const failed = ref(false)
 const exported = ref('')
 let selection = 0
 let refreshGeneration = 0
 let disposed = false
 let timer: ReturnType<typeof setInterval> | undefined
 let unsubscribe: (() => void) | undefined
 async function refresh() {
  const generation = ++refreshGeneration
  try {
   const [nextStatus, nextList] = await Promise.all([api.status(), api.list()])
   if (disposed || generation !== refreshGeneration) return
   status.value = nextStatus
   samples.value = nextList.sort((a,b) => b.createdAt-a.createdAt)
  } catch { if (!disposed) failed.value = true }
 }
 async function choose(id: string) {
  const generation = ++selection
  sample.value = null; label.value = ''; privacy.value = false; quality.value = false
  try {
   const next = await api.get(id)
   if (disposed || generation !== selection) return
   sample.value = next; label.value = next.label; privacy.value = next.privacyApproved
  } catch { if (!disposed) failed.value = true }
 }
 async function mutate(work: () => Promise<void>) {
  if (busy.value) return
  busy.value = true; failed.value = false
  try { await work() } catch { failed.value = true } finally { busy.value = false }
 }
 async function start(minutes: number, limit: number, consent: boolean) {
  if (!consent) return
  await mutate(() => api.start(minutes,limit,consent))
 }
 async function review(verdict: string) {
  const current=sample.value
  if (!current || (verdict === 'accepted' && !quality.value)) return
  await mutate(async () => {
   await api.review(current.id,current.revision,label.value,privacy.value,verdict)
   await choose(current.id)
  })
 }
 async function remove() {
  const current=sample.value;if (!current) return
  await mutate(async () => { await api.remove(current.id); selection++;sample.value=null;label.value='';privacy.value=false;quality.value=false })
 }
 async function exportApproved() {
  const ids=samples.value.filter(s => s.verdict==='accepted' && s.privacyApproved).slice(0,100).map(s => s.id)
  if (!ids.length) return
  await mutate(async () => { exported.value=await api.export(ids) })
 }
 async function relabel(providerId: string,model: string,consent: boolean) {
  const current=sample.value;if (!current || !consent) return
  await mutate(async () => { await api.relabel(current.id,current.revision,providerId,model,consent);await choose(current.id) })
 }
 return {status,samples,sample,label,privacy,quality,busy,failed,exported,choose,refresh,start,review,remove,exportApproved,relabel,
  stop: () => mutate(() => api.stop()), openExports: () => mutate(() => api.openExports()),
  initialize: async () => { unsubscribe=api.subscribe(() => { void refresh() });await refresh();if(!disposed) timer=setInterval(() => { void api.status().then(next=>{if(!disposed)status.value=next}).catch(()=>{if(!disposed)failed.value=true}) },5000) },
  dispose: () => {disposed=true;selection++;refreshGeneration++;unsubscribe?.();if(timer)clearInterval(timer)},
 }
}
