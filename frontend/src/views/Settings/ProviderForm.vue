<script setup lang="ts">
import DgIcon from '@/components/DgIcon.vue'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import ComboBox from '@/components/ComboBox.vue'

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
  MAX_PROVIDER_MODELS,
  useProvidersStore,
  type ProviderDraft,
  type ProviderErrors,
  type ProviderField,
} from '@/stores/providers'

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

// Clear model listings when their source configuration changes.
watch(draft, () => {
  if (modelsState.value.phase === 'done') {
    modelsState.value = { phase: 'idle' }
  }
})

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
  modelsState.value = { phase: 'fetching' }
  try {
    const result = await listProviderModels(
      usesStoredKey.value && props.provider !== null
        ? { providerId: props.provider.id }
        : { protocol: draft.protocol, endpoint: draft.endpoint.trim(), secret: draft.secret.trim() },
    )
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

onMounted(() => nameInput.value?.focus())

/** Also the only place the typed key leaves component state. */
function closeForm(): void {
  Object.assign(draft, emptyDraft())
  emit('done')
}

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
        <select class="dg-input" :value="draft.protocol" @change="onProtocolChange">
          <option v-for="option in PROVIDER_PROTOCOLS" :key="option" :value="option">
            {{ protocolLabel(option) }}
          </option>
        </select>
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
            <button type="button" class="link-button" :disabled="!canAddModel" @click="addModelRow">
              <DgIcon name="plus" :size="12" />
              {{ t('settings.providers.form.addModel') }}
            </button>
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
              class="icon-button"
              :disabled="draft.models.length === 1 && model.trim() === ''"
              :aria-label="t('settings.providers.form.removeModel')"
              :title="t('settings.providers.form.removeModel')"
              @click="removeModelRow(index)"
            >
              <DgIcon name="minus" :size="14" />
            </button>
          </li>
        </ul>
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
        <span class="dg-field-label">{{ t('settings.providers.form.maxImages') }}</span>
        <input
          v-model.number="draft.maxImages"
          class="dg-input"
          type="number"
          min="0"
          max="20"
          step="1"
          :aria-invalid="errors.maxImages ? 'true' : undefined"
        />
        <p v-if="errors.maxImages" class="form__error">{{ errorText('maxImages') }}</p>
        <p v-else class="form__hint">{{ t('settings.providers.form.maxImagesHint') }}</p>
      </label>
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
  gap: 6px;
}

.model-list__combo {
  flex: 1;
  min-width: 0;
}

.icon-button {
  display: inline-grid;
  flex: none;
  width: 28px;
  height: 28px;
  padding: 0;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--dg-text-muted);
  cursor: pointer;
  place-items: center;
}

.icon-button:not(:disabled):hover {
  background: var(--dg-hover-fill);
  color: var(--dg-text-primary);
}

.icon-button:disabled {
  opacity: 0.4;
  cursor: default;
}

.icon-button:focus-visible {
  outline: none;
  box-shadow: 0 0 0 3px var(--dg-focus-ring);
}

.form__error,
.form__hint,
.form__warning {
  font-size: 11px;
  line-height: 1.45;
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
}
</style>
