<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import LiquidGlassSurface from '@/components/LiquidGlassSurface.vue'
import { getSettings } from '@/api/settings'
import { useChatStore } from '@/stores/chat'

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  close: []
  error: [message: string]
}>()

const { t } = useI18n()
const store = useChatStore()

// ---- shared action wrapper ----

async function perform(action: () => Promise<unknown>): Promise<void> {
  emit('error', '')
  try {
    await action()
  } catch {
    emit('error', t('chat.actionError'))
  }
}

// ---- close on Escape ----

function onKeydown(e: KeyboardEvent): void {
  if (e.key === 'Escape' && props.open) emit('close')
}

watch(() => props.open, (val) => {
  if (val) {
    document.addEventListener('keydown', onKeydown)
  } else {
    document.removeEventListener('keydown', onKeydown)
  }
})

onUnmounted(() => {
  document.removeEventListener('keydown', onKeydown)
})

// ---- sidebar view ----

type SidebarView = 'conversations' | 'memory'
const sidebarView = ref<SidebarView>('conversations')
const transitioning = ref(false)

function switchView(view: SidebarView): void {
  if (view === sidebarView.value) return
  transitioning.value = true
  // Allow fade-out, then swap
  setTimeout(() => {
    sidebarView.value = view
    requestAnimationFrame(() => {
      transitioning.value = false
    })
  }, 90)
}

// ---- conversation list ----

const pendingRemoveId = ref<string | null>(null)

async function confirmRemove(): Promise<void> {
  const id = pendingRemoveId.value
  if (id === null) return
  pendingRemoveId.value = null
  await perform(() => store.removeConversation(id))
}

// ---- date grouping ----

interface GroupedConversation {
  date: 'today' | 'yesterday' | 'older'
  conversations: { id: string; title: string; updatedAt: number }[]
}

function groupConversations(list: { id: string; title: string; updatedAt: number }[]): GroupedConversation[] {
  const now = Date.now()
  const MS_PER_DAY = 86_400_000
  const msToday = now % MS_PER_DAY
  const todayStart = now - msToday
  const yesterdayStart = todayStart - MS_PER_DAY

  const groups: GroupedConversation[] = [
    { date: 'today', conversations: [] },
    { date: 'yesterday', conversations: [] },
    { date: 'older', conversations: [] },
  ]

  for (const conv of list) {
    const ts = conv.updatedAt
    if (ts >= todayStart) {
      groups[0].conversations.push(conv)
    } else if (ts >= yesterdayStart) {
      groups[1].conversations.push(conv)
    } else {
      groups[2].conversations.push(conv)
    }
  }

  return groups.filter((g) => g.conversations.length > 0)
}

const grouped = ref<GroupedConversation[]>([])

watch(() => store.listedConversations, (list) => {
  grouped.value = groupConversations(list as { id: string; title: string; updatedAt: number }[])
}, { immediate: true })

// ---- global memory ----

const memoryDraft = ref('')
const memorySaved = ref(false)
const memoryInput = ref<HTMLTextAreaElement | null>(null)
const memoryLoaded = ref(false)
const memorySaving = ref(false)
let savedTimer: ReturnType<typeof setTimeout> | undefined

onUnmounted(() => clearTimeout(savedTimer))

function autoGrow(el: HTMLTextAreaElement | null): void {
  if (!el) return
  el.style.height = 'auto'
  el.style.height = `${el.scrollHeight}px`
}

watch(memoryDraft, async () => {
  await nextTick()
  autoGrow(memoryInput.value)
})

watch(memoryDraft, () => {
  memorySaved.value = false
})

watch(sidebarView, (view) => {
  if (view === 'memory' && !memoryLoaded.value) void loadMemory()
})

async function loadMemory(): Promise<void> {
  try {
    const settings = await getSettings()
    memoryDraft.value = settings.chat?.memory ?? ''
    memoryLoaded.value = true
    await nextTick()
    autoGrow(memoryInput.value)
  } catch {
    emit('error', t('chat.loadError'))
  }
}

