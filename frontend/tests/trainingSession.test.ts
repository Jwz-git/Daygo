import assert from 'node:assert/strict'
import test from 'node:test'
import { createTrainingSession, type TrainingAPI, type TrainingSample } from '../src/stores/trainingSession'

function fixture() {
 let notify=()=>{}
 let writes=0
 const sample: TrainingSample={id:'a',revision:1,purpose:'transcribe',model:'fixture',prompt:'anonymous',schema:'{}',images:[],candidate:'{}',teachers:[],label:'{}',verdict:'pending',privacyApproved:false,group:'fixture'}
 const api: TrainingAPI={
 status:async()=>({active:false,until:0,limit:0,captured:0,skipped:0,failed:0}),
 list:async()=>[{id:'a',purpose:sample.purpose,createdAt:1,model:'fixture',verdict:sample.verdict,privacyApproved:sample.privacyApproved,revision:sample.revision}],
 get:async()=>({...sample}), start:async()=>{writes++;notify()},stop:async()=>{notify()},
 review:async(_id,_revision,label,privacy,verdict)=>{writes++;sample.label=label;sample.privacyApproved=privacy;sample.verdict=verdict;sample.revision++;notify()},
 remove:async()=>{writes++;notify()},export:async()=>{writes++;return 'anonymous.zip'},relabel:async()=>{writes++;notify()},openExports:async()=>{},
 subscribe:fn=>{notify=fn;return()=>{notify=()=>{}}},
 }
 return {api,sample,writes:()=>writes}
}

test('training requires explicit collection, quality and sending consent',async()=>{
 const f=fixture(),s=createTrainingSession(f.api);await s.initialize();await s.choose('a')
 await s.start(60,50,false);await s.review('accepted');await s.relabel('p','m',false);await s.exportApproved()
 assert.equal(f.writes(),0)
 s.quality.value=true;s.privacy.value=true;await s.review('accepted')
 assert.equal(s.sample.value?.revision,2);assert.equal(s.quality.value,false)
 await s.refresh();await s.exportApproved();assert.equal(s.exported.value,'anonymous.zip')
 s.dispose()
})

test('failed mutations keep the draft and do not approve optimistically',async()=>{
 const f=fixture();f.api.review=async()=>{throw Error('fixture')};const s=createTrainingSession(f.api)
 await s.choose('a');s.label.value='edited';s.quality.value=true;await s.review('accepted')
 assert.equal(s.label.value,'edited');assert.equal(s.sample.value?.verdict,'pending');assert.equal(s.failed.value,true)
})

test('late sample reads cannot replace the latest selection',async()=>{
 const f=fixture();let resolve: ((s:TrainingSample)=>void)|undefined
 f.api.get=async id=>id==='a'?new Promise(r=>{resolve=r}):{...f.sample,id}
 const s=createTrainingSession(f.api);const first=s.choose('a');await s.choose('b');resolve?.({...f.sample});await first
 assert.equal(s.sample.value?.id,'b');s.dispose()
})
