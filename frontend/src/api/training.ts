import { app } from '../../wailsjs/go/models'
import * as backend from '../../wailsjs/go/app/Backend'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import type { TrainingAPI } from '@/stores/trainingSession'

function available(): void { if (!('go' in window) || window.go === undefined) throw new Error('wails_unavailable') }
export const trainingAPI: TrainingAPI = {
 status: async () => { available();return backend.GetTrainingStatus() },
 list: async () => { available();return (await backend.ListTrainingSamples()) ?? [] },
 get: async id => { available();const result=await backend.GetTrainingSample(id);return {...result,images:result.images??[],teachers:result.teachers??[]} },
 start: async (minutes,limit,consent) => {available();await backend.StartTrainingCapture(new app.TrainingStartDTO({minutes,limit,consent}))},
 stop: async () => {available();await backend.StopTrainingCapture()},
 review: async (id,revision,label,privacyApproved,verdict) => {available();await backend.ReviewTrainingSample(new app.TrainingReviewDTO({id,revision,label,privacyApproved,verdict}))},
 remove: async id => {available();await backend.DeleteTrainingSample(id)},
 export: async ids => {available();return backend.ExportTrainingSamples(ids)},
 relabel: async (id,revision,providerId,model,consent) => {available();await backend.RelabelTrainingSample(new app.TrainingTeacherDTO({id,revision,providerId,model,consent}))},
 openExports: async () => {available();await backend.OpenTrainingExports()},
 subscribe: callback => {if (!('go' in window) || window.go===undefined) return () => undefined;return EventsOn('training:updated',callback)},
}
