<script setup lang="ts">
import DgIcon from '@/components/DgIcon.vue'
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import DgSelect, { type DgSelectOption } from '@/components/DgSelect.vue'

import {
  PROVIDER_PROTOCOLS,
  type ProviderDTO,
  type ProviderModelsResult,
  type ProviderProtocol,
} from '@/api/dto'
import { listProviderModels } from '@/api/providers'
import {
  DEFAULT_ENDPOINTS,
  draftOf,
  emptyDraft,
  MAX_PROVIDER_IMAGES,
  MAX_PROVIDER_MODELS,
  useProvidersStore,
  type ProviderDraft,
  type ProviderErrors,
  type ProviderField,
} from '@/stores/providers'
import { USER_AGENT_PRESETS } from './userAgentPresets'

/*
 * The add/edit form owns the full draft lifecycle: the draft, its validation
 * errors and the model listing. The section mounts it at the top of the list
 * to add a service (provider = null) or in place of a service's row to edit
 * it, and drops it on `done`; the typed key never outlives the component.
 */
const props = defineProps<{ provider: ProviderDTO | null }>()
const emit = defineEmits<{ done: [] }>()

const { t, te } = useI18n()
const store = useProvidersStore()

/** null while adding, a provider id while editing. */
const editingId = computed(() => props.provider?.id ?? null)
const errors = ref<ProviderErrors>({})
const draft = reactive<ProviderDraft>(props.provider === null ? emptyDraft() : draftOf(props.provider))

draft.models = [...new Set(draft.models.map((model) => model.trim()).filter(Boolean))]

const nameInput = ref<HTMLInputElement | null>(null)

const modelPlaceholder = computed(() =>
  t(`settings.providers.form.modelPlaceholder.${draft.protocol}`),
)

// Plain HTTP is accepted, only flagged: localhost gateways have no TLS story.
const isPlainHttp = computed(() => draft.endpoint.trim().startsWith('http://'))

type ModelsState =
  | { phase: 'idle' }
  | { phase: 'fetching' }
  | { phase: 'done'; result: ProviderModelsResult }

const modelsState = ref<ModelsState>({ phase: 'idle' })

const modelQuery = ref('')
const modelOptions = computed(() => {
  const state = modelsState.value
  if (state.phase !== 'done' || !state.result.ok) return []
  return [...new Set(state.result.models)]
})
const filteredModelOptions = computed(() => {
  const query = modelQuery.value.trim().toLowerCase()
  return modelOptions.value.filter((model) => model.toLowerCase().includes(query))
})
const selectedModels = computed(() => new Set(draft.models))
const manualModel = ref('')
const canAddModel = computed(() => draft.models.length < MAX_PROVIDER_MODELS)
const pendingModel = computed(() => {
  const model = manualModel.value.trim()
  return selectedModels.value.has(model) ? '' : model
})
const canPickModel = computed(() =>
  draft.models.length + Number(pendingModel.value !== '') < MAX_PROVIDER_MODELS,
)

function removeModel(model: string): void {
  draft.models = draft.models.filter((value) => value !== model)
}

function toggleModel(model: string, event: Event): void {
  if (!(event.target as HTMLInputElement).checked) {
    removeModel(model)
    return
  }
  if (selectedModels.value.has(model) || !canPickModel.value) return
  draft.models.push(model)
}

function addManualModel(): void {
  if (!canAddModel.value || pendingModel.value === '') return
  draft.models.push(pendingModel.value)
  manualModel.value = ''
}

// Only source changes invalidate candidates. Editing the selection keeps the list available.
let modelsRevision = 0
watch(
  () => [draft.protocol, draft.endpoint, draft.secret, draft.userAgent],
  () => {
    modelsRevision++
    modelsState.value = { phase: 'idle' }
  },
  { flush: 'sync' },
)
onBeforeUnmount(() => { modelsRevision++ })

/*
 * A saved service with its key in the keychain can list models without the
 * key being typed again: while the draft still points at the saved protocol
 * and address and no new key is entered, the request names the provider and
 * Go reads the stored key (ListProviderModels' providerId form).
 */
const usesStoredKey = computed(() => {
  const saved = props.provider
  return saved !== null
    && saved.hasSecret
    && draft.secret.trim() === ''
    && draft.protocol === saved.protocol
    && draft.endpoint.trim() === saved.endpoint
})

const canFetchModels = computed(
  () =>
    draft.endpoint.trim() !== '' &&
    (draft.secret.trim() !== '' || usesStoredKey.value),
)

