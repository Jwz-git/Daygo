<script setup lang="ts">
import { computed, ref } from 'vue'
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
  } catch {
    // Binding errors surface through the section's error copy.
  } finally {
    testingSavedId.value = null
  }
}

function protocolLabel(protocol: ProviderProtocol): string {
  return t(`settings.providers.protocol.${protocol}`)
}

function protocolIcon(protocol: ProviderProtocol): string {
  switch (protocol) {
    case 'openai': return '◎'
    case 'openai_responses': return '◉'
    case 'anthropic': return '◈'
    default: return '○'
  }
}

function isPrimary(provider: ProviderDTO): boolean {
  return store.routing.chain[0] === provider.id
}

function isFallback(provider: ProviderDTO): boolean {
  return store.routing.chain.includes(provider.id) && !isPrimary(provider)
}

async function confirmRemove(id: string): Promise<void> {
  await store.remove(id)
  pendingRemoveId.value = null
  form.value?.closeIfEditing(id)
}
</script>

<template>
  <!-- Section Header -->
  <header class="section-header">
    <div class="section-header__icon">
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M12 2L2 7l10 5 10-5-10-5z"/>
        <path d="M2 17l10 5 10-5"/>
        <path d="M2 12l10 5 10-5"/>
      </svg>
    </div>
    <div class="section-header__text">
      <h1 class="section-header__title">{{ t('settings.nav.providers') }}</h1>
      <p class="section-header__desc">{{ t('settings.providers.description') }}</p>
    </div>
  </header>

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
          <div class="provider-card__identity">
            <span class="provider-card__protocol-icon" :title="protocolLabel(provider.protocol)">
              {{ protocolIcon(provider.protocol) }}
            </span>
            <h2 class="provider-card__name">{{ provider.displayName }}</h2>
          </div>
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
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/>
                <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/>
              </svg>
              {{ t('settings.providers.form.endpoint') }}
            </dt>
            <dd class="meta-value meta-value--mono">{{ provider.endpoint }}</dd>
          </div>
          <div class="meta-row">
            <dt class="meta-label">
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="12" r="3"/>
                <path d="M12 1v4M12 19v4M4.22 4.22l2.83 2.83M16.95 16.95l2.83 2.83M1 12h4M19 12h4M4.22 19.78l2.83-2.83M16.95 7.05l2.83-2.83"/>
              </svg>
              {{ t('settings.providers.form.model') }}
            </dt>
            <dd class="meta-value meta-value--model">{{ provider.model }}</dd>
          </div>
          <div v-if="provider.maxImages > 0" class="meta-row">
            <dt class="meta-label">
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <rect x="3" y="3" width="18" height="18" rx="2" ry="2"/>
                <circle cx="8.5" cy="8.5" r="1.5"/>
                <polyline points="21 15 16 10 5 21"/>
              </svg>
              {{ t('settings.providers.form.maxImages') }}
            </dt>
            <dd class="meta-value">
              {{ t('settings.providers.form.maxImagesValue', { count: provider.maxImages }) }}
            </dd>
          </div>
          <div class="meta-row">
            <dt class="meta-label">
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <rect x="3" y="11" width="18" height="11" rx="2" ry="2"/>
                <path d="M7 11V7a5 5 0 0 1 10 0v4"/>
              </svg>
              {{ t('settings.providers.form.apiKey') }}
            </dt>
            <dd class="meta-value meta-value--key">
              <span v-if="provider.hasSecret" class="key-status key-status--configured">
                <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3">
                  <polyline points="20 6 9 17 4 12"/>
                </svg>
                {{ t('settings.providers.secret.configured') }}
              </span>
              <span v-else class="key-status key-status--missing">
                <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3">
                  <line x1="18" y1="6" x2="6" y2="18"/>
                  <line x1="6" y1="6" x2="18" y2="18"/>
                </svg>
                {{ t('settings.providers.secret.missing') }}
              </span>
            </dd>
          </div>
        </dl>

        <!-- Testing Status -->
        <div v-if="testingSavedId === provider.id" class="provider-card__testing">
          <div class="testing-spinner" />
          <span>{{ t('settings.providers.test.running') }}</span>
        </div>

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
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
              <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
            </svg>
            {{ t('common.action.edit') }}
          </button>
          <button
            type="button"
            class="dg-button dg-button--secondary"
            :disabled="!provider.hasSecret || testingSavedId !== null"
            @click="runSavedTest(provider)"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/>
              <polyline points="22 4 12 14.01 9 11.01"/>
            </svg>
            {{ t('settings.providers.test.run') }}
          </button>
          <button
            v-if="provider.hasSecret"
            type="button"
            class="dg-button dg-button--ghost"
            @click="store.clearSecret(provider.id)"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/>
              <polyline points="16 17 21 12 16 7"/>
              <line x1="21" y1="12" x2="9" y2="12"/>
            </svg>
            {{ t('settings.providers.secret.clear') }}
          </button>
          <button
            type="button"
            class="dg-button dg-button--ghost dg-button--danger-ghost"
            @click="pendingRemoveId = provider.id"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="3 6 5 6 21 6"/>
              <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
            </svg>
            {{ t('common.action.delete') }}
          </button>
        </footer>
      </article>
    </TransitionGroup>
  </section>

  <!-- Empty State -->
  <div v-else class="empty-state">
    <div class="empty-state__icon">
      <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
        <path d="M12 2L2 7l10 5 10-5-10-5z"/>
        <path d="M2 17l10 5 10-5"/>
        <path d="M2 12l10 5 10-5"/>
      </svg>
    </div>
    <h2 class="empty-state__title">{{ t('settings.providers.empty') }}</h2>
    <p class="empty-state__hint">{{ t('settings.providers.description') }}</p>
  </div>

  <!-- Add Form -->
  <ProviderForm ref="form" />

  <!-- Routing Chain -->
  <ProviderRoutingChain v-if="!store.isEmpty" />

  <!-- Keychain Notice -->
  <section class="keychain-notice">
    <div class="keychain-notice__icon">
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.778 7.778 5.5 5.5 0 0 1 7.777-7.777zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4"/>
      </svg>
    </div>
    <div class="keychain-notice__text">
      <h3 class="keychain-notice__title">{{ t('settings.providers.secret.keychainTitle') }}</h3>
      <p class="keychain-notice__body">{{ t('settings.providers.secret.keychain') }}</p>
    </div>
  </section>
