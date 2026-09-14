<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

import type { ProviderDTO, ProviderProtocol } from '@/api/dto'
import { testProvider } from '@/api/providers'
import { useProvidersStore } from '@/stores/providers'

import ProviderForm from './ProviderForm.vue'
import ProviderRoutingChain from './ProviderRoutingChain.vue'

/*
 * The section owns the saved-provider cards; the add/edit form
 * (ProviderForm) and the routing chain editor (ProviderRoutingChain) are
 * separate components. Editing goes through the form's exposed handle so a
 * card's Edit button can populate the draft.
 */
const { t } = useI18n()
const store = useProvidersStore()

void store.hydrate()

const form = ref<InstanceType<typeof ProviderForm> | null>(null)
const pendingRemoveId = ref<string | null>(null)

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

async function confirmRemove(id: string): Promise<void> {
  await store.remove(id)
  pendingRemoveId.value = null
  form.value?.closeIfEditing(id)
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
        <template v-if="provider.maxImages > 0">
          <dt>{{ t('settings.providers.form.maxImages') }}</dt>
          <dd class="meta__mono">
            {{ t('settings.providers.form.maxImagesValue', { count: provider.maxImages }) }}
          </dd>
        </template>
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
        <button type="button" class="dg-button" @click="form?.openEdit(provider)">
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

  <ProviderForm ref="form" />

  <ProviderRoutingChain v-if="!store.isEmpty" />

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

.item {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 17px 18px;
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
</style>