async function saveMemory(): Promise<void> {
  if (!memoryLoaded.value || memorySaving.value) return
  memorySaving.value = true
  const value = memoryDraft.value
  await perform(async () => {
    await store.saveMemory(value)
    memorySaved.value = memoryDraft.value === value
    clearTimeout(savedTimer)
    savedTimer = setTimeout(() => { memorySaved.value = false }, 2000)
  })
  memorySaving.value = false
}

// ---- derived ----

const hasConversations = computed(() => grouped.value.length > 0)
</script>

<template>
  <Teleport to="body">
    <!-- Backdrop -->
    <Transition name="drawer-fade">
      <div
        v-if="open"
        class="drawer-backdrop"
        aria-hidden="true"
        @click="emit('close')"
      />
    </Transition>

    <!-- Drawer panel: slides in from the right -->
    <Transition name="drawer-slide">
      <LiquidGlassSurface
        v-if="open"
        as="aside"
        intensity="dense"
        tracking
        class="drawer"
        role="complementary"
        :aria-label="t('chat.title')"
      >
        <!-- Sticky header: view tabs + close -->
        <header class="drawer__head">
          <nav class="drawer__tabs" role="tablist">
            <button
              type="button"
              role="tab"
              class="drawer__tab"
              :class="{ 'drawer__tab--active': sidebarView === 'conversations' }"
              :aria-selected="sidebarView === 'conversations'"
              @click="switchView('conversations')"
            >
              <span class="drawer__tab-text">{{ t('chat.drawer.conversations') }}</span>
            </button>
            <button
              type="button"
              role="tab"
              class="drawer__tab"
              :class="{ 'drawer__tab--active': sidebarView === 'memory' }"
              :aria-selected="sidebarView === 'memory'"
              @click="switchView('memory')"
            >
              <span class="drawer__tab-text">{{ t('chat.drawer.memory') }}</span>
            </button>
          </nav>
          <button
            type="button"
            class="drawer__close"
            :aria-label="t('chat.hideSidebar')"
            @click="emit('close')"
          >
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none" aria-hidden="true">
              <path d="M4 4l8 8M12 4l-8 8" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/>
            </svg>
          </button>
        </header>

        <!-- Body: view with cross-fade -->
        <div class="drawer__body" :class="{ 'drawer__body--fading': transitioning }">
          <!-- Conversations -->
          <section
            v-if="sidebarView === 'conversations'"
            class="drawer__pane drawer__pane--conversations"
            role="tabpanel"
          >
            <button
              type="button"
              class="drawer__new"
              @click="perform(store.newConversation); emit('close')"
            >
              <svg width="14" height="14" viewBox="0 0 16 16" fill="none" aria-hidden="true">
                <path d="M8 3v10M3 8h10" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/>
              </svg>
              <span>{{ t('chat.drawer.newChat') }}</span>
            </button>

            <div class="drawer__scroll dg-scroll">
              <p v-if="!hasConversations" class="drawer__empty">
                {{ t('chat.emptyConversations') }}
              </p>

              <template v-else>
                <div
                  v-for="group in grouped"
                  :key="group.date"
                  class="drawer__group"
                >
                  <h4 class="drawer__group-label">
                    {{ t(`chat.drawer.${group.date}`) }}
                  </h4>
                  <ul class="drawer__list">
                    <li
                      v-for="conv in group.conversations"
                      :key="conv.id"
                      class="drawer__item"
                      :class="{
                        'drawer__item--active': conv.id === store.activeId,
                        'drawer__item--removing': pendingRemoveId === conv.id,
                      }"
                    >
                      <button
                        type="button"
                        class="drawer__open"
                        @click="perform(() => store.select(conv.id)); emit('close')"
                      >
                        <span class="drawer__title">
                          {{ conv.title || t('chat.drawer.untitled') }}
                        </span>
                      </button>
                      <button
                        v-if="pendingRemoveId !== conv.id"
                        type="button"
                        class="drawer__delete"
                        :aria-label="t('chat.deleteConversation')"
                        @click="pendingRemoveId = conv.id"
                      >
                        <svg width="12" height="12" viewBox="0 0 16 16" fill="none" aria-hidden="true">
                          <path d="M3 4h10M6 4V3a1 1 0 0 1 1-1h2a1 1 0 0 1 1 1v1M12 4v9a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
                        </svg>
                      </button>
                    </li>
                  </ul>
                </div>
              </template>
            </div>
          </section>

          <!-- Memory -->
          <section
            v-else
            class="drawer__pane drawer__pane--memory"
            role="tabpanel"
          >
            <header class="memory__head">
              <h4 class="memory__title">{{ t('chat.memory.title') }}</h4>
              <p class="memory__hint">{{ t('chat.memory.hint') }}</p>
            </header>
            <textarea
              ref="memoryInput"
              v-model="memoryDraft"
              :disabled="!memoryLoaded"
              :aria-label="t('chat.memory.title')"
              class="dg-input memory__textarea"
              rows="6"
              :placeholder="t('chat.memory.placeholder')"
            />
            <footer class="memory__foot">
              <Transition name="saved-pop">
                <span v-if="memorySaved" class="memory__saved">
                  <svg width="12" height="12" viewBox="0 0 16 16" fill="none" aria-hidden="true">
                    <path d="M3 8l3 3 7-7" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
                  </svg>
                  {{ t('chat.memory.saved') }}
                </span>
              </Transition>
              <button
                type="button"
                class="memory__save"
                :disabled="!memoryLoaded || memorySaving"
                @click="saveMemory"
              >
                {{ t('chat.memory.save') }}
              </button>
            </footer>
          </section>
        </div>

        <!-- Confirm delete overlay (floating) -->
        <Transition name="confirm-pop">
          <div v-if="pendingRemoveId" class="confirm-bar">
            <span class="confirm-bar__text">{{ t('chat.removeConfirm') }}</span>
            <div class="confirm-bar__actions">
              <button
                type="button"
                class="confirm-bar__btn confirm-bar__btn--ghost"
                @click="pendingRemoveId = null"
              >
                {{ t('common.action.cancel') }}
              </button>
              <button
                type="button"
                class="confirm-bar__btn confirm-bar__btn--danger"
                @click="confirmRemove"
              >
                {{ t('common.action.delete') }}
              </button>
            </div>
          </div>
        </Transition>
      </LiquidGlassSurface>
    </Transition>
  </Teleport>
