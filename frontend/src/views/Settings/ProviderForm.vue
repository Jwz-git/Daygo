<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import ComboBox from '@/components/ComboBox.vue'

import {
  PROVIDER_PROTOCOLS,
  type ProviderDTO,
  type ProviderModelsResult,
  type ProviderProtocol,
  type ProviderTestResult,
} from '@/api/dto'
import { listProviderModels } from '@/api/providers'
import { testProviderConnection, WAILS_UNAVAILABLE } from '@/api/providerTest'
import {
  DEFAULT_ENDPOINTS,
  draftOf,
  emptyDraft,
  MAX_PROVIDER_MODELS,
  useProvidersStore,
  type ProviderDraft,
  type ProviderErrors,
  type ProviderField,
} from '@/stores/providers'

/*
 * The add/edit form owns the full draft lifecycle: the draft, its validation
 * errors, the draft connection test and the model listing. The section opens
 * it through the exposed openEdit/closeIfEditing; the add button lives here
 * because it toggles with the form.
 */
const { t, te } = useI18n()
const store = useProvidersStore()

const formOpen = ref(false)
/** null while adding, a provider id while editing. */
const editingId = ref<string | null>(null)
const errors = ref<ProviderErrors>({})
const draft = reactive<ProviderDraft>(emptyDraft())

const nameInput = ref<HTMLInputElement | null>(null)

const modelPlaceholder = computed(() =>
  t(`settings.providers.form.modelPlaceholder.${draft.protocol}`),
)

// Plain HTTP is accepted, only flagged: localhost gateways have no TLS story.
const isPlainHttp = computed(() => draft.endpoint.trim().startsWith('http://'))

type TestState =
  | { phase: 'idle' }
  | { phase: 'running' }
  | { phase: 'done'; result: ProviderTestResult }

const testState = ref<TestState>({ phase: 'idle' })

/** Which model the draft probe runs against; '' picks the first filled one. */
const testModel = ref('')

type ModelsState =
  | { phase: 'idle' }
  | { phase: 'fetching' }
  | { phase: 'done'; result: ProviderModelsResult }

const modelsState = ref<ModelsState>({ phase: 'idle' })

/** The model a draft probe actually sends: the picked one, else the first. */
const resolvedTestModel = computed(() => {
  const picked = testModel.value.trim()
  if (picked !== '' && filledModels.value.includes(picked)) return picked
  return filledModels.value[0] ?? ''
})

/** Dropdown options for each model combobox; free text stays allowed. */
const modelOptions = computed(() => {
  const state = modelsState.value
  if (state.phase !== 'done' || !state.result.ok) return []
  return state.result.models.map((model) => ({ value: model, label: model }))
})

/** The models that carry a non-blank id; what the test dropdown offers. */
const filledModels = computed(() =>
  draft.models.map((model) => model.trim()).filter((model) => model !== ''),
)

const canAddModel = computed(() => draft.models.length < MAX_PROVIDER_MODELS)

function addModelRow(): void {
  if (!canAddModel.value) return
  draft.models.push('')
}

function removeModelRow(index: number): void {
  draft.models.splice(index, 1)
  // The list never goes empty: a lone blank row keeps one editable field.
  if (draft.models.length === 0) draft.models.push('')
  if (testModel.value !== '' && !filledModels.value.includes(testModel.value)) {
    testModel.value = ''
  }
}

function setModel(index: number, value: string): void {
  draft.models[index] = value
}

/** Fill any blank rows (then append) with a fetched list, deduped in order. */
function applyFetchedModels(models: string[]): void {
  const have = new Set(filledModels.value)
  const additions = models.filter((model) => model.trim() !== '' && !have.has(model.trim()))
  if (additions.length === 0) return
  const kept = draft.models.filter((model) => model.trim() !== '')
  const merged = [...kept, ...additions].slice(0, MAX_PROVIDER_MODELS)
  draft.models = merged.length > 0 ? merged : ['']
}

// A result describes the draft as it was when tested; any later edit makes it
// stale, so it clears instead of lingering next to a different configuration.
watch(draft, () => {
  testState.value = { phase: 'idle' }
  if (modelsState.value.phase === 'done') {
    modelsState.value = { phase: 'idle' }
  }
})