</template>

<style scoped>
/* Section Header */
.section-header {
  display: flex;
  align-items: flex-start;
  gap: 14px;
  padding: 20px 18px;
  border: 1px solid var(--dg-card-border);
  border-radius: var(--dg-card-radius);
  background: var(--dg-card-fill);
}

.section-header__icon {
  flex: none;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  border-radius: 10px;
  background: var(--dg-control-fill);
  color: var(--dg-accent);
}

.section-header__text {
  flex: 1;
  min-width: 0;
}

.section-header__title {
  margin: 0;
  color: var(--dg-text-primary);
  font-size: 16px;
  font-weight: 600;
}

.section-header__desc {
  margin: 4px 0 0;
  color: var(--dg-text-secondary);
  font-size: 13px;
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

.provider-card__identity {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.provider-card__protocol-icon {
  flex: none;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 6px;
  background: var(--dg-control-fill);
  color: var(--dg-accent);
  font-size: 14px;
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

.meta-value--model {
  color: var(--dg-accent-text);
  font-weight: 500;
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

/* Testing Status */
.provider-card__testing {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-radius: 6px;
  background: var(--dg-control-fill);
  color: var(--dg-accent-text);
  font-size: 12px;
}

.testing-spinner {
  width: 14px;
  height: 14px;
  border: 2px solid var(--dg-chip-border);
  border-top-color: var(--dg-accent);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
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
  .testing-spinner {
    animation: none;
  }

  .card-enter-active,
  .card-leave-active {
    transition: none;
  }
}
</style>