function modelsFailureText(result: ProviderModelsResult): string {
  const key = `settings.providers.test.error.${result.errorCode}`
  const known = te(key) ? t(key) : ''
  return known === '' ? result.message : known
}

async function fetchModels(): Promise<void> {
  if (modelsState.value.phase === 'fetching' || !canFetchModels.value) return
  const revision = ++modelsRevision
  modelQuery.value = ''
  modelsState.value = { phase: 'fetching' }
  try {
    const result = await listProviderModels(
      usesStoredKey.value && props.provider !== null
        ? { providerId: props.provider.id }
        : {
            protocol: draft.protocol,
            endpoint: draft.endpoint.trim(),
            secret: draft.secret.trim(),
            userAgent: draft.userAgent.trim(),
          },
    )
    if (revision !== modelsRevision) return
    // Fetching offers choices; only an explicit pick configures a model.
    modelsState.value = { phase: 'done', result }
  } catch {
    if (revision !== modelsRevision) return
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
  return code === undefined ? '' : t(`settings.providers.error.${code}`, { max: MAX_PROVIDER_IMAGES })
}

onMounted(() => nameInput.value?.focus())

/** Also the only place the typed key leaves component state. */
function closeForm(): void {
  Object.assign(draft, emptyDraft())
  emit('done')
}

async function submit(): Promise<void> {
  addManualModel()
  const id = editingId.value
  const failures = id === null ? await store.add(draft) : await store.update(id, draft)

  if (failures !== null) {
    errors.value = failures
    return
  }
  closeForm()
}

const protocolOptions = computed<DgSelectOption<ProviderProtocol>[]>(() =>
  PROVIDER_PROTOCOLS.filter((protocol) => protocol !== 'chatgpt').map((option) => ({ value: option, label: protocolLabel(option) })),
)

function onProtocolChange(next: ProviderProtocol): void {
  // Swap the suggested base URL only while the field still holds a suggestion,
  // so a hand-typed endpoint is never overwritten.
  const current = draft.endpoint.trim()
  if (current === '' || current === DEFAULT_ENDPOINTS[draft.protocol]) {
    draft.endpoint = DEFAULT_ENDPOINTS[next]
  }
  draft.protocol = next
}

const userAgentPresetOptions = computed<DgSelectOption<string>[]>(() => [
  { value: 'custom', label: t('settings.providers.form.userAgentPreset.custom') },
  ...USER_AGENT_PRESETS.map((preset) => ({
    value: preset.key,
    label: t(`settings.providers.form.userAgentPreset.${preset.key}`),
  })),
])

/** The preset whose value matches the draft exactly, else 'custom'. */
const selectedUserAgentPreset = computed(() => {
  const current = draft.userAgent.trim()
  const match = USER_AGENT_PRESETS.find((preset) => preset.value === current)
  return match?.key ?? 'custom'
})

/** 'custom' is a no-op: it never overwrites what the user typed. */
function applyUserAgentPreset(key: string): void {
  const preset = USER_AGENT_PRESETS.find((item) => item.key === key)
  if (preset !== undefined) draft.userAgent = preset.value
}
</script>

<template>
  <form class="form" @submit.prevent="submit">
    <h3 class="form__title">
      {{ editingId === null ? t('settings.providers.form.addTitle') : t('settings.providers.form.editTitle') }}
    </h3>

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
        <p v-if="errors.displayName" class="form__error">{{ errorText('displayName') }}</p>
      </label>

      <label class="form__cell">
        <span class="dg-field-label">{{ t('settings.providers.protocol.label') }}</span>
        <DgSelect
          :model-value="draft.protocol"
          :options="protocolOptions"
          :aria-label="t('settings.providers.protocol.label')"
          @update:model-value="onProtocolChange"
        />
      </label>

      <label class="form__cell form__cell--wide">
        <span class="dg-field-label">{{ t('settings.providers.form.endpoint') }}</span>
        <input
          v-model="draft.endpoint"
          class="dg-input dg-input--mono"
          type="url"
          inputmode="url"
          spellcheck="false"
          :placeholder="DEFAULT_ENDPOINTS[draft.protocol]"
          :aria-invalid="errors.endpoint ? 'true' : undefined"
        />
        <p v-if="errors.endpoint" class="form__error">{{ errorText('endpoint') }}</p>
        <p v-else-if="isPlainHttp" class="form__warning">{{ t('settings.providers.form.httpWarning') }}</p>
      </label>

      <!-- The key sits before the models: fetching the model list needs it. -->
      <label class="form__cell form__cell--wide">
        <span class="dg-field-label">{{ t('settings.providers.form.apiKey') }}</span>
        <input
          v-model="draft.secret"
          class="dg-input"
          type="password"
          autocomplete="off"
          spellcheck="false"
          :placeholder="t('settings.providers.form.apiKeyPlaceholder')"
        />
        <p v-if="editingId !== null" class="form__hint">{{ t('settings.providers.form.apiKeyKeepHint') }}</p>
      </label>

      <div class="form__cell form__cell--wide">
        <div class="form__label-row">
          <span class="dg-field-label">{{ t('settings.providers.form.models') }}</span>
          <span class="form__inline-actions">
            <button
              type="button"
              class="link-button"
              :disabled="!canFetchModels || modelsState.phase === 'fetching'"
              @click="fetchModels"
            >
              <DgIcon name="refresh" :size="12" />
              {{ t('settings.providers.models.fetch') }}
            </button>
          </span>
        </div>
        <ul v-if="draft.models.length" class="model-tags">
          <li v-for="model in draft.models" :key="model" class="model-tag">
            <span>{{ model }}</span>
            <button
              type="button"
              class="model-tag__remove"
              :aria-label="`${t('settings.providers.form.removeModel')} ${model}`"
              :title="t('settings.providers.form.removeModel')"
              @click="removeModel(model)"
            >
              <DgIcon name="close" :size="12" />
            </button>
          </li>
        </ul>
        <details v-if="modelOptions.length" class="model-picker" open>
          <summary class="dg-input model-picker__trigger">
            {{ t('settings.providers.models.choose') }}
          </summary>
          <div class="model-picker__panel">
            <input
              v-model="modelQuery"
              class="dg-input"
              type="search"
              :aria-label="t('settings.providers.models.search')"
              :placeholder="t('settings.providers.models.search')"
              @keydown.enter.prevent
            />
            <div class="model-picker__options">
              <label v-for="model in filteredModelOptions" :key="model" class="model-picker__option">
                <input
                  type="checkbox"
                  :value="model"
                  :checked="selectedModels.has(model)"
                  :disabled="!canPickModel && !selectedModels.has(model)"
                  @change="toggleModel(model, $event)"
                />
                <span>{{ model }}</span>
              </label>
              <p v-if="filteredModelOptions.length === 0" class="form__hint" role="status">
                {{ t('settings.providers.models.noMatches') }}
              </p>
            </div>
          </div>
        </details>
        <div v-if="!modelOptions.length || pendingModel" class="model-entry">
          <input
            v-model="manualModel"
            class="dg-input dg-input--mono model-entry__input"
            type="text"
            spellcheck="false"
            :disabled="!canAddModel"
            :placeholder="modelPlaceholder"
            :aria-label="t('settings.providers.form.models')"
            @keydown.enter.prevent="addManualModel"
          />
          <button
            type="button"
            class="link-button"
            :disabled="!canAddModel || pendingModel === ''"
            @click="addManualModel"
          >
            <DgIcon name="plus" :size="12" />
            {{ t('settings.providers.form.addModel') }}
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
        <p v-else-if="!modelOptions.length" class="form__hint">{{ t('settings.providers.form.modelsHint') }}</p>
      </div>

      <label class="form__cell">
        <span class="dg-field-label">{{ t('settings.providers.form.maxImages') }}</span>
        <input
          v-model.number="draft.maxImages"
          class="dg-input"
          type="number"
          min="0"
          :max="MAX_PROVIDER_IMAGES"
          step="1"
          :aria-invalid="errors.maxImages ? 'true' : undefined"
        />
        <p v-if="errors.maxImages" class="form__error">{{ errorText('maxImages') }}</p>
        <p v-else class="form__hint">{{ t('settings.providers.form.maxImagesHint') }}</p>
      </label>

      <div class="form__cell form__cell--wide">
        <span class="dg-field-label">{{ t('settings.providers.form.userAgent') }}</span>
        <div class="form__ua-row">
          <input
            v-model="draft.userAgent"
            class="dg-input dg-input--mono form__ua-input"
            type="text"
            spellcheck="false"
            autocomplete="off"
            :placeholder="t('settings.providers.form.userAgentPlaceholder')"
            :aria-invalid="errors.userAgent ? 'true' : undefined"
          />
          <DgSelect
            class="form__ua-preset"
            :model-value="selectedUserAgentPreset"
            :options="userAgentPresetOptions"
            :aria-label="t('settings.providers.form.userAgentPreset.label')"
            @update:model-value="applyUserAgentPreset"
          />
        </div>
        <p v-if="errors.userAgent" class="form__error">{{ errorText('userAgent') }}</p>
        <p v-else class="form__hint">{{ t('settings.providers.form.userAgentHint') }}</p>
      </div>
    </div>

    <footer class="form__footer">
      <p class="form__hint">{{ t('modelPlayground.saveFirst') }}</p>
      <span class="form__actions">
        <button type="button" class="dg-button" @click="closeForm">{{ t('common.action.cancel') }}</button>
        <button type="submit" class="dg-button dg-button--primary">{{ t('common.action.save') }}</button>
      </span>
    </footer>
  </form>
</template>

<style scoped>
.form {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 16px 18px;
  border: 1px solid var(--dg-card-border);
  border-radius: var(--dg-card-radius);
  background: var(--dg-card-fill);
  box-shadow: var(--dg-card-shadow);
}

.form__title {
  color: var(--dg-text-primary);
  font-size: 14px;
  font-weight: 600;
}

.form__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px 12px;
}

