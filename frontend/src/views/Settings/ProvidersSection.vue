<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import {
  PROVIDER_PROTOCOLS,
  type ProviderDTO,
  type ProviderModelsResult,
  type ProviderProtocol,
  type ProviderTestResult,
} from '@/api/dto'
import { listProviderModels, testProvider } from '@/api/providers'
import { testProviderConnection, WAILS_UNAVAILABLE } from '@/api/providerTest'
import {
  DEFAULT_ENDPOINTS,
  draftOf,
  emptyDraft,
  useProvidersStore,
  type ProviderDraft,
  type ProviderErrors,
  type ProviderField,
} from '@/stores/providers'

const { t, te } = useI18n()
const store = useProvidersStore()

void store.hydrate()

const formOpen = ref(false)
/** null while adding, a provider id while editing. */
const editingId = ref<string | null>(null)
const pendingRemoveId = ref<string | null>(null)
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

type ModelsState =
  | { phase: 'idle' }
  | { phase: 'fetching' }
  | { phase: 'done'; result: ProviderModelsResult }
  | { phase: 'picked'; model: string }

const modelsState = ref<ModelsState>({ phase: 'idle' })

// A result describes the draft as it was when tested; any later edit makes it
// stale, so it clears instead of lingering next to a different configuration.
watch(draft, () => {
  testState.value = { phase: 'idle' }
  if (modelsState.value.phase === 'done' || modelsState.value.phase === 'picked') {
    modelsState.value = { phase: 'idle' }
  }
})

/** The typed key, or — while editing — nothing: the stored key is in the
 * keychain and never comes back, so a saved provider with no new key cannot
 * run a draft probe; the card's test button covers that case. */