</template>

<style scoped>
/* ============================================================
   Backdrop
   ============================================================ */
.drawer-backdrop {
  position: fixed;
  inset: 0;
  background: rgb(0 0 0 / 0.22);
  backdrop-filter: blur(2px);
  -webkit-backdrop-filter: blur(2px);
  z-index: 40;
}

.drawer-fade-enter-active,
.drawer-fade-leave-active {
  transition: opacity 220ms ease;
}
.drawer-fade-enter-from,
.drawer-fade-leave-to {
  opacity: 0;
}

/* ============================================================
   Drawer shell — slides from the right
   ============================================================ */
.drawer {
  position: fixed;
  top: 0;
  right: 0;
  bottom: 0;
  width: 380px;
  max-width: calc(100vw - 64px);
  display: flex;
  flex-direction: column;
  z-index: 50;
  border-left: 1px solid var(--dg-card-border);
  border-radius: 0;
  overflow: hidden;
}

.drawer-slide-enter-active,
.drawer-slide-leave-active {
  transition:
    transform 280ms var(--dg-ease-glide),
    opacity 200ms ease;
}
.drawer-slide-enter-from,
.drawer-slide-leave-to {
  transform: translateX(100%);
  opacity: 0;
}

/* ============================================================
   Header (sticky)
   ============================================================ */
