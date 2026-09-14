<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import type { ChatMessageDTO } from '@/api/dto'
import { useChatStore } from '@/stores/chat'

import ChatBubble from './ChatBubble.vue'
import ChatDateDivider from './ChatDateDivider.vue'
import ChatToolCard from './ChatToolCard.vue'

const { t } = useI18n()
const store = useChatStore()

const scroller = ref<HTMLElement | null>(null)

async function scrollToBottom(): Promise<void> {
  await nextTick()
  const el = scroller.value
  if (el !== null) el.scrollTop = el.scrollHeight
}

watch(
  () => store.messages.at(-1)?.id,
  () => { void scrollToBottom() },
)

watch(
  () => store.activeId,
  () => { void scrollToBottom() },
)

// ---- render items with date dividers ----

type RenderItem =
  | { kind: 'date'; ts: number }
  | { kind: 'message'; message: ChatMessageDTO }
  | { kind: 'tool'; call: ChatMessageDTO | null; result: ChatMessageDTO | null }

const MS_PER_DAY = 86_400_000

/** Return the start-of-day for a given timestamp in ms. */
function dayStart(ts: number): number {
  return ts - (ts % MS_PER_DAY)
}

const renderItems = computed<RenderItem[]>(() => {
  const items: RenderItem[] = []
  const messages = store.messages
  let prevDay = -1

  for (let i = 0; i < messages.length; i++) {
    const message = messages[i]

    // Insert date divider when day changes
    const day = dayStart(message.createdAt)
    if (day !== prevDay) {
      items.push({ kind: 'date', ts: message.createdAt })
      prevDay = day
    }

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

function itemKey(item: RenderItem): string {
  if (item.kind === 'date') return `date-${item.ts}`
  if (item.kind === 'message') return `msg-${item.message.id}`
  const id = (item.call ?? item.result)?.id ?? 0
  return `tool-${id}`
}

function groupKey(item: { kind: 'tool'; call: ChatMessageDTO | null; result: ChatMessageDTO | null }): number {
  return (item.call ?? item.result)?.id ?? 0
}
</script>

<template>
  <div ref="scroller" class="transcript dg-scroll" :aria-busy="store.loading">
    <template v-if="store.messages.length === 0 && !store.loading">
      <!-- Empty handled by ChatWelcome in parent -->
      <slot name="empty" />
    </template>

    <template v-for="item in renderItems" :key="itemKey(item)">
      <!-- Date divider -->
      <ChatDateDivider
        v-if="item.kind === 'date'"
        :timestamp="item.ts * 1000"
      />

      <!-- Message bubble -->
      <ChatBubble
        v-else-if="item.kind === 'message'"
        :content="item.message.content"
        :role="item.message.role as 'user' | 'assistant'"
        :timestamp="item.message.createdAt * 1000"
        :status="item.message.status as 'ok' | 'failed' | 'canceled' | ''"
      />

      <!-- Tool call card -->
      <ChatToolCard
        v-else-if="item.kind === 'tool'"
        :item="item"
      />
    </template>
  </div>
</template>

<style scoped>
.transcript {
  display: flex;
  flex-direction: column;
  gap: 20px;
  flex: 1;
  min-height: 0;
  padding: 24px 12px;
  overflow-y: auto;
}
</style>
