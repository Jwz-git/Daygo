<script setup lang="ts">
import DgIcon from '@/components/DgIcon.vue'
import DgSelect from '@/components/DgSelect.vue'
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

// The picker is an action, not a value: it always shows its placeholder, and a
// pick adds the entry (which then drops out of the options).
function onAddChange(value: string): void {
  if (value !== '') void addEntry(value)
}
</script>

<template>
  <section class="routing">
    <div class="routing__head">
      <h3 class="routing__title">{{ t('settings.providers.routing.title') }}</h3>
      <p class="routing__hint">{{ t('settings.providers.routing.description') }}</p>
    </div>

    <TransitionGroup v-if="chainRows.length > 0" name="reorder" tag="ol" class="routing__chain">
      <li v-for="(row, index) in chainRows" :key="row.key" class="routing__entry">
        <span class="routing__position" :class="{ 'is-primary': index === 0 }">{{ index + 1 }}</span>
        <span class="routing__name">
          {{ row.provider.displayName }}
          <span class="routing__model">{{ row.model }}</span>
        </span>
        <span v-if="index === 0" class="badge">{{ t('settings.providers.routing.primaryBadge') }}</span>
        <span class="routing__move">
          <button
            type="button"
            class="icon-button"
            :disabled="index === 0"
            :aria-label="t('settings.providers.routing.moveUp')"
            :title="t('settings.providers.routing.moveUp')"
            @click="moveEntry(index, -1)"
          >
            <DgIcon name="chevronUp" :size="14" />
          </button>
          <button
            type="button"
            class="icon-button"
            :disabled="index === chainRows.length - 1"
            :aria-label="t('settings.providers.routing.moveDown')"
            :title="t('settings.providers.routing.moveDown')"
            @click="moveEntry(index, 1)"
          >
            <DgIcon name="chevronDown" :size="14" />
          </button>
          <button
            type="button"
            class="icon-button icon-button--danger"
            :aria-label="t('settings.providers.routing.removeEntry')"
            :title="t('settings.providers.routing.removeEntry')"
            @click="removeEntry(index)"
          >
            <DgIcon name="minus" :size="14" />
          </button>
        </span>
      </li>
    </TransitionGroup>
    <p v-else class="routing__empty">{{ t('settings.providers.routing.none') }}</p>

    <label v-if="addOptions.length > 0" class="routing__add">
      <span class="routing__add-label">{{ t('settings.providers.routing.addEntry') }}</span>
      <DgSelect
        model-value=""
        :options="addOptions"
        :placeholder="t('settings.providers.routing.pickEntry')"
        :aria-label="t('settings.providers.routing.addEntry')"
        @update:model-value="onAddChange"
      />
    </label>
  </section>
</template>

<style scoped>
.routing {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 8px;
}

.routing__head {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.routing__title {
  color: var(--dg-text-primary);
  font-size: 15px;
  font-weight: 650;
}

.routing__hint {
  max-width: 56ch;
  color: var(--dg-text-secondary);
  font-size: 12px;
}

.routing__chain {
  display: flex;
  flex-direction: column;
  margin: 0;
  padding: 0;
  list-style: none;
}

.routing__entry {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 2px;
  border-bottom: 1px solid var(--dg-timeline-grid);
}

.routing__position {
  display: inline-grid;
  flex: none;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: var(--dg-track-fill);
  color: var(--dg-text-secondary);
  font-size: 11px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  place-items: center;
}

.routing__position.is-primary {
  background: var(--dg-accent);
  color: #ffffff;
}

.routing__name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  color: var(--dg-text-primary);
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.routing__model {
  margin-left: 4px;
  color: var(--dg-text-secondary);
}

.badge {
  flex: none;
  padding: 1px 7px;
  border-radius: 999px;
  box-shadow: inset 0 0 0 1px var(--dg-accent);
  color: var(--dg-accent-text);
  font-size: 10px;
  font-weight: 650;
}

.routing__move {
  display: flex;
  flex: none;
  gap: 2px;
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

.icon-button:not(:disabled):hover {
  background: var(--dg-hover-fill);
  color: var(--dg-text-primary);
}

.icon-button--danger:not(:disabled):hover {
  background: var(--dg-danger-fill);
  color: var(--dg-danger);
}

.icon-button:disabled {
  opacity: 0.35;
  cursor: default;
}

.icon-button:focus-visible {
  outline: none;
  box-shadow: 0 0 0 3px var(--dg-focus-ring);
}

.routing__empty {
  padding: 10px 2px;
  color: var(--dg-text-muted);
  font-size: 12px;
}

.routing__add {
  display: grid;
  grid-template-columns: auto minmax(0, 260px);
  align-items: center;
  gap: 12px;
  padding: 6px 2px 0;
}

.routing__add-label {
  color: var(--dg-text-secondary);
  font-size: 12px;
}

/* Moving an entry glides it to its new place. */
.reorder-move {
  transition: transform var(--dg-motion-base) var(--dg-ease-glide);
}

@media (max-width: 620px) {
  .routing__add { grid-template-columns: minmax(0, 1fr); }
}

@media (prefers-reduced-motion: reduce) {
  .reorder-move { transition: none; }
}
</style>