.form__cell {
  display: flex;
  flex-direction: column;
  gap: 5px;
  min-width: 0;
}

.form__cell--wide {
  grid-column: 1 / -1;
}

.dg-input--mono {
  font-family: var(--dg-font-mono);
  font-size: 12px;
}

.form__label-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.form__inline-actions {
  display: inline-flex;
  gap: 12px;
}

/* Quiet text actions inside the form, so only Save reads as a button. */
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

.link-button:disabled {
  color: var(--dg-text-muted);
  cursor: default;
}

.link-button:not(:disabled):hover {
  text-decoration: underline;
}

.link-button:focus-visible {
  border-radius: 4px;
  outline: none;
  box-shadow: 0 0 0 3px var(--dg-focus-ring);
}

.model-picker__trigger {
  cursor: pointer;
}

.model-picker__panel {
  display: grid;
  gap: 6px;
  margin-top: 5px;
  padding: 8px;
  border: 1px solid var(--dg-card-border);
  border-radius: 8px;
}

.model-picker__options {
  max-height: 180px;
  overflow: auto;
}

.model-picker__option {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px;
  font-family: var(--dg-font-mono);
  font-size: 12px;
  overflow-wrap: anywhere;
}

.model-picker__option:hover {
  background: var(--dg-hover-fill);
}

