<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import type { ChatMessageDTO } from '@/api/dto'

type ToolItem =
  | { kind: 'tool'; call: ChatMessageDTO | null; result: ChatMessageDTO | null }

const props = defineProps<{
  item: ToolItem
}>()

const { t, te } = useI18n()

// ---- expanded state (global per-id set managed by parent) ----
const showRawJson = ref(false)

// ---- helpers ----

function groupKey(): number {
  return (props.item.call ?? props.item.result)?.id ?? 0
}

function parseToolArguments(raw: string): Record<string, unknown> {
  if (raw === '') return {}
  try {
    const parsed: unknown = JSON.parse(raw)
    if (typeof parsed === 'object' && parsed !== null && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>
    }
  } catch { /* malformed */ }
  return {}
}

function stringArg(args: Record<string, unknown>, key: string): string {
  const value = args[key]
  return typeof value === 'string' ? value : ''
}

function numberArg(args: Record<string, unknown>, key: string): number | null {
  const value = args[key]
  return typeof value === 'number' ? value : null
}

// ---- tool label ----

function toolLabel(): string {
  const name = (props.item.call ?? props.item.result)?.toolName ?? ''
  const key = `chat.tools.names.${name}`
  if (te(key)) return t(key)
  return name === '' ? t('chat.tools.unknownTool') : `${t('chat.tools.unknownTool')} ${name}`
}

// ---- tool detail (one identifying arg shown on collapsed line) ----

function toolDetail(): string {
  const message = props.item.call ?? props.item.result
  if (message === null) return ''
  const args = parseToolArguments(props.item.call?.toolArguments ?? '')
  switch (message.toolName) {
    case 'timeline':
    case 'daily':
    case 'goal_set':
      return stringArg(args, 'day')
    case 'weekly':
      return stringArg(args, 'weekStart')
    case 'card':
    case 'card_update':
    case 'card_delete': {
      const cardId = numberArg(args, 'cardId')
      return cardId === null ? '' : `#${cardId}`
    }
    case 'category_add':
      return stringArg(args, 'name')
    case 'category_update':
    case 'category_remove': {
      const categoryId = stringArg(args, 'categoryId')
      return categoryId === '' ? '' : categoryId.length > 13 ? `${categoryId.slice(0, 13)}…` : categoryId
    }
    default:
      return ''
  }
}

// ---- outcome ----

interface ToolOutcome { ok: boolean; code: string }

function toolOutcome(): ToolOutcome | null {
  const result = props.item.result
  if (result === null) return null
  try {
    const parsed: unknown = JSON.parse(result.content)
    if (typeof parsed === 'object' && parsed !== null && 'ok' in parsed) {
      const envelope = parsed as { ok: unknown; error?: unknown }
      if (envelope.ok === true) return { ok: true, code: '' }
      let code = ''
      if (typeof envelope.error === 'object' && envelope.error !== null && 'code' in envelope.error) {
        const raw = (envelope.error as { code: unknown }).code
        if (typeof raw === 'string') code = raw
      }
      return { ok: false, code }
    }
  } catch { /* unparseable */ }
  return null
}

// ---- structured result display ----

interface KVEntry { key: string; value: string }

function structuredResult(): KVEntry[] {
  const result = props.item.result
  if (result === null) return []
  try {
    const parsed: unknown = JSON.parse(result.content)
    if (typeof parsed !== 'object' || parsed === null) return []
    const obj = parsed as Record<string, unknown>

    const entries: KVEntry[] = []
    // Detect common patterns
    if ('cards' in obj && Array.isArray(obj.cards)) {
      entries.push({ key: 'cards', value: t('chat.tools.timeline.cardsFound', { count: (obj.cards as unknown[]).length }) })
    }
    if ('categories' in obj && Array.isArray(obj.categories)) {
      entries.push({ key: 'categories', value: t('chat.tools.categories.count', { count: (obj.categories as unknown[]).length }) })
    }
    if ('ok' in obj) {
      entries.push({ key: 'ok', value: String(obj.ok) })
    }
    if ('error' in obj && typeof obj.error === 'object' && obj.error !== null) {
      const err = obj.error as Record<string, unknown>
      if ('message' in err) entries.push({ key: 'error.message', value: String(err.message) })
      if ('code' in err) entries.push({ key: 'error.code', value: String(err.code) })
    }
    // fallback: show top-level scalar keys
    for (const [k, v] of Object.entries(obj)) {
      if (['cards', 'categories', 'ok', 'error'].includes(k)) continue
      if (typeof v === 'string' || typeof v === 'number' || typeof v === 'boolean') {
        entries.push({ key: k, value: String(v) })
      }
    }
    return entries
  } catch { /* fall through */ }
  return []
}