.drawer__head {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 14px 14px 12px;
  flex: none;
  border-bottom: 1px solid var(--dg-card-border);
}

.drawer__tabs {
  display: flex;
  flex: 1;
  gap: 2px;
  padding: 3px;
  border: 1px solid var(--dg-chip-border);
  border-radius: 8px;
  background: var(--dg-track-fill);
}

.drawer__tab {
  flex: 1;
  padding: 6px 8px;
  border: none;
  border-radius: 6px;
  background: none;
  color: var(--dg-text-secondary);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition:
    background-color var(--dg-motion-base) ease,
    color var(--dg-motion-base) ease,
    transform var(--dg-motion-fast) ease;
}

.drawer__tab:hover:not(.drawer__tab--active) {
  color: var(--dg-text-primary);
}

.drawer__tab--active {
  background: var(--dg-card-fill);
  color: var(--dg-text-primary);
  box-shadow:
    0 1px 0 rgb(255 255 255 / 0.04) inset,
    0 1px 2px rgb(0 0 0 / 0.08);
  font-weight: 600;
}

.drawer__close {
  flex: none;
  width: 30px;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--dg-chip-border);
  border-radius: 8px;
  background: var(--dg-track-fill);
  color: var(--dg-text-secondary);
  cursor: pointer;
  transition:
    background var(--dg-motion-base) ease,
    color var(--dg-motion-base) ease,
    border-color var(--dg-motion-base) ease,
    transform var(--dg-motion-fast) ease;
}

.drawer__close:hover {
  background: var(--dg-control-fill);
  color: var(--dg-text-primary);
}

.drawer__close:active {
  transform: scale(0.94);
}

/* ============================================================
   Body (scrolls between panes)
   ============================================================ */
.drawer__body {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  transition: opacity 90ms ease;
}

.drawer__body--fading {
  opacity: 0;
}

.drawer__pane {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 14px;
  overflow: hidden;
}

/* ---- conversations pane ---- */

.drawer__new {
  flex: none;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  width: 100%;
  padding: 9px 12px;
  border: 1px solid var(--dg-chip-border);
  border-radius: 8px;
  background: var(--dg-accent-fill);
  color: var(--dg-accent-text);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition:
    transform var(--dg-motion-fast) ease,
    filter var(--dg-motion-base) ease,
    opacity var(--dg-motion-base) ease;
}

.drawer__new:hover {
  filter: brightness(1.06);
}

.drawer__new:active {
  transform: scale(0.985);
}

.drawer__scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  margin: 0;
  padding: 0;
}

.drawer__empty {
  margin: 24px 0;
  color: var(--dg-text-muted);
  font-size: 12px;
  text-align: center;
}

.drawer__group + .drawer__group {
  margin-top: 10px;
}

