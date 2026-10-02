<script setup lang="ts">
import DgIcon from '@/components/DgIcon.vue'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

import type { ProviderDTO, ProviderProtocol } from '@/api/dto'
import { useProvidersStore } from '@/stores/providers'

import ProviderForm from './ProviderForm.vue'
import ProviderRoutingChain from './ProviderRoutingChain.vue'

/*
 * AI services, laid out like the other settings sections: a title row with
 * the page actions, then flat rows with hairline separators. One service is
 * one row; adding opens the form above the list, editing opens it in place of
 * the row. Models are chips that open the model test page for that pair.
 */
const { t } = useI18n()
const store = useProvidersStore()

void store.hydrate()

/** 'new' while adding, a provider id while editing, null when closed. */
const editing = ref<'new' | string | null>(null)
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

function startEdit(provider: ProviderDTO): void {
  pendingRemoveId.value = null
  editing.value = provider.id
}

async function confirmRemove(id: string): Promise<void> {
  await store.remove(id)
  pendingRemoveId.value = null
  if (editing.value === id) editing.value = null
}
</script>

<template>
  <header class="head">
    <div class="head__text">
      <h2 class="head__title">{{ t('settings.providers.title') }}</h2>
      <p class="head__hint">{{ t('settings.providers.description') }}</p>
    </div>
    <div class="head__actions">
      <RouterLink class="dg-button dg-button--small" :to="{ name: 'model-playground' }">
        {{ t('modelPlayground.title') }}
      </RouterLink>
      <button
        type="button"
        class="dg-button dg-button--primary dg-button--small"
        :disabled="editing === 'new'"
        @click="editing = 'new'"
      >
        <DgIcon name="plus" :size="13" />
        {{ t('settings.providers.add') }}
      </button>
    </div>
  </header>

  <Transition name="fold">
    <ProviderForm v-if="editing === 'new'" :provider="null" @done="editing = null" />
  </Transition>

  <ul v-if="!store.isEmpty" class="services">
    <li v-for="provider in store.providers" :key="provider.id" class="service">
      <ProviderForm v-if="editing === provider.id" :provider="provider" @done="editing = null" />

      <template v-else>
        <div class="service__main">
          <div class="service__title-row">
            <h3 class="service__name">{{ provider.displayName }}</h3>
            <span v-if="isPrimary(provider)" class="badge badge--primary">
              {{ t('settings.providers.routing.primaryBadge') }}
            </span>
            <span v-else-if="isFallback(provider)" class="badge badge--fallback">
              {{ t('settings.providers.routing.fallbackBadge') }}
            </span>
            <span class="service__protocol">{{ protocolLabel(provider.protocol) }}</span>
          </div>

          <p class="service__meta">
            <span class="service__endpoint">{{ provider.endpoint }}</span>
            <span
              class="service__key"
              :class="provider.hasSecret ? 'is-configured' : 'is-missing'"
            >
              <DgIcon :name="provider.hasSecret ? 'lock' : 'close'" :size="11" />
              {{ provider.hasSecret ? t('settings.providers.secret.configured') : t('settings.providers.secret.missing') }}
            </span>
            <span v-if="provider.maxImages > 0" class="service__images">
              {{ t('settings.providers.form.maxImages') }} {{ t('settings.providers.form.maxImagesValue', { count: provider.maxImages }) }}
            </span>
          </p>

          <!-- Each model chip opens the model test page for this pair. -->
          <ul class="service__models" :aria-label="t('settings.providers.form.models')">
            <li v-for="model in provider.models" :key="model">
              <RouterLink
                class="model-chip"
                :to="{ name: 'model-playground', query: { providerId: provider.id, model } }"
                :title="t('modelPlayground.title')"
              >
                {{ model }}
                <DgIcon name="arrowRight" :size="10" />
              </RouterLink>
            </li>
          </ul>

          <div v-if="pendingRemoveId === provider.id" class="service__confirm" role="alert">
            <span>{{ t('settings.providers.removeConfirm', { name: provider.displayName }) }}</span>
            <button type="button" class="dg-button dg-button--small" @click="pendingRemoveId = null">
              {{ t('common.action.cancel') }}
            </button>
            <button type="button" class="dg-button dg-button--small dg-button--danger" @click="confirmRemove(provider.id)">
              {{ t('common.action.delete') }}
            </button>
          </div>
        </div>

        <div v-if="pendingRemoveId !== provider.id" class="service__actions">
          <button type="button" class="dg-button dg-button--small" @click="startEdit(provider)">
            {{ t('common.action.edit') }}
          </button>
          <button
            v-if="provider.hasSecret"
            type="button"
            class="icon-button"
            :title="t('settings.providers.secret.clear')"
            :aria-label="t('settings.providers.secret.clear')"
            @click="store.clearSecret(provider.id)"
          >
            <DgIcon name="key" :size="14" />
          </button>
          <button
            type="button"
            class="icon-button icon-button--danger"
            :title="t('common.action.delete')"
            :aria-label="t('common.action.delete')"
            @click="pendingRemoveId = provider.id"
          >
            <DgIcon name="trash" :size="14" />
          </button>
        </div>
      </template>
    </li>
  </ul>

  <div v-else-if="editing !== 'new'" class="empty">
    <p class="empty__title">{{ t('settings.providers.empty') }}</p>
    <p class="empty__hint">{{ t('settings.providers.emptyHint') }}</p>
  </div>

  <ProviderRoutingChain v-if="!store.isEmpty" />

  <p class="keychain">
    <DgIcon name="lock" :size="12" />
    <span><b>{{ t('settings.providers.secret.keychainTitle') }}</b> · {{ t('settings.providers.secret.keychain') }}</span>
  </p>