// ---- pretty json ----

function prettyJson(raw: string): string {
  if (raw === '') return ''
  try {
    return JSON.stringify(JSON.parse(raw) as unknown, null, 2)
  } catch {
    return raw
  }
}

// ---- copy ----

const copiedArg = ref(false)
const copiedResult = ref(false)

async function copyArg(): Promise<void> {
  const args = props.item.call?.toolArguments ?? ''
  if (!args) return
  try {
    await navigator.clipboard.writeText(prettyJson(args))
    copiedArg.value = true
    setTimeout(() => { copiedArg.value = false }, 2000)
  } catch { /* unavailable */ }
}

async function copyResult(): Promise<void> {
  const content = props.item.result?.content ?? ''
  if (!content) return
  try {
    await navigator.clipboard.writeText(prettyJson(content))
    copiedResult.value = true
    setTimeout(() => { copiedResult.value = false }, 2000)
  } catch { /* unavailable */ }
}

// ---- derived states ----

const outcome = computed(() => toolOutcome())
const isPending = computed(() => props.item.result === null)
const detail = computed(() => toolDetail())
const structured = computed(() => structuredResult())

const headerClass = computed(() => {
  if (isPending.value) return 'tool-card__status--pending'
  if (!outcome.value) return ''
  return outcome.value.ok ? 'tool-card__status--ok' : 'tool-card__status--error'
})
</script>

<template>
  <div class="tool-card">
    <!-- Collapsed header (always visible) -->
    <button
      type="button"
      class="tool-card__head"
      @click="showRawJson = !showRawJson"
    >
      <!-- Status icon -->
      <span class="tool-card__icon" :class="headerClass">
        <svg v-if="isPending" class="tool-card__spinner" width="12" height="12" viewBox="0 0 16 16" fill="none">
          <circle cx="8" cy="8" r="6" stroke="currentColor" stroke-width="2" stroke-dasharray="20 18" stroke-linecap="round"/>
        </svg>
        <svg v-else-if="outcome?.ok" width="12" height="12" viewBox="0 0 16 16" fill="none">
          <circle cx="8" cy="8" r="6" stroke="currentColor" stroke-width="2"/>
          <path d="M5 8l2 2 4-4" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
        <svg v-else width="12" height="12" viewBox="0 0 16 16" fill="none">
          <circle cx="8" cy="8" r="6" stroke="currentColor" stroke-width="2"/>
          <path d="M8 5v4M8 11v.5" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
        </svg>
      </span>

      <!-- Tool name -->
      <span class="tool-card__label">{{ toolLabel() }}</span>

      <!-- Detail -->
      <span v-if="detail" class="tool-card__detail">{{ detail }}</span>

      <!-- Status badge -->
      <span v-if="isPending" class="tool-card__badge tool-card__badge--pending">
        {{ t('status.pending') }}
      </span>
      <span v-else-if="outcome" class="tool-card__badge" :class="outcome.ok ? 'tool-card__badge--ok' : 'tool-card__badge--error'">
        {{ outcome.ok ? t('chat.tools.resultOk') : (outcome.code !== '' ? `${t('chat.tools.resultError')}（${outcome.code}）` : t('chat.tools.resultError')) }}
      </span>

      <!-- Chevron -->
      <span class="tool-card__chevron" aria-hidden="true">
        {{ showRawJson ? '▾' : '▸' }}
      </span>
    </button>

    <!-- Expanded body -->
    <div v-if="showRawJson" class="tool-card__body">
      <!-- Arguments -->
      <div v-if="item.call && item.call.toolArguments" class="tool-card__section">
        <div class="tool-card__section-head">
          <span class="tool-card__section-label">{{ t('chat.tools.arguments') }}</span>
          <button type="button" class="tool-card__copy-btn" @click="copyArg">
            {{ copiedArg ? t('chat.tools.copied') : t('chat.tools.copyArguments') }}
          </button>
        </div>
        <pre class="tool-card__pre">{{ prettyJson(item.call.toolArguments) }}</pre>
      </div>

      <!-- Result -->
      <div v-if="item.result && item.result.content" class="tool-card__section">
        <div class="tool-card__section-head">
          <span class="tool-card__section-label">{{ t('chat.tools.result') }}</span>
          <button type="button" class="tool-card__copy-btn" @click="copyResult">
            {{ copiedResult ? t('chat.tools.copied') : t('chat.tools.copyResult') }}
          </button>
        </div>

        <!-- Structured view -->
        <div v-if="structured.length > 0" class="tool-card__kv">
          <div v-for="entry in structured" :key="entry.key" class="tool-card__kv-row">
            <span class="tool-card__kv-key">{{ entry.key }}</span>
            <span class="tool-card__kv-value">{{ entry.value }}</span>
          </div>
        </div>

        <pre class="tool-card__pre">{{ prettyJson(item.result.content) }}</pre>
      </div>
    </div>
  </div>
