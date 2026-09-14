<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import type { ChatMessageDTO } from '@/api/dto'
import { useChatStore } from '@/stores/chat'

const { t, te } = useI18n()
const store = useChatStore()

const scroller = ref<HTMLElement | null>(null)

async function scrollToBottom(): Promise<void> {
  await nextTick()
  const element = scroller.value
  if (element !== null) element.scrollTop = element.scrollHeight
}

watch(
  () => store.messages.at(-1)?.id,
  () => {
    void scrollToBottom()
  },
)
watch(
  () => store.activeId,
  () => {
    void scrollToBottom()
  },
)

function statusLabel(status: string): string {
  if (status === 'failed') return t('chat.status.failed')
  if (status === 'canceled') return t('chat.status.canceled')
  return ''
}

// ---- tool message groups ----

/**
 * Render units for the transcript. A tool_call and its adjacent tool_result
 * collapse into one group; everything else renders as a plain bubble. A
 * result whose call fell outside the page still renders (with no arguments),
 * so paging can never hide a half-pair.
 */
type RenderItem =
  | { kind: 'message'; message: ChatMessageDTO }
  | { kind: 'tool'; call: ChatMessageDTO | null; result: ChatMessageDTO | null }

const renderItems = computed<RenderItem[]>(() => {
  const items: RenderItem[] = []
  const messages = store.messages
  for (let i = 0; i < messages.length; i++) {
    const message = messages[i]
    if (message.role === 'tool_call') {
      const next = messages[i + 1]
      const result = next !== undefined && next.role === 'tool_result' ? next : null
      if (result !== null) i++
      items.push({ kind: 'tool', call: message, result })
    } else if (message.role === 'tool_result') {
      items.push({ kind: 'tool', call: null, result: message })
    } else {
      items.push({ kind: 'message', message })
    }
  }
  return items
})

const expandedToolIds = ref(new Set<number>())

function groupKey(item: Extract<RenderItem, { kind: 'tool' }>): number {
  return (item.call ?? item.result)?.id ?? 0
}

function isToolExpanded(item: Extract<RenderItem, { kind: 'tool' }>): boolean {
  return expandedToolIds.value.has(groupKey(item))
}

function toggleTool(item: Extract<RenderItem, { kind: 'tool' }>): void {
  const key = groupKey(item)
  const next = new Set(expandedToolIds.value)
  if (next.has(key)) next.delete(key)
  else next.add(key)
  expandedToolIds.value = next
}

/** Arguments JSON narrowed through unknown, per the Wails boundary rule. */
function parseToolArguments(raw: string): Record<string, unknown> {
  if (raw === '') return {}
  try {
    const parsed: unknown = JSON.parse(raw)
    if (typeof parsed === 'object' && parsed !== null && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>
    }
  } catch {
    // Malformed arguments render as empty; the raw text stays one click away.
  }
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

function toolLabel(item: Extract<RenderItem, { kind: 'tool' }>): string {
  const name = (item.call ?? item.result)?.toolName ?? ''
  const key = `chat.tools.names.${name}`
  if (te(key)) return t(key)
  return name === '' ? t('chat.tools.unknownTool') : `${t('chat.tools.unknownTool')} ${name}`
}

/** One identifying argument per tool, shown on the collapsed line. */
function toolDetail(item: Extract<RenderItem, { kind: 'tool' }>): string {
  const message = item.call ?? item.result
  if (message === null) return ''
  const args = parseToolArguments(item.call?.toolArguments ?? '')
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

interface ToolOutcome {
  ok: boolean
  code: string
}

/** The result envelope's verdict, or null when there is no result yet. */
function toolOutcome(item: Extract<RenderItem, { kind: 'tool' }>): ToolOutcome | null {
  const result = item.result
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
  } catch {
    // Unparseable content counts as no verdict rather than an error.
  }
  return null
}

function prettyJSON(raw: string): string {
  if (raw === '') return ''
  try {
    return JSON.stringify(JSON.parse(raw) as unknown, null, 2)
  } catch {
    return raw
  }
}
</script>

<template>
  <div ref="scroller" class="transcript dg-scroll" :aria-busy="store.loading">
    <p v-if="store.messages.length === 0 && !store.loading" class="transcript__empty">
      {{ t('chat.emptyConversations') }}
    </p>
    <template v-for="item in renderItems" :key="item.kind === 'message' ? item.message.id : groupKey(item)">
      <div
        v-if="item.kind === 'message'"
        class="bubble"
        :class="`bubble--${item.message.role}`"
      >
        <p class="bubble__text">{{ item.message.content }}</p>
        <span
          v-if="item.message.role === 'assistant' && item.message.status !== 'ok' && item.message.status !== ''"
          class="bubble__status"
          :class="`bubble__status--${item.message.status}`"
        >
          {{ statusLabel(item.message.status) }}
        </span>
      </div>

      <div v-else class="tool-group">
        <button
          type="button"
          class="tool-group__head"
          :aria-expanded="isToolExpanded(item)"
          :title="isToolExpanded(item) ? t('chat.tools.collapse') : t('chat.tools.expand')"
          @click="toggleTool(item)"
        >
          <span class="tool-group__chevron" aria-hidden="true">
            {{ isToolExpanded(item) ? '▾' : '▸' }}
          </span>
          <span class="tool-group__label">{{ toolLabel(item) }}</span>
          <span v-if="toolDetail(item) !== ''" class="tool-group__detail">
            {{ toolDetail(item) }}
          </span>
          <span
            v-if="toolOutcome(item) !== null"
            class="tool-group__status"
            :class="toolOutcome(item)?.ok ? 'tool-group__status--ok' : 'tool-group__status--error'"
          >
            {{ toolOutcome(item)?.ok
              ? t('chat.tools.resultOk')
              : toolOutcome(item)?.code !== ''
                ? `${t('chat.tools.resultError')}（${toolOutcome(item)?.code}）`
                : t('chat.tools.resultError') }}
          </span>
        </button>
        <div v-if="isToolExpanded(item)" class="tool-group__body">
          <section v-if="item.call !== null && prettyJSON(item.call.toolArguments) !== ''">
            <p class="tool-group__section-label">{{ t('chat.tools.arguments') }}</p>
            <pre class="tool-group__pre">{{ prettyJSON(item.call.toolArguments) }}</pre>
          </section>
          <section v-if="item.result !== null && prettyJSON(item.result.content) !== ''">
            <p class="tool-group__section-label">{{ t('chat.tools.result') }}</p>
            <pre class="tool-group__pre">{{ prettyJSON(item.result.content) }}</pre>
          </section>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.transcript {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 10px;
  min-height: 0;
  padding: 12px 0;
  overflow-y: auto;
}

.transcript__empty {
  margin: auto;
  color: var(--dg-text-muted);
  font-size: 13px;
}

/* ---- bubbles ---- */

.bubble {
  max-width: 78%;
  padding: 9px 12px;
  border-radius: 10px;
  font-size: 13px;
  line-height: 1.55;
}

/* Accent-tinted glass: the gradient and specular top edge give the user's
   own words a touch more material than the assistant's frosted card. */
.bubble--user {
  align-self: flex-end;
  border: 1px solid color-mix(in srgb, var(--dg-accent) 24%, transparent);
  border-bottom-right-radius: 4px;
  background: linear-gradient(
    180deg,
    color-mix(in srgb, var(--dg-accent) 19%, transparent),
    color-mix(in srgb, var(--dg-accent) 12%, transparent)
  );
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.16);
  color: var(--dg-text-primary);
}