</template>

<style scoped>
.head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.head__text {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.head__title {
  color: var(--dg-text-primary);
  font-size: 15px;
  font-weight: 650;
}

.head__hint {
  max-width: 56ch;
  color: var(--dg-text-secondary);
  font-size: 12px;
}

.head__actions {
  display: flex;
  flex: none;
  gap: 8px;
}

.dg-button--small {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  min-height: 28px;
  padding: 4px 11px;
  font-size: 12px;
  text-decoration: none;
}

.dg-button--danger {
  background: var(--dg-danger);
  box-shadow: none;
  color: #ffffff;
}

.dg-button--danger:not(:disabled):hover {
  background: color-mix(in srgb, var(--dg-danger) 86%, #ffffff);
}

/* Services: flat rows with hairlines, like SettingRow. */
.services {
  display: flex;
  flex-direction: column;
  margin: 0;
  padding: 0;
  list-style: none;
}

.service {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  padding: 15px 2px;
  border-bottom: 1px solid var(--dg-timeline-grid);
}

.service:last-child {
  border-bottom: none;
}

.service > .form {
  flex: 1;
}

.service__main {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}

.service__title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.service__name {
  min-width: 0;
  overflow: hidden;
  color: var(--dg-text-primary);
  font-size: 14px;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.service__protocol {
  flex: none;
  color: var(--dg-text-muted);
  font-size: 11px;
}

.badge {
  flex: none;
  padding: 1px 7px;
  border-radius: 999px;
  font-size: 10px;
  font-weight: 650;
}

.badge--primary {
  background: var(--dg-accent);
  color: #ffffff;
}

.badge--fallback {
  box-shadow: inset 0 0 0 1px var(--dg-accent);
  color: var(--dg-accent-text);
}

.service__meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px 12px;
  color: var(--dg-text-secondary);
  font-size: 12px;
}

.service__endpoint {
  min-width: 0;
  overflow-wrap: anywhere;
  font-family: var(--dg-font-mono);
  font-size: 11px;
}

.service__key {
  display: inline-flex;
  flex: none;
  align-items: center;
  gap: 4px;
}

.service__key.is-configured { color: var(--dg-success); }
.service__key.is-missing { color: var(--dg-warning); }

.service__images {
  flex: none;
  color: var(--dg-text-muted);
}

.service__models {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin: 2px 0 0;
  padding: 0;
  list-style: none;
}

.model-chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  border-radius: 6px;
  background: var(--dg-track-fill);
  color: var(--dg-text-secondary);
  font-size: 12px;
  text-decoration: none;
  transition: background-color var(--dg-motion-fast) ease, color var(--dg-motion-fast) ease;
}

.model-chip :deep(svg) {
  opacity: 0;
  transition: opacity var(--dg-motion-fast) ease;
}

.model-chip:hover {
  background: var(--dg-hover-fill);
  color: var(--dg-accent-text);
}

.model-chip:hover :deep(svg),
.model-chip:focus-visible :deep(svg) {
  opacity: 1;
}

.model-chip:focus-visible {
  outline: none;
  box-shadow: 0 0 0 3px var(--dg-focus-ring);
}

.service__actions {
  display: flex;
  flex: none;
  align-items: center;
  gap: 2px;
}

.service__actions .dg-button {
  margin-right: 4px;
}

.icon-button {
  display: inline-grid;
  width: 28px;
  height: 28px;
  padding: 0;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--dg-text-muted);
  cursor: pointer;
  place-items: center;
  transition: background-color var(--dg-motion-fast) ease, color var(--dg-motion-fast) ease;
}

.icon-button:hover {
  background: var(--dg-hover-fill);
  color: var(--dg-text-primary);
}

.icon-button--danger:hover {
  background: var(--dg-danger-fill);
  color: var(--dg-danger);
}

.icon-button:focus-visible {
  outline: none;
  box-shadow: 0 0 0 3px var(--dg-focus-ring);
}

.service__confirm {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-top: 4px;
  padding: 8px 10px;
  border-radius: 8px;
  background: var(--dg-danger-fill);
  color: var(--dg-danger);
  font-size: 12px;
}

.service__confirm span {
  flex: 1;
  min-width: 12em;
}

.empty {
  padding: 28px 16px;
  border: 1px dashed var(--dg-timeline-grid);
  border-radius: var(--dg-card-radius);
  text-align: center;
}

.empty__title {
  color: var(--dg-text-primary);
  font-size: 14px;
  font-weight: 600;
}

.empty__hint {
  margin-top: 4px;
  color: var(--dg-text-muted);
  font-size: 12px;
}

.keychain {
  display: flex;
  align-items: baseline;
  gap: 6px;
  color: var(--dg-text-muted);
  font-size: 11px;
  line-height: 1.5;
}

.keychain :deep(svg) {
  flex: none;
  transform: translateY(1px);
}

.keychain b {
  color: var(--dg-text-secondary);
  font-weight: 600;
}

.fold-enter-active,
.fold-leave-active {
  transition: opacity var(--dg-motion-base) ease, transform var(--dg-motion-base) var(--dg-ease-glide);
}

.fold-enter-from,
.fold-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}

@media (max-width: 620px) {
  .head { flex-direction: column; }
  .service { flex-direction: column; }
  .service__actions { align-self: flex-end; }
}

@media (prefers-reduced-motion: reduce) {
  .fold-enter-active,
  .fold-leave-active { transition: none; }
}
</style>