.model-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.model-tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  max-width: 100%;
  padding: 2px 4px 2px 8px;
  border: 1px solid var(--dg-chip-border);
  border-radius: 999px;
  background: var(--dg-chip-fill);
  color: var(--dg-text-primary);
  font-family: var(--dg-font-mono);
  font-size: 12px;
  overflow-wrap: anywhere;
}

.model-tag__remove {
  display: inline-grid;
  flex: none;
  width: 22px;
  height: 22px;
  padding: 0;
  border: none;
  border-radius: 50%;
  background: transparent;
  color: var(--dg-text-muted);
  cursor: pointer;
  place-items: center;
}

.model-tag__remove:hover {
  background: var(--dg-hover-fill);
  color: var(--dg-text-primary);
}

.model-tag__remove:focus-visible {
  outline: none;
  box-shadow: 0 0 0 3px var(--dg-focus-ring);
}

.model-entry {
  display: flex;
  align-items: center;
  gap: 8px;
}

.model-entry .link-button {
  flex: none;
}

.model-entry__input {
  flex: 1;
  min-width: 0;
}

.form__error,
.form__hint,
.form__warning {
  font-size: 11px;
  line-height: 1.45;
}

.form__ua-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.form__ua-input {
  flex: 1;
  min-width: 0;
}

.form__ua-preset {
  flex: none;
  width: 176px;
}

.form__error { color: var(--dg-danger); }
.form__warning { color: var(--dg-warning); }
.form__hint { color: var(--dg-text-muted); }

.form__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--dg-timeline-grid);
}

.form__actions {
  display: inline-flex;
  flex: none;
  gap: 8px;
}

@media (max-width: 620px) {
  .form__grid { grid-template-columns: minmax(0, 1fr); }
  .form__footer { flex-direction: column; align-items: stretch; }
  .form__actions { justify-content: flex-end; }
  .form__ua-row { flex-direction: column; align-items: stretch; }
  .form__ua-preset { width: auto; }
}
</style>
