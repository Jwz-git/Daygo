<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { ProviderDTO, ProviderRoutingEntry } from '@/api/dto'
import { useProvidersStore } from '@/stores/providers'

/* The routing chain editor: which (provider, model) pair answers first and who
   falls back. One ordered list — position 1 is the primary — with add, reorder
   and remove. Entirely store-driven; the section only places it. */
const { t } = useI18n()
const store = useProvidersStore()

/** Unit separator: cannot occur in a provider id or a model name, so it is a
    safe join for the add-dropdown option value and the row key. */
const SEP = ''

interface ChainRow {
  entry: ProviderRoutingEntry
  provider: ProviderDTO
  model: string
  key: string
}

/** An entry's effective model: its own, or the provider's first when empty. */
function resolveModel(provider: ProviderDTO, model: string): string {
  return model !== '' ? model : provider.models[0] ?? ''
}

/** Chain rows joined to their provider, resilient to a vanished provider id. */
const chainRows = computed<ChainRow[]>(() =>
  store.routing.chain.flatMap((entry) => {
    const provider = store.providers.find((candidate) => candidate.id === entry.providerId)
    if (provider === undefined) return []
    return [
      {
        entry,
        provider,
        model: resolveModel(provider, entry.model),
        key: entry.providerId + SEP + entry.model,
      },
    ]
  }),
)

/** The (provider, resolved-model) pairs already in the chain. */
const chainResolved = computed(
  () => new Set(chainRows.value.map((row) => row.provider.id + SEP + row.model)),
)

interface AddOption {
  label: string
  value: string
}

/** Every concrete (provider, model) pair not already represented in the chain. */
const addOptions = computed<AddOption[]>(() => {
  const out: AddOption[] = []
  for (const provider of store.providers) {
    for (const model of provider.models) {
      if (chainResolved.value.has(provider.id + SEP + model)) continue
      out.push({
        label: `${provider.displayName} · ${model}`,
        value: provider.id + SEP + model,
      })
    }
  }
  return out
})

async function addEntry(value: string): Promise<void> {
  const [providerId, model] = value.split(SEP)
  if (providerId === undefined || model === undefined) return
  await store.setChain([...store.routing.chain, { providerId, model }])
}

async function removeEntry(index: number): Promise<void> {
  await store.setChain(store.routing.chain.filter((_, position) => position !== index))
}

/** Swap two adjacent chain positions. */
async function moveEntry(index: number, offset: -1 | 1): Promise<void> {
  const chain = store.routing.chain.map((entry) => ({ ...entry }))
  const target = index + offset
  if (target < 0 || target >= chain.length) return
  ;[chain[index], chain[target]] = [chain[target], chain[index]]
  await store.setChain(chain)
}

function onAddChange(event: Event): void {
  const target = event.target as HTMLSelectElement | null
  const value = target?.value ?? ''
  if (value !== '') void addEntry(value)
  // The select snaps back to its placeholder once the option list refreshes.
  if (target !== null) target.value = ''
}
</script>

<template>
  <section class="routing dg-card">
    <h2 class="routing__title">{{ t('settings.providers.routing.title') }}</h2>
    <p class="routing__hint">{{ t('settings.providers.routing.description') }}</p>

    <ol v-if="chainRows.length > 0" class="routing__chain">
      <li v-for="(row, index) in chainRows" :key="row.key" class="routing__entry">
        <span class="routing__position">{{ index + 1 }}</span>
        <span class="routing__name">
          {{ row.provider.displayName }}
          <span class="routing__model">{{ row.model }}</span>
        </span>
        <span v-if="index === 0" class="badge badge--accent">
          {{ t('settings.providers.routing.primaryBadge') }}
        </span>
        <span class="routing__move">
          <button
            type="button"
            class="dg-button dg-button--icon"
            :disabled="index === 0"
            :aria-label="t('settings.providers.routing.moveUp')"
            @click="moveEntry(index, -1)"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
              <polyline points="18 15 12 9 6 15" />
            </svg>
          </button>
          <button
            type="button"
            class="dg-button dg-button--icon"
            :disabled="index === chainRows.length - 1"
            :aria-label="t('settings.providers.routing.moveDown')"
            @click="moveEntry(index, 1)"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
              <polyline points="6 9 12 15 18 9" />
            </svg>
          </button>
          <button
            type="button"
            class="dg-button dg-button--icon"
            :aria-label="t('settings.providers.routing.removeEntry')"
            @click="removeEntry(index)"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
              <line x1="5" y1="12" x2="19" y2="12" />
            </svg>
          </button>
        </span>
      </li>
    </ol>
    <p v-else class="routing__empty">{{ t('settings.providers.routing.none') }}</p>

    <label v-if="addOptions.length > 0" class="routing__add">
      <span class="dg-field-label">{{ t('settings.providers.routing.addEntry') }}</span>
      <select class="dg-input" value="" @change="onAddChange">
        <option value="">{{ t('settings.providers.routing.pickEntry') }}</option>
        <option v-for="option in addOptions" :key="option.value" :value="option.value">
          {{ option.label }}
        </option>
      </select>
    </label>
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

.routing__model {
  color: var(--dg-accent-text);
  font-weight: 500;
}

.routing__move {
  display: flex;
  gap: 4px;
}

.dg-button--icon {
  flex: none;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  padding: 4px 0;
}

.dg-button--icon svg {
  flex: none;
}

.routing__empty {
  color: var(--dg-text-muted);
  font-size: 12px;
}

.routing__add {
  display: flex;
  flex-direction: column;
  gap: 4px;
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
</style>