const canTest = computed(
  () =>
    draft.endpoint.trim() !== '' &&
    draft.model.trim() !== '' &&
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
      model: draft.model.trim(),
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

function pickModel(model: string): void {
  draft.model = model
  modelsState.value = { phase: 'picked', model }
}

/** Probe a saved provider with its keychain key. */
const testingSavedId = ref<string | null>(null)

async function runSavedTest(provider: ProviderDTO): Promise<void> {
  if (testingSavedId.value !== null) return
  testingSavedId.value = provider.id
  try {
    await testProvider(provider.id)
    // The result toast is future work; the binding's outcome is visible
    // through latency/capabilities in the diagnostics slice. Not blocking.
  } catch {
    // Binding errors surface through the section's error copy.
  } finally {
    testingSavedId.value = null
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
  formOpen.value = true
  void nextTick(() => nameInput.value?.focus())
}

function openEdit(provider: ProviderDTO): void {
  resetDraft(draftOf(provider))
  editingId.value = provider.id
  errors.value = {}
  formOpen.value = true
  void nextTick(() => nameInput.value?.focus())
}

/** Also the only place the typed key leaves component state. */
function closeForm(): void {
  resetDraft(emptyDraft())
  editingId.value = null
  errors.value = {}
  formOpen.value = false
  testState.value = { phase: 'idle' }
  modelsState.value = { phase: 'idle' }
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

// ---- Routing chain editor ----

/** Providers not yet in the chain; the "add fallback" dropdown options. */
const unchained = computed(() =>
  store.providers.filter((provider) => !store.routing.chain.includes(provider.id)),
)

/** The chain rows with their provider, resilient to a vanished id. */
const chainRows = computed(() =>
  store.routing.chain
    .map((id) => store.providers.find((provider) => provider.id === id))
    .filter((provider): provider is ProviderDTO => provider !== undefined),
)

async function setPrimary(id: string): Promise<void> {
  const rest = store.routing.chain.filter((entry) => entry !== id)
  await store.setChain([id, ...rest])
}

async function addFallback(id: string): Promise<void> {
  await store.setChain([...store.routing.chain, id])
}

async function removeEntry(id: string): Promise<void> {
  await store.setChain(store.routing.chain.filter((entry) => entry !== id))
}

/** Swap two adjacent chain positions. */
async function moveEntry(index: number, offset: -1 | 1): Promise<void> {
  const chain = [...store.routing.chain]
  const target = index + offset
  if (target < 0 || target >= chain.length) return
  ;[chain[index], chain[target]] = [chain[target], chain[index]]
  await store.setChain(chain)
}

function onPrimaryChange(event: Event): void {
  void setPrimary((event.target as HTMLSelectElement).value)
}

function onAddFallbackChange(event: Event): void {
  const target = event.target as HTMLSelectElement | null
  const value = target?.value ?? ''
  if (value !== '') void addFallback(value)
  // The select resets to its placeholder once the option list refreshes.
  if (target !== null) target.value = ''
}

async function confirmRemove(id: string): Promise<void> {
  await store.remove(id)
  pendingRemoveId.value = null
  if (editingId.value === id) closeForm()
}
</script>

<template>
  <p class="intro">{{ t('settings.providers.description') }}</p>

  <ul v-if="!store.isEmpty" class="list">
    <li v-for="provider in store.providers" :key="provider.id" class="item dg-card">
      <header class="item__head">
        <h2 class="item__name">{{ provider.displayName }}</h2>
        <span
          v-if="store.routing.chain[0] === provider.id"
          class="badge badge--accent"
        >
          {{ t('settings.providers.routing.primaryBadge') }}
        </span>
        <span
          v-else-if="store.routing.chain.includes(provider.id)"
          class="badge"
        >
          {{ t('settings.providers.routing.fallbackBadge') }}
        </span>
        <span class="badge">{{ protocolLabel(provider.protocol) }}</span>
      </header>

      <dl class="meta">
        <dt>{{ t('settings.providers.form.endpoint') }}</dt>
        <dd class="meta__mono">{{ provider.endpoint }}</dd>
        <dt>{{ t('settings.providers.form.model') }}</dt>
        <dd class="meta__mono">{{ provider.model }}</dd>
        <dt>{{ t('settings.providers.form.apiKey') }}</dt>
        <dd>
          {{
            provider.hasSecret
              ? t('settings.providers.secret.configured')
              : t('settings.providers.secret.missing')
          }}
        </dd>
      </dl>

      <div v-if="pendingRemoveId === provider.id" class="actions actions--confirm">
        <p class="actions__prompt">
          {{
            t('settings.providers.removeConfirm', { name: provider.displayName })
          }}
        </p>
        <button type="button" class="dg-button" @click="pendingRemoveId = null">
          {{ t('common.action.cancel') }}
        </button>
        <button
          type="button"
          class="dg-button dg-button--primary"
          @click="confirmRemove(provider.id)"
        >
          {{ t('common.action.delete') }}
        </button>
      </div>
      <div v-else class="actions">
        <button type="button" class="dg-button" @click="openEdit(provider)">
          {{ t('common.action.edit') }}
        </button>
        <button
          type="button"
          class="dg-button"
          :disabled="!provider.hasSecret || testingSavedId !== null"
          @click="runSavedTest(provider)"
        >
          {{ t('settings.providers.test.run') }}
        </button>
        <button
          v-if="provider.hasSecret"
          type="button"
          class="dg-button"
          @click="store.clearSecret(provider.id)"
        >
          {{ t('settings.providers.secret.clear') }}
        </button>
        <button
          type="button"
          class="dg-button"
          @click="pendingRemoveId = provider.id"
        >
          {{ t('common.action.delete') }}
        </button>
      </div>
    </li>
  </ul>

  <p v-else class="empty">{{ t('settings.providers.empty') }}</p>

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

      <label class="form__cell">
        <span class="dg-field-label">{{ t('settings.providers.form.model') }}</span>
        <div class="form__key-row">
          <input
            v-model="draft.model"
            class="dg-input"
            type="text"
            spellcheck="false"
            :placeholder="modelPlaceholder"
            :aria-invalid="errors.model ? 'true' : undefined"
          />
          <button
            type="button"
            class="dg-button"
            :disabled="!canFetchModels || modelsState.phase === 'fetching'"
            @click="fetchModels"
          >
            {{ t('settings.providers.models.fetch') }}
          </button>
        </div>
        <p v-if="errors.model" class="form__error">{{ errorText('model') }}</p>
        <p v-else-if="modelsState.phase === 'fetching'" class="form__hint">
          {{ t('settings.providers.models.fetching') }}
        </p>
        <p v-else-if="modelsState.phase === 'done' && !modelsState.result.ok" class="form__error">
          {{ modelsFailureText(modelsState.result) }}
        </p>
        <div
          v-else-if="modelsState.phase === 'done' && modelsState.result.models.length > 0"
          class="form__models"
        >
          <button
            v-for="model in modelsState.result.models"
            :key="model"
            type="button"
            class="badge badge--button"
            @click="pickModel(model)"
          >
            {{ model }}
          </button>
        </div>
        <p v-else-if="modelsState.phase === 'done'" class="form__hint">
          {{ t('settings.providers.models.empty') }}
        </p>
        <p v-else-if="modelsState.phase === 'picked'" class="form__test-ok">
          {{ t('settings.providers.models.picked', { model: modelsState.model }) }}
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
        <p v-if="editingId !== null" class="form__hint">
          {{ t('settings.providers.form.apiKeyKeepHint') }}
        </p>
        <p v-if="testState.phase === 'running'" class="form__hint">
          {{ t('settings.providers.test.running') }}
        </p>
        <p
          v-else-if="testState.phase === 'done' && testState.result.ok"
          class="form__test-ok"
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

  <section v-if="!store.isEmpty" class="routing dg-card">
    <h2 class="routing__title">{{ t('settings.providers.routing.title') }}</h2>
    <p class="routing__hint">{{ t('settings.providers.routing.description') }}</p>

    <ol v-if="chainRows.length > 0" class="routing__chain">
      <li v-for="(provider, index) in chainRows" :key="provider.id" class="routing__entry">
        <span class="routing__position">{{ index + 1 }}</span>
        <span class="routing__name">{{ provider.displayName }}</span>
        <span v-if="index === 0" class="badge badge--accent">
          {{ t('settings.providers.routing.primaryBadge') }}
        </span>
        <span class="routing__move">
          <button
            type="button"
            class="dg-button"
            :disabled="index === 0"
            :aria-label="t('settings.providers.routing.moveUp')"
            @click="moveEntry(index, -1)"
          >
            ↑
          </button>
          <button
            type="button"
            class="dg-button"
            :disabled="index === chainRows.length - 1"
            :aria-label="t('settings.providers.routing.moveDown')"
            @click="moveEntry(index, 1)"
          >
            ↓
          </button>
          <button
            type="button"
            class="dg-button"
            @click="removeEntry(provider.id)"
          >
            {{ t('common.action.delete') }}
          </button>
        </span>
      </li>
    </ol>

    <div class="routing__grid">
      <label class="form__cell">
        <span class="dg-field-label">
          {{ t('settings.providers.routing.primary') }}
        </span>
        <select
          class="dg-input"
          :value="store.routing.chain[0] ?? ''"
          @change="onPrimaryChange"
        >
          <option v-if="store.routing.chain.length === 0" value="">
            {{ t('settings.providers.routing.none') }}
          </option>
          <option v-for="provider in store.providers" :key="provider.id" :value="provider.id">
            {{ provider.displayName }}
          </option>
        </select>
      </label>

      <label v-if="unchained.length > 0" class="form__cell">
        <span class="dg-field-label">
          {{ t('settings.providers.routing.addFallback') }}
        </span>
        <select class="dg-input" value="" @change="onAddFallbackChange">
          <option value="">{{ t('settings.providers.routing.pickFallback') }}</option>
          <option v-for="provider in unchained" :key="provider.id" :value="provider.id">
            {{ provider.displayName }}
          </option>
        </select>
      </label>
    </div>
  </section>

  <section class="notice">
    <h2 class="notice__title">
      {{ t('settings.providers.secret.keychainTitle') }}
    </h2>
    <p class="notice__body">{{ t('settings.providers.secret.keychain') }}</p>
  </section>
</template>

<style scoped>
.intro,
.empty {
  color: var(--dg-text-secondary);
  font-size: 13px;
}

.list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.item,
.form,
.routing {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 17px 18px;
}

.item {
  transition:
    border-color var(--dg-motion-base) ease;
}

.item:hover {
  border-color: var(--dg-chip-border);
}

.item__head {
  display: flex;
  align-items: center;
  gap: 8px;
}

.item__name {
  color: var(--dg-text-primary);
  font-size: 14px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.badge {
  flex: none;
  padding: 3px 8px;
  border: 1px solid var(--dg-chip-border);
  border-radius: 999px;
  background: var(--dg-chip-fill);
  color: var(--dg-chip-text);
  font-size: 10px;
  font-weight: 620;
  letter-spacing: 0.02em;
}

.badge--accent {
  border-color: transparent;
  background: var(--dg-control-fill);
  color: var(--dg-accent-text);
}

.badge--button {
  cursor: pointer;
  font-size: 11px;
}

.badge--button:hover {
  border-color: var(--dg-accent-text);
}

.meta {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 2px 12px;
  margin: 0;
  font-size: 12px;
}

.meta dt {
  color: var(--dg-text-muted);
}

.meta dd {
  margin: 0;
  color: var(--dg-text-secondary);
  overflow-wrap: anywhere;
}

.meta__mono {
  font-family: var(--dg-font-mono);
}

.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.actions--confirm {
  align-items: center;
  padding: 8px 10px;
  border-radius: 6px;
  background: var(--dg-danger-fill);
}

.actions__prompt {
  flex: 1;
  min-width: 0;
  color: var(--dg-text-primary);
  font-size: 12px;
}

.form__title,
.routing__title {
  color: var(--dg-text-primary);
  font-size: 14px;
  font-weight: 600;
}

.form__grid,
.routing__grid {
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

.form__key-row .dg-input {
  flex: 1;
  min-width: 0;
}

.form__models {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 6px;
}

.form__test-ok {
  color: var(--dg-accent-text);
}

.form__hint,
.routing__hint {
  color: var(--dg-text-muted);
}

.routing__hint {
  font-size: 12px;
}

.routing__chain {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.routing__entry {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  border: 1px solid var(--dg-chip-border);
  border-radius: 6px;
}

.routing__position {
  flex: none;
  width: 18px;
  color: var(--dg-text-muted);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
  text-align: center;
}

.routing__name {
  flex: 1;
  min-width: 0;
  color: var(--dg-text-primary);
  font-size: 13px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.routing__move {
  display: flex;
  gap: 4px;
}

.routing__move .dg-button {
  padding: 3px 8px;
  font-size: 11px;
}

.form__actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.add {
  align-self: flex-start;
}

/*
 * A reassurance about where keys live, not a failure to act on: the quiet
 * track fill keeps it an aside. The danger tint stays reserved for errors and
 * the delete confirmation.
 */
.notice {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 12px 14px;
  border: 1px solid var(--dg-chip-border);
  border-radius: var(--dg-card-radius);
  background: var(--dg-track-fill);
}

.notice__title {
  color: var(--dg-text-primary);
  font-size: 13px;
  font-weight: 600;
}

.notice__body {
  color: var(--dg-text-secondary);
  font-size: 12px;
}

@media (max-width: 620px) {
  .form__grid,
  .routing__grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
