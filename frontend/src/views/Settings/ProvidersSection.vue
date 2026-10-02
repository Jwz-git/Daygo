<script setup lang="ts">
import DgIcon from '@/components/DgIcon.vue'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

import type { ProviderDTO, ProviderProtocol } from '@/api/dto'
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

function protocolLabel(protocol: ProviderProtocol): string {
  return t(`settings.providers.protocol.${protocol}`)
}

function isPrimary(provider: ProviderDTO): boolean {
  return store.routing.chain[0]?.providerId === provider.id
}

function isFallback(provider: ProviderDTO): boolean {
  return store.routing.chain.some((entry) => entry.providerId === provider.id) && !isPrimary(provider)
}

async function confirmRemove(id: string): Promise<void> {
  await store.remove(id)
  pendingRemoveId.value = null
  form.value?.closeIfEditing(id)
}
</script>

<template>
  <!-- Section intro: a shared h2 + hint, matching the other settings groups. -->
  <header class="providers-head">
    <h2 class="providers-head__title">{{ t('settings.providers.title') }}</h2>
    <p class="providers-head__hint">{{ t('settings.providers.description') }}</p>
    <RouterLink class="dg-button" :to="{ name: 'model-playground' }">{{ t('modelPlayground.title') }}</RouterLink>
  </header>

  <!-- Add/edit form (the add button lives here); kept above the list. -->
  <ProviderForm ref="form" />

  <!-- Provider Cards -->
  <section v-if="!store.isEmpty" class="providers-list">
    <TransitionGroup name="card" tag="div" class="providers-list__grid">
      <article
        v-for="provider in store.providers"
        :key="provider.id"
        class="provider-card dg-card"
        :class="{
          'provider-card--primary': isPrimary(provider),
          'provider-card--pending-remove': pendingRemoveId === provider.id,
        }"
      >
        <!-- Card Header -->
        <header class="provider-card__header">
          <h3 class="provider-card__name">{{ provider.displayName }}</h3>
          <div class="provider-card__badges">
            <span v-if="isPrimary(provider)" class="badge badge--primary">
              {{ t('settings.providers.routing.primaryBadge') }}
            </span>
            <span v-else-if="isFallback(provider)" class="badge badge--fallback">
              {{ t('settings.providers.routing.fallbackBadge') }}
            </span>
            <span class="badge badge--protocol">{{ protocolLabel(provider.protocol) }}</span>
          </div>
        </header>

        <!-- Card Meta -->
        <dl class="provider-card__meta">
          <div class="meta-row">
            <dt class="meta-label">
              <DgIcon name="link" :size="12" />
              {{ t('settings.providers.form.endpoint') }}
            </dt>
            <dd class="meta-value meta-value--mono">{{ provider.endpoint }}</dd>
          </div>
          <div class="meta-row meta-row--models">
            <dt class="meta-label">
              <DgIcon name="sparkle" :size="12" />
              {{ t('settings.providers.form.models') }}
            </dt>
            <dd class="meta-value">
              <ul class="model-list">
                <li v-for="model in provider.models" :key="model" class="model-list__item">
                  <span class="model-list__name">{{ model }}</span>
                  <RouterLink class="dg-button dg-button--tiny" :to="{ name: 'model-playground', query: { providerId: provider.id, model } }">{{ t('modelPlayground.title') }}</RouterLink>
                </li>
              </ul>
            </dd>
          </div>
          <div v-if="provider.maxImages > 0" class="meta-row">
            <dt class="meta-label">
              <DgIcon name="image" :size="12" />
              {{ t('settings.providers.form.maxImages') }}
            </dt>
            <dd class="meta-value">
              {{ t('settings.providers.form.maxImagesValue', { count: provider.maxImages }) }}
            </dd>
          </div>
          <div class="meta-row">
            <dt class="meta-label">
              <DgIcon name="lock" :size="12" />
              {{ t('settings.providers.form.apiKey') }}
            </dt>
            <dd class="meta-value meta-value--key">
              <span v-if="provider.hasSecret" class="key-status key-status--configured">
                <DgIcon name="check" :size="10" />
                {{ t('settings.providers.secret.configured') }}
              </span>
              <span v-else class="key-status key-status--missing">
                <DgIcon name="close" :size="10" />
                {{ t('settings.providers.secret.missing') }}
              </span>
            </dd>
          </div>
        </dl>

        <!-- Remove Confirmation -->
        <div v-if="pendingRemoveId === provider.id" class="provider-card__confirm">
          <p class="confirm-text">
            {{ t('settings.providers.removeConfirm', { name: provider.displayName }) }}
          </p>
          <div class="confirm-actions">
            <button type="button" class="dg-button dg-button--secondary" @click="pendingRemoveId = null">
              {{ t('common.action.cancel') }}
            </button>
            <button type="button" class="dg-button dg-button--danger" @click="confirmRemove(provider.id)">
              {{ t('common.action.delete') }}
            </button>
          </div>
        </div>

        <!-- Card Actions -->
        <footer v-else class="provider-card__actions">
          <button type="button" class="dg-button dg-button--secondary" @click="form?.openEdit(provider)">
            <DgIcon name="pencil" :size="14" />
            {{ t('common.action.edit') }}
          </button>
          <button
            v-if="provider.hasSecret"
            type="button"
            class="dg-button dg-button--ghost"
            @click="store.clearSecret(provider.id)"
          >
            <DgIcon name="signOut" :size="14" />
            {{ t('settings.providers.secret.clear') }}
          </button>
          <button
            type="button"
            class="dg-button dg-button--ghost dg-button--danger-ghost"
            @click="pendingRemoveId = provider.id"
          >
            <DgIcon name="trash" :size="14" />
            {{ t('common.action.delete') }}
          </button>
        </footer>
      </article>
    </TransitionGroup>
  </section>

  <!-- Empty State: its own copy, not the section description. -->
  <div v-else class="empty-state">
    <div class="empty-state__icon" aria-hidden="true">
      <DgIcon name="layers" :size="48" />
    </div>
    <h3 class="empty-state__title">{{ t('settings.providers.empty') }}</h3>
    <p class="empty-state__hint">{{ t('settings.providers.emptyHint') }}</p>
  </div>

  <!-- Routing Chain -->
  <ProviderRoutingChain v-if="!store.isEmpty" />

  <!-- Keychain Notice -->
  <section class="keychain-notice">
    <div class="keychain-notice__icon" aria-hidden="true">
      <DgIcon name="key" :size="16" />
    </div>
    <div class="keychain-notice__text">
      <h3 class="keychain-notice__title">{{ t('settings.providers.secret.keychainTitle') }}</h3>
      <p class="keychain-notice__body">{{ t('settings.providers.secret.keychain') }}</p>
    </div>
  </section>
