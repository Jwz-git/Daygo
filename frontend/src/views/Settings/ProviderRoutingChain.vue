<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { ProviderDTO } from '@/api/dto'
import { useProvidersStore } from '@/stores/providers'

/* The routing chain editor: which provider answers first and who falls back.
   Entirely store-driven; the section only places it. */
const { t } = useI18n()
const store = useProvidersStore()

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
</script>

<template>
  <section class="routing dg-card">
    <h2 class="routing__title">{{ t('settings.providers.routing.title') }}</h2>
    <p class="routing__hint">{{ t('settings.providers.routing.description') }}</p>

    <ol v-if="chainRows.length > 0" class="routing__chain">
      <li v-for="(provider, index) in chainRows" :key="provider.id" class="routing__entry">
        <span class="routing__position">{{ index + 1 }}</span>
        <span class="routing__name">{{ provider.displayName }} · {{ provider.model }}</span>
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
      <label class="routing__cell">
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
            {{ provider.displayName }} · {{ provider.model }}
          </option>
        </select>
      </label>

      <label v-if="unchained.length > 0" class="routing__cell">
        <span class="dg-field-label">
          {{ t('settings.providers.routing.addFallback') }}
        </span>
        <select class="dg-input" value="" @change="onAddFallbackChange">
          <option value="">{{ t('settings.providers.routing.pickFallback') }}</option>
          <option v-for="provider in unchained" :key="provider.id" :value="provider.id">
            {{ provider.displayName }} · {{ provider.model }}
          </option>
        </select>
      </label>
    </div>
  </section>
</template>

<style scoped>
.routing {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 17px 18px;
}

.routing__title {
  color: var(--dg-text-primary);
  font-size: 14px;
  font-weight: 600;
}

.routing__hint {
  color: var(--dg-text-muted);
  font-size: 12px;
}

.routing__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
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

@media (max-width: 620px) {
  .routing__grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