</template>

<style scoped>
.tool-card {
  align-self: stretch;
  border: 1px solid var(--dg-card-border);
  border-radius: 10px;
  background: var(--dg-track-fill);
  font-size: 12px;
  overflow: hidden;
}

.tool-card__head {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  min-height: 36px;
  padding: 6px 10px;
  border: none;
  background: none;
  color: var(--dg-text-secondary);
  cursor: pointer;
  text-align: left;
  transition: background var(--dg-motion-base) ease;
}

.tool-card__head:hover {
  background: var(--dg-hover-fill);
}

.tool-card__head:focus-visible {
  outline: none;
  box-shadow: inset 0 0 0 2px var(--dg-focus-ring);
}

/* Status icon */
.tool-card__icon {
  flex: none;
  display: flex;
  align-items: center;
}

.tool-card__status--ok { color: var(--dg-accent-text); }
.tool-card__status--error { color: var(--dg-danger); }
.tool-card__status--pending { color: var(--dg-text-muted); }

.tool-card__spinner {
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

/* Tool label */
.tool-card__label {
  flex: none;
  font-weight: 600;
  color: var(--dg-text-secondary);
}

.tool-card__detail {
  color: var(--dg-text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 11px;
}

/* Badge */
.tool-card__badge {
  flex: none;
  margin-left: auto;
  padding: 1px 7px;
  border-radius: 999px;
  font-size: 10px;
  font-weight: 620;
}

.tool-card__badge--ok {
  background: color-mix(in srgb, var(--dg-accent) 12%, transparent);
  color: var(--dg-accent-text);
}

.tool-card__badge--error {
  background: var(--dg-danger-fill);
  color: var(--dg-danger);
}

.tool-card__badge--pending {
  background: var(--dg-track-fill);
  color: var(--dg-text-muted);
  border: 1px solid var(--dg-chip-border);
}

/* Chevron */
.tool-card__chevron {
  flex: none;
  color: var(--dg-text-muted);
  font-size: 10px;
  margin-left: 4px;
}

/* ---- expanded body ---- */

.tool-card__body {
  border-top: 1px solid var(--dg-card-border);
  padding: 10px 12px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.tool-card__section {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.tool-card__section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.tool-card__section-label {
  color: var(--dg-text-muted);
  font-size: 10px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.tool-card__copy-btn {
  padding: 2px 8px;
  border: 1px solid var(--dg-chip-border);
  border-radius: 4px;
  background: none;
  color: var(--dg-text-muted);
  font-size: 10px;
  cursor: pointer;
  transition: background var(--dg-motion-base) ease, color var(--dg-motion-base) ease;
}

.tool-card__copy-btn:hover {
  background: var(--dg-hover-fill);
  color: var(--dg-text-secondary);
}

/* Structured key-value display */
.tool-card__kv {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 8px;
  border-radius: 6px;
  background: var(--dg-panel-fill);
}

.tool-card__kv-row {
  display: flex;
  gap: 8px;
  font-size: 12px;
}

.tool-card__kv-key {
  flex: none;
  color: var(--dg-text-muted);
  min-width: 80px;
}

.tool-card__kv-value {
  color: var(--dg-text-primary);
  font-weight: 500;
}

/* JSON pre */
.tool-card__pre {
  margin: 0;
  padding: 8px;
  border-radius: 6px;
  background: var(--dg-panel-fill);
  color: var(--dg-text-secondary);
  font-size: 11px;
  font-family: var(--dg-font-mono);
  line-height: 1.5;
  white-space: pre;
  overflow-x: auto;
  max-height: 200px;
  overflow-y: auto;
}
</style>
