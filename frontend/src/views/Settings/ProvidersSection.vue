<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import MilestoneNotice from '@/components/MilestoneNotice.vue'
import {
  PROVIDER_PROTOCOLS,
  type ProviderDTO,
  type ProviderProtocol,
} from '@/api/dto'
import {
  DEFAULT_ENDPOINTS,
  draftOf,
  emptyDraft,
  useProvidersStore,
  type ProviderDraft,
  type ProviderErrors,
  type ProviderField,
} from '@/stores/providers'

const { t } = useI18n()
const store = useProvidersStore()

void store.hydrate()

const formOpen = ref(false)
/** null while adding, a provider id while editing. */
const editingId = ref<string | null>(null)
const pendingRemoveId = ref<string | null>(null)
const errors = ref<ProviderErrors>({})
const draft = reactive<ProviderDraft>(emptyDraft())

const secondaryChoices = computed(() =>
  store.providers.filter((provider) => provider.id !== store.routing.primary),
)

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
}

function openEdit(provider: ProviderDTO): void {
  resetDraft(draftOf(provider))
  editingId.value = provider.id
  errors.value = {}
  formOpen.value = true
}

/** Also the only place the typed key leaves component state. */
function closeForm(): void {
  resetDraft(emptyDraft())
  editingId.value = null
  errors.value = {}
  formOpen.value = false
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

function onPrimaryChange(event: Event): void {
  void store.setPrimary((event.target as HTMLSelectElement).value)
}

function onSecondaryChange(event: Event): void {
  const value = (event.target as HTMLSelectElement).value
  void store.setSecondary(value === '' ? null : value)
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
          v-if="store.routing.primary === provider.id"
          class="badge badge--accent"
        >
          {{ t('settings.providers.routing.primaryBadge') }}
        </span>
        <span
          v-else-if="store.routing.secondary === provider.id"
          class="badge badge--accent"
        >
          {{ t('settings.providers.routing.secondaryBadge') }}
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
      </label>

      <label class="form__cell">
        <span class="dg-field-label">{{ t('settings.providers.form.model') }}</span>
        <input
          v-model="draft.model"
          class="dg-input"
          type="text"
          spellcheck="false"
          :placeholder="t('settings.providers.form.modelPlaceholder')"
          :aria-invalid="errors.model ? 'true' : undefined"
        />
        <p v-if="errors.model" class="form__error">{{ errorText('model') }}</p>
      </label>

      <label class="form__cell">
        <span class="dg-field-label">{{ t('settings.providers.form.apiKey') }}</span>
        <input
          v-model="draft.secret"
          class="dg-input"
          type="password"
          autocomplete="off"
          spellcheck="false"
          :placeholder="t('settings.providers.form.apiKeyPlaceholder')"
        />
        <p v-if="editingId !== null" class="form__hint">
          {{ t('settings.providers.form.apiKeyKeepHint') }}
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
    <div class="routing__grid">
      <label class="form__cell">
        <span class="dg-field-label">
          {{ t('settings.providers.routing.primary') }}
        </span>
        <select
          class="dg-input"
          :value="store.routing.primary"
          @change="onPrimaryChange"
        >
          <option v-for="provider in store.providers" :key="provider.id" :value="provider.id">
            {{ provider.displayName }}
          </option>
        </select>
      </label>

      <label class="form__cell">
        <span class="dg-field-label">
          {{ t('settings.providers.routing.secondary') }}
        </span>
        <select
          class="dg-input"
          :value="store.routing.secondary ?? ''"
          @change="onSecondaryChange"
        >
          <option value="">{{ t('settings.providers.routing.none') }}</option>
          <option v-for="provider in secondaryChoices" :key="provider.id" :value="provider.id">
            {{ provider.displayName }}
          </option>
        </select>
      </label>
    </div>
  </section>

  <section class="notice">
    <h2 class="notice__title">
      {{ t('settings.providers.secret.sessionOnlyTitle') }}
    </h2>
    <p class="notice__body">{{ t('settings.providers.secret.sessionOnly') }}</p>
  </section>

  <MilestoneNotice
    title-key="settings.providers.test"
    description-key="settings.providers.testDescription"
    :milestone="2"
  />
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
    border-color var(--dg-motion-base) ease,
    box-shadow var(--dg-motion-base) ease,
    transform var(--dg-motion-base) var(--dg-ease-out);
}

.item:hover {
  border-color: var(--dg-chip-border);
  box-shadow:
    0 16px 38px rgba(25, 18, 30, 0.08),
    inset 0 1px 0 rgba(255, 255, 255, 0.06);
  transform: translateY(-1px);
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
.form__hint {
  margin-top: 4px;
  font-size: 11px;
}

.form__error {
  color: var(--dg-danger);
}

.form__hint,
.routing__hint {
  color: var(--dg-text-muted);
}

.routing__hint {
  font-size: 12px;
}

.form__actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.add {
  align-self: flex-start;
}

.notice {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 12px 14px;
  border-radius: var(--dg-card-radius);
  background: var(--dg-danger-fill);
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
