<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'

import type { ChatMessageDTO } from '@/api/dto'
import { useChatStore } from '@/stores/chat'

import ChatBubble from './ChatBubble.vue'
import ChatDateDivider from './ChatDateDivider.vue'

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

const MS_PER_DAY = 86_400_000

/** Return the start-of-day for a given timestamp in ms. */
function dayStart(ts: number): number {
  return ts - (ts % MS_PER_DAY)
}

// Tool rows stay in the store (the pending flag reads the trailing role) but
// are never rendered: the transcript shows user and assistant messages only.
const renderItems = computed<RenderItem[]>(() => {
  const items: RenderItem[] = []
  const messages = store.messages
  let prevDay = -1

  for (const message of messages) {
    if (message.role === 'tool_call' || message.role === 'tool_result') continue

    // Insert date divider when day changes
    const day = dayStart(message.createdAt)
    if (day !== prevDay) {
      items.push({ kind: 'date', ts: message.createdAt })
      prevDay = day
    }

    items.push({ kind: 'message', message })
  }
  return items
})

function itemKey(item: RenderItem): string {
  if (item.kind === 'date') return `date-${item.ts}`
  return `msg-${item.message.id}`
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
        v-if="item.kind === 'message'"
        :content="item.message.content"
        :role="item.message.role as 'user' | 'assistant'"
        :timestamp="item.message.createdAt * 1000"
        :status="item.message.status as 'ok' | 'failed' | 'canceled' | ''"
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