.drawer__group-label {
  margin: 0 0 6px;
  padding: 0 4px;
  color: var(--dg-text-muted);
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

.drawer__list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.drawer__item {
  position: relative;
  display: flex;
  align-items: center;
  border-radius: 6px;
  transition: background-color var(--dg-motion-base) ease;
}

.drawer__item:hover {
  background: var(--dg-control-fill-hover);
}

.drawer__item--active {
  background: var(--dg-control-fill);
}

.drawer__item--active .drawer__title {
  color: var(--dg-accent-text);
  font-weight: 600;
}

.drawer__open {
  flex: 1;
  min-width: 0;
  padding: 8px 10px;
  border: none;
  background: none;
  text-align: left;
  cursor: pointer;
  border-radius: 6px;
}

.drawer__title {
  display: block;
  color: var(--dg-text-primary);
  font-size: 12.5px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.drawer__delete {
  flex: none;
  width: 28px;
  height: 28px;
  margin-right: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: none;
  border-radius: 5px;
  background: transparent;
  color: var(--dg-text-muted);
  cursor: pointer;
  opacity: 0;
  transition:
    opacity var(--dg-motion-base) ease,
    color var(--dg-motion-base) ease,
    background-color var(--dg-motion-base) ease;
}

.drawer__item:hover .drawer__delete {
  opacity: 1;
}

.drawer__delete:hover {
  color: var(--dg-danger);
  background: var(--dg-control-fill);
}

/* ---- memory pane ---- */

.drawer__pane--memory {
  gap: 12px;
}

.memory__head {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.memory__title {
  margin: 0;
  color: var(--dg-text-primary);
  font-size: 14px;
  font-weight: 600;
}

.memory__hint {
  margin: 0;
  color: var(--dg-text-muted);
  font-size: 11.5px;
  line-height: 1.45;
}

.memory__textarea {
  flex: 1;
  width: 100%;
  min-height: 0;
  padding: 10px 12px;
  border-radius: 8px;
  font-size: 12.5px;
  line-height: 1.55;
  resize: none;
  overflow-y: auto;
}

.memory__foot {
  flex: none;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  min-height: 28px;
}

.memory__saved {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: var(--dg-accent-text);
  font-size: 11.5px;
  font-weight: 500;
}

.saved-pop-enter-active,
.saved-pop-leave-active {
  transition: opacity 180ms ease, transform 180ms ease;
}
.saved-pop-enter-from,
.saved-pop-leave-to {
  opacity: 0;
  transform: translateY(2px);
}

.memory__save {
  padding: 6px 14px;
  border: 1px solid var(--dg-chip-border);
  border-radius: 7px;
  background: var(--dg-accent-fill);
  color: var(--dg-accent-text);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition:
    filter var(--dg-motion-base) ease,
    transform var(--dg-motion-fast) ease,
    opacity var(--dg-motion-base) ease;
}

.memory__save:hover:not(:disabled) {
  filter: brightness(1.06);
}

.memory__save:active:not(:disabled) {
  transform: scale(0.96);
}

.memory__save:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

/* ============================================================
   Confirm bar (floating above body)
   ============================================================ */
.confirm-bar {
  position: absolute;
  left: 12px;
  right: 12px;
  bottom: 12px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 10px 12px;
  border: 1px solid var(--dg-card-border);
  border-radius: 10px;
  background: var(--dg-card-fill);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  box-shadow:
    0 8px 24px rgb(0 0 0 / 0.18),
    0 1px 0 rgb(255 255 255 / 0.04) inset;
}

.confirm-bar__text {
  color: var(--dg-text-primary);
  font-size: 12px;
  flex: 1;
  min-width: 0;
}

.confirm-bar__actions {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: none;
}

.confirm-bar__btn {
  padding: 5px 10px;
  border-radius: 6px;
  border: 1px solid var(--dg-chip-border);
  background: var(--dg-track-fill);
  color: var(--dg-text-secondary);
  font-size: 11.5px;
  font-weight: 500;
  cursor: pointer;
  transition:
    background var(--dg-motion-base) ease,
    color var(--dg-motion-base) ease,
    transform var(--dg-motion-fast) ease;
}

.confirm-bar__btn:active {
  transform: scale(0.96);
}

.confirm-bar__btn--ghost:hover {
  background: var(--dg-control-fill);
  color: var(--dg-text-primary);
}

.confirm-bar__btn--danger {
  background: var(--dg-danger-fill);
  border-color: transparent;
  color: var(--dg-danger-text);
}

.confirm-bar__btn--danger:hover {
  filter: brightness(1.08);
}

.confirm-pop-enter-active,
.confirm-pop-leave-active {
  transition: transform 200ms var(--dg-ease-glide), opacity 180ms ease;
}
.confirm-pop-enter-from,
.confirm-pop-leave-to {
  opacity: 0;
  transform: translateY(8px);
}

/* ============================================================
   Narrow viewports
   ============================================================ */
@media (max-width: 480px) {
  .drawer {
    width: calc(100vw - 32px);
    max-width: none;
  }
}
</style>