.bubble--assistant {
  align-self: flex-start;
  border: 1px solid var(--dg-card-border);
  border-bottom-left-radius: 4px;
  background: var(--dg-card-fill);
  box-shadow: var(--dg-card-shadow);
  color: var(--dg-text-primary);
}

/* New entries settle in. Enter-only, so a history render cannot strand a
   leave state; existing keyed nodes are not remounted and do not replay. */
@media (prefers-reduced-motion: no-preference) {
  .bubble,
  .tool-group {
    animation: bubble-in var(--dg-motion-base) var(--dg-ease-glide) both;
  }
}

@keyframes bubble-in {
  from {
    opacity: 0;
    transform: translateY(5px);
  }
}

.bubble__text {
  margin: 0;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.bubble__status {
  display: inline-block;
  margin-top: 6px;
  padding: 2px 7px;
  border-radius: 999px;
  background: var(--dg-danger-fill);
  color: var(--dg-danger);
  font-size: 10px;
  font-weight: 620;
}

/* ---- tool groups ---- */

.tool-group {
  align-self: stretch;
  border: 1px solid var(--dg-card-border);
  border-radius: 8px;
  background: var(--dg-track-fill);
  font-size: 12px;
  overflow: hidden;
}

.tool-group__head {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  min-height: 32px;
  padding: 4px 10px;
  border: none;
  background: none;
  color: var(--dg-text-secondary);
  cursor: pointer;
  text-align: left;
}

.tool-group__head:hover {
  background: var(--dg-hover-fill);
}

.tool-group__head:focus-visible {
  outline: none;
  box-shadow: inset 0 0 0 2px var(--dg-focus-ring);
}

.tool-group__chevron {
  flex: none;
  color: var(--dg-text-muted);
  font-size: 10px;
}

.tool-group__label {
  flex: none;
  color: var(--dg-text-secondary);
  font-weight: 600;
}

.tool-group__detail {
  min-width: 0;
  color: var(--dg-text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tool-group__status {
  flex: none;
  margin-left: auto;
  font-size: 10px;
  font-weight: 600;
}

.tool-group__status--ok {
  color: var(--dg-accent-text);
}

.tool-group__status--error {
  color: var(--dg-danger);
}

.tool-group__body {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 8px 10px 10px 22px;
  border-top: 1px solid var(--dg-card-border);
}

.tool-group__section-label {
  margin: 0 0 3px;
  color: var(--dg-text-muted);
  font-size: 10px;
  font-weight: 600;
}

.tool-group__pre {
  margin: 0;
  padding: 8px;
  border-radius: 6px;
  background: var(--dg-panel-fill);
  color: var(--dg-text-secondary);
  font-size: 11px;
  line-height: 1.5;
  white-space: pre;
  overflow-x: auto;
}
</style>