/** The typed key, or — while editing — nothing: the stored key is in the
 * keychain and never comes back, so a saved provider with no new key cannot
 * run a draft probe; the card's test button covers that case. */
const canTest = computed(
  () =>
    draft.endpoint.trim() !== '' &&
    resolvedTestModel.value !== '' &&
    draft.secret.trim() !== '',
)

const canFetchModels = computed(
  () =>
    draft.endpoint.trim() !== '' &&
    draft.secret.trim() !== '',
)

function testFailureText(result: ProviderTestResult): string {
  const key = `settings.providers.test.error.${result.errorCode}`
  const known = te(key) ? t(key) : ''
  return known === '' ? result.message : known
}

function modelsFailureText(result: ProviderModelsResult): string {
  const key = `settings.providers.test.error.${result.errorCode}`
  const known = te(key) ? t(key) : ''
  return known === '' ? result.message : known
}

async function runTest(): Promise<void> {
  if (testState.value.phase === 'running' || !canTest.value) return
  testState.value = { phase: 'running' }
  try {
    const result = await testProviderConnection({
      protocol: draft.protocol,
      endpoint: draft.endpoint.trim(),
      model: resolvedTestModel.value,
      secret: draft.secret.trim(),
    })
    testState.value = { phase: 'done', result }
  } catch (error) {
    testState.value = {
      phase: 'done',
      result: {
        ok: false,
        model: '',
        latencyMs: 0,
        capabilities: [],
        errorCode: error instanceof Error && error.message === WAILS_UNAVAILABLE
          ? WAILS_UNAVAILABLE
          : 'unavailable',
        message: error instanceof Error ? error.message : String(error),
      },
    }
  }
}

async function fetchModels(): Promise<void> {
  if (modelsState.value.phase === 'fetching' || !canFetchModels.value) return
  modelsState.value = { phase: 'fetching' }
  try {
    const result = await listProviderModels({
      protocol: draft.protocol,
      endpoint: draft.endpoint.trim(),
      secret: draft.secret.trim(),
    })
    modelsState.value = { phase: 'done', result }
    if (result.ok) applyFetchedModels(result.models)
  } catch {
    modelsState.value = {
      phase: 'done',
      result: {
        ok: false,
        models: [],
        errorCode: 'unavailable',
        message: t('settings.providers.models.unavailable'),
      },
    }
  }
}

function protocolLabel(protocol: ProviderProtocol): string {
  return t(`settings.providers.protocol.${protocol}`)
}

function errorText(field: ProviderField): string {
  const code = errors.value[field]
  return code === undefined ? '' : t(`settings.providers.error.${code}`)
}

function resetDraft(next: ProviderDraft): void {
  Object.assign(draft, next)
}

function openAdd(): void {
  resetDraft(emptyDraft())
  editingId.value = null
  errors.value = {}
  testModel.value = ''
  formOpen.value = true
  void nextTick(() => nameInput.value?.focus())
}

function openEdit(provider: ProviderDTO): void {
  resetDraft(draftOf(provider))
  editingId.value = provider.id
  errors.value = {}
  testModel.value = ''
  formOpen.value = true
  void nextTick(() => nameInput.value?.focus())
}

/** Also the only place the typed key leaves component state. */
function closeForm(): void {
  resetDraft(emptyDraft())
  editingId.value = null
  errors.value = {}
  testModel.value = ''
  formOpen.value = false
  testState.value = { phase: 'idle' }
  modelsState.value = { phase: 'idle' }
}

/** A removed provider cannot stay open in the editor. */
function closeIfEditing(id: string): void {
  if (editingId.value === id) closeForm()
}

defineExpose({ openEdit, closeIfEditing })

async function submit(): Promise<void> {
  const id = editingId.value
  const failures = id === null ? await store.add(draft) : await store.update(id, draft)

  if (failures !== null) {
    errors.value = failures
    return
  }
  closeForm()
}