</template>

<style scoped>
/* Section intro (shared group-head shape) */
.providers-head {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.providers-head__title {
  color: var(--dg-text-primary);
  font-size: 15px;
  font-weight: 650;
}

.providers-head__hint {
  color: var(--dg-text-secondary);
  font-size: 12px;
  max-width: 56ch;
}

/* Providers List */
.providers-list {
  display: flex;
  flex-direction: column;
}

.providers-list__grid {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

/* Provider Card */
.provider-card {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 18px 20px;
  transition:
    border-color var(--dg-motion-base) ease,
    box-shadow var(--dg-motion-base) ease;
}

.provider-card:hover {
  border-color: var(--dg-chip-border);
}

.provider-card--primary {
  border-color: var(--dg-accent);
  background: linear-gradient(
    135deg,
    color-mix(in srgb, var(--dg-accent) 6%, var(--dg-card-fill)) 0%,
    var(--dg-card-fill) 100%
  );
}

.provider-card--pending-remove {
  border-color: var(--dg-danger);
  background: var(--dg-danger-fill);
}

/* Card Header */
.provider-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.provider-card__name {
  margin: 0;
  color: var(--dg-text-primary);
  font-size: 15px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.provider-card__badges {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

/* Badges */
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

.badge--primary {
  border-color: transparent;
  background: var(--dg-accent);
  color: #ffffff;
}

.badge--fallback {
  border-color: var(--dg-accent);
  background: var(--dg-control-fill);
  color: var(--dg-accent-text);
}

.badge--protocol {
  text-transform: uppercase;
  font-size: 9px;
  letter-spacing: 0.04em;
}

/* Meta Info */
.provider-card__meta {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin: 0;
  padding: 12px 14px;
  border-radius: 8px;
  background: var(--dg-track-fill);
}

.meta-row {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 8px 12px;
  align-items: start;
}

.meta-label {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--dg-text-muted);
  font-size: 12px;
  white-space: nowrap;
}

.meta-label svg {
  flex: none;
  opacity: 0.7;
}

.meta-value {
  margin: 0;
  color: var(--dg-text-secondary);
  font-size: 12px;
  overflow-wrap: anywhere;
}

.meta-value--mono {
  font-family: var(--dg-font-mono);
  font-size: 11px;
  color: var(--dg-text-secondary);
}

.meta-row--models {
  align-items: start;
}

.model-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.model-list__item {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.model-list__name {
  color: var(--dg-accent-text);
  font-weight: 500;
  font-size: 12px;
  overflow-wrap: anywhere;
}

.dg-button--tiny {
  flex: none;
  padding: 2px 8px;
  font-size: 11px;
}

.model-list__result {
  font-size: 11px;
}

.model-list__result--ok {
  color: var(--dg-success);
}

.model-list__result--err {
  color: var(--dg-danger);
}

.meta-value--key {
  display: flex;
  align-items: center;
}

.key-status {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 500;
}

.key-status svg {
  flex: none;
}

.key-status--configured {
  background: color-mix(in srgb, var(--dg-success) 15%, transparent);
  color: var(--dg-success);
}

.key-status--missing {
  background: color-mix(in srgb, var(--dg-warning) 15%, transparent);
  color: var(--dg-warning);
}

/* Confirm Remove */
.provider-card__confirm {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px 14px;
  border-radius: 8px;
  background: var(--dg-danger-fill);
}

.confirm-text {
  margin: 0;
  color: var(--dg-danger-text);
  font-size: 13px;
}

.confirm-actions {
  display: flex;
  gap: 8px;
}

/* Card Actions */
.provider-card__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.provider-card__actions .dg-button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.provider-card__actions .dg-button svg {
  flex: none;
}

/* Button Variants */
.dg-button--danger {
  background: var(--dg-danger);
  border-color: var(--dg-danger);
  color: #ffffff;
}

.dg-button--danger:hover:not(:disabled) {
  background: color-mix(in srgb, var(--dg-danger) 85%, white);
}

.dg-button--ghost {
  background: transparent;
  border-color: transparent;
  color: var(--dg-text-secondary);
}

.dg-button--ghost:hover:not(:disabled) {
  background: var(--dg-hover-fill);
  color: var(--dg-text-primary);
}

.dg-button--danger-ghost {
  color: var(--dg-danger);
}

.dg-button--danger-ghost:hover:not(:disabled) {
  background: var(--dg-danger-fill);
}

/* Empty State */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 48px 24px;
  border: 1px dashed var(--dg-chip-border);
  border-radius: var(--dg-card-radius);
  background: var(--dg-track-fill);
  text-align: center;
}

.empty-state__icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 72px;
  height: 72px;
  border-radius: 16px;
  background: var(--dg-control-fill);
  color: var(--dg-text-muted);
  opacity: 0.6;
}

.empty-state__title {
  margin: 0;
  color: var(--dg-text-primary);
  font-size: 15px;
  font-weight: 600;
}

.empty-state__hint {
  margin: 0;
  color: var(--dg-text-muted);
  font-size: 13px;
}

/* Keychain Notice */
.keychain-notice {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 14px 16px;
  border: 1px solid var(--dg-chip-border);
  border-radius: var(--dg-card-radius);
  background: var(--dg-track-fill);
}

.keychain-notice__icon {
  flex: none;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: 8px;
  background: var(--dg-chip-fill);
  color: var(--dg-accent);
}

.keychain-notice__text {
  flex: 1;
  min-width: 0;
}

.keychain-notice__title {
  margin: 0;
  color: var(--dg-text-primary);
  font-size: 13px;
  font-weight: 600;
}

.keychain-notice__body {
  margin: 4px 0 0;
  color: var(--dg-text-secondary);
  font-size: 12px;
}

/* Card Transitions */
.card-enter-active,
.card-leave-active {
  transition:
    opacity var(--dg-motion-base) ease,
    transform var(--dg-motion-base) var(--dg-ease-glide);
}

.card-enter-from {
  opacity: 0;
  transform: translateY(-8px);
}

.card-leave-to {
  opacity: 0;
  transform: translateX(8px);
}

/* Responsive */
@media (max-width: 620px) {
  .provider-card__header {
    flex-direction: column;
    align-items: flex-start;
  }

  .provider-card__badges {
    margin-top: 4px;
  }

  .provider-card__actions {
    flex-direction: column;
  }

  .provider-card__actions .dg-button {
    width: 100%;
    justify-content: center;
  }
}

@media (prefers-reduced-motion: reduce) {
  .card-enter-active,
  .card-leave-active {
    transition: none;
  }
}
</style>