function onProtocolChange(event: Event): void {
  const next = (event.target as HTMLSelectElement).value as ProviderProtocol
  // Swap the suggested base URL only while the field still holds a suggestion,
  // so a hand-typed endpoint is never overwritten.
  const current = draft.endpoint.trim()
  if (current === '' || current === DEFAULT_ENDPOINTS[draft.protocol]) {
    draft.endpoint = DEFAULT_ENDPOINTS[next]
  }
  draft.protocol = next
}
</script>

<template>
  <form v-if="formOpen" class="form dg-card" @submit.prevent="submit">
    <h2 class="form__title">
      {{
        editingId === null
          ? t('settings.providers.form.addTitle')
          : t('settings.providers.form.editTitle')
      }}
    </h2>

    <div class="form__grid">
      <label class="form__cell">
        <span class="dg-field-label">{{ t('settings.providers.form.name') }}</span>
        <input
          ref="nameInput"
          v-model="draft.displayName"
          class="dg-input"
          type="text"
          :placeholder="t('settings.providers.form.namePlaceholder')"
          :aria-invalid="errors.displayName ? 'true' : undefined"
        />
        <p v-if="errors.displayName" class="form__error">
          {{ errorText('displayName') }}
        </p>
      </label>

      <label class="form__cell">
        <span class="dg-field-label">
          {{ t('settings.providers.protocol.label') }}
        </span>
        <select class="dg-input" :value="draft.protocol" @change="onProtocolChange">
          <option v-for="option in PROVIDER_PROTOCOLS" :key="option" :value="option">
            {{ protocolLabel(option) }}
          </option>
        </select>
      </label>

      <label class="form__cell form__cell--wide">
        <span class="dg-field-label">
          {{ t('settings.providers.form.endpoint') }}
        </span>
        <input
          v-model="draft.endpoint"
          class="dg-input"
          type="url"
          inputmode="url"
          spellcheck="false"
          :placeholder="DEFAULT_ENDPOINTS[draft.protocol]"
          :aria-invalid="errors.endpoint ? 'true' : undefined"
        />
        <p v-if="errors.endpoint" class="form__error">{{ errorText('endpoint') }}</p>
        <p v-else-if="isPlainHttp" class="form__warning">
          {{ t('settings.providers.form.httpWarning') }}
        </p>
      </label>

      <div class="form__cell form__cell--wide">
        <span class="dg-field-label">{{ t('settings.providers.form.models') }}</span>
        <ul class="model-list">
          <li v-for="(model, index) in draft.models" :key="index" class="model-list__row">
            <ComboBox
              :model-value="model"
              class="model-list__combo"
              :options="modelOptions"
              :placeholder="modelPlaceholder"
              :aria-label="t('settings.providers.form.modelAria', { index: index + 1 })"
              @update:model-value="(value) => setModel(index, value)"
            />
            <button
              type="button"
              class="dg-button dg-button--icon"
              :disabled="draft.models.length === 1 && model.trim() === ''"
              :aria-label="t('settings.providers.form.removeModel')"
              @click="removeModelRow(index)"
            >
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
                <line x1="5" y1="12" x2="19" y2="12" />
              </svg>
            </button>
          </li>
        </ul>
        <div class="model-list__actions">
          <button
            type="button"
            class="dg-button"
            :disabled="!canAddModel"
            @click="addModelRow"
          >
            {{ t('settings.providers.form.addModel') }}
          </button>
          <button
            type="button"
            class="dg-button"
            :disabled="!canFetchModels || modelsState.phase === 'fetching'"
            @click="fetchModels"
          >
            {{ t('settings.providers.models.fetch') }}
          </button>
        </div>
        <p v-if="errors.models" class="form__error">{{ errorText('models') }}</p>
        <p v-else-if="modelsState.phase === 'fetching'" class="form__hint" role="status" aria-live="polite">
          {{ t('settings.providers.models.fetching') }}
        </p>
        <p
          v-else-if="modelsState.phase === 'done' && !modelsState.result.ok"
          class="form__error"
          role="status"
          aria-live="polite"
        >
          {{ modelsFailureText(modelsState.result) }}
        </p>
        <p
          v-else-if="modelsState.phase === 'done' && modelsState.result.models.length === 0"
          class="form__hint"
          role="status"
          aria-live="polite"
        >
          {{ t('settings.providers.models.empty') }}
        </p>
        <p v-else class="form__hint">{{ t('settings.providers.form.modelsHint') }}</p>
      </div>

      <label class="form__cell">
        <span class="dg-field-label">
          {{ t('settings.providers.form.maxImages') }}
        </span>
        <input
          v-model.number="draft.maxImages"
          class="dg-input"
          type="number"
          min="0"
          max="20"
          step="1"
          :aria-invalid="errors.maxImages ? 'true' : undefined"
        />
        <p v-if="errors.maxImages" class="form__error">
          {{ errorText('maxImages') }}
        </p>
        <p v-else class="form__hint">
          {{ t('settings.providers.form.maxImagesHint') }}
        </p>
      </label>

      <label class="form__cell">
        <span class="dg-field-label">{{ t('settings.providers.form.apiKey') }}</span>
        <div class="form__key-row">
          <input
            v-model="draft.secret"
            class="dg-input"
            type="password"
            autocomplete="off"
            spellcheck="false"
            :placeholder="t('settings.providers.form.apiKeyPlaceholder')"
          />
          <button
            type="button"
            class="dg-button"
            :disabled="!canTest || testState.phase === 'running'"
            @click="runTest"
          >
            {{ t('settings.providers.test.run') }}
          </button>
        </div>
        <label v-if="filledModels.length > 1" class="form__test-model">
          <span class="dg-field-label">{{ t('settings.providers.test.model') }}</span>
          <select v-model="testModel" class="dg-input">
            <option value="">
              {{ t('settings.providers.test.modelDefault', { model: filledModels[0] }) }}
            </option>
            <option v-for="model in filledModels" :key="model" :value="model">
              {{ model }}
            </option>
          </select>
        </label>
        <p v-if="editingId !== null" class="form__hint">
          {{ t('settings.providers.form.apiKeyKeepHint') }}
        </p>
        <p v-if="testState.phase === 'running'" class="form__hint" role="status" aria-live="polite">
          {{ t('settings.providers.test.running') }}
        </p>
        <p
          v-else-if="testState.phase === 'done' && testState.result.ok"
          class="form__test-ok"
          role="status"
          aria-live="polite"
        >
          {{
            t('settings.providers.test.passed', {
              model: testState.result.model,
              latency: testState.result.latencyMs,
            })
          }}
        </p>
        <p
          v-else-if="testState.phase === 'done'"
          class="form__error"
          role="status"
          aria-live="polite"
        >
          {{ testFailureText(testState.result) }}
        </p>
      </label>
    </div>

    <div class="form__actions">
      <button type="button" class="dg-button" @click="closeForm">
        {{ t('common.action.cancel') }}
      </button>
      <button type="submit" class="dg-button dg-button--primary">
        {{ t('common.action.save') }}
      </button>
    </div>
  </form>

  <button v-else type="button" class="dg-button dg-button--primary add" @click="openAdd">
    {{ t('settings.providers.add') }}
  </button>
</template>

<style scoped>
.form {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 17px 18px;
}

.form__title {
  color: var(--dg-text-primary);
  font-size: 14px;
  font-weight: 600;
}

.form__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.form__cell--wide {
  grid-column: 1 / -1;
}

.form__error,
.form__hint,
.form__warning {
  margin-top: 4px;
  font-size: 11px;
}

.form__error {
  color: var(--dg-danger);
}

.form__warning {
  color: var(--dg-warning);
}

.form__key-row {
  display: flex;
  gap: 8px;
}

.form__key-row .dg-input,
.form__key-row .combo {
  flex: 1;
  min-width: 0;
}

.model-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.model-list__row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.model-list__combo {
  flex: 1;
  min-width: 0;
}

.model-list__actions {
  display: flex;
  gap: 8px;
  margin-top: 8px;
}

.dg-button--icon {
  flex: none;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  padding: 0;
}

.form__test-model {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-top: 8px;
}

.form__test-ok {
  color: var(--dg-accent-text);
}

.form__hint {
  color: var(--dg-text-muted);
}

.form__actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.add {
  align-self: flex-start;
}

@media (max-width: 620px) {
  .form__grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
