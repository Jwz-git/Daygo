<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { getAgentConnection, type AgentConnection } from '@/api/agent'
import { cliCommand, isEphemeralExecutable, mcpClientConfig } from '@/lib/agentConnection'
import { resolveDesktopPlatform, type DesktopPlatform } from '@/lib/desktopPlatform'
import SettingGroup from './SettingGroup.vue'
import SettingRow from './SettingRow.vue'
import SwitchControl from './SwitchControl.vue'
import { useSettingsSection } from './useSettingsSection'

type Tab = 'mcp' | 'cli'
type SnippetId = 'mcp' | 'read' | 'write'

const { t } = useI18n()
const { state, settings, load, persist, writeFailed } = useSettingsSection()

const enabled = computed(() => settings.value?.system.agentEditsEnabled ?? false)
const chatEdits = computed(() => settings.value?.chat.editMode === 'edits')

// null until loaded or when the binding is unavailable; the snippets then
// fall back to the bare `daygo` command name.
const connection = ref<AgentConnection | null>(null)
const platform = ref<DesktopPlatform>('unknown')
const executable = computed(() => connection.value?.executablePath ?? '')
const ephemeral = computed(() => isEphemeralExecutable(executable.value))
// The Windows release is a GUI-subsystem binary, so a terminal gets no output
// from it; only the pipe-driven MCP entry is offered there.
const cliAvailable = computed(() => platform.value !== 'windows')

const tab = ref<Tab>('mcp')
const activeTab = computed<Tab>(() => (cliAvailable.value ? tab.value : 'mcp'))

const mcpSnippet = computed(() => mcpClientConfig(executable.value))
const readSnippet = computed(() => cliCommand(executable.value, ['timeline', 'today', '--json']))
const writeSnippet = computed(() =>
  cliCommand(executable.value, ['write', 'goal_set', '{"day":"YYYY-MM-DD","focusTargetMinutes":120}', '--json']),
)

const status = computed(() => {
  if (connection.value === null) return null
  if (!connection.value.socketActive) return 'inactive'
  return enabled.value ? 'writable' : 'readonly'
})

const copied = ref<SnippetId | null>(null)
let copiedTimer = 0

onMounted(() => {
  void load()
  void getAgentConnection().then(
    (value) => { connection.value = value },
    () => { connection.value = null },
  )
  void resolveDesktopPlatform().then((value) => { platform.value = value })
})
onBeforeUnmount(() => window.clearTimeout(copiedTimer))

function onToggle(next: boolean): void {
  void persist({ agentEditsEnabled: next })
}

function onToggleChatEdits(next: boolean): void {
  void persist({ chatEditMode: next ? 'edits' : 'readonly' })
}

async function copy(id: SnippetId, text: string): Promise<void> {
  window.clearTimeout(copiedTimer)
  try {
    await navigator.clipboard.writeText(text)
    copied.value = id
  } catch {
    copied.value = null
    return
  }
  copiedTimer = window.setTimeout(() => { copied.value = null }, 1600)
}
</script>

<template>
  <div class="agent">
    <SettingGroup :title="t('settings.agentAccess.connectTitle')" :hint="t('settings.agentAccess.connectHint')">
      <div class="agent__bar">
        <div v-if="cliAvailable" class="segmented" role="tablist">
          <button
            v-for="id in (['mcp', 'cli'] as const)"
            :key="id"
            type="button"
            role="tab"
            class="segmented__item"
            :class="{ 'is-active': activeTab === id }"
            :aria-selected="activeTab === id"
            @click="tab = id"
          >
            {{ t(`settings.agentAccess.tabs.${id}`) }}
          </button>
        </div>
        <span v-if="status" class="status" :class="`status--${status}`">
          <span class="status__dot" aria-hidden="true" />
          {{ t(`settings.agentAccess.status.${status}`) }}
        </span>
      </div>

      <p v-if="ephemeral" class="notice" role="alert">{{ t('settings.agentAccess.translocated') }}</p>

      <div v-if="activeTab === 'mcp'" class="panel" role="tabpanel">
        <p class="panel__lead">{{ t('settings.agentAccess.mcp.lead') }}</p>
        <div class="snippet snippet--block">
          <pre><code>{{ mcpSnippet }}</code></pre>
          <button
            type="button"
            class="snippet__copy"
            :class="{ 'is-done': copied === 'mcp' }"
            @click="copy('mcp', mcpSnippet)"
          >
            {{ copied === 'mcp' ? t('settings.agentAccess.copied') : t('settings.agentAccess.copy') }}
          </button>
        </div>
        <p class="panel__note">{{ t('settings.agentAccess.mcp.note') }}</p>
        <p v-if="!cliAvailable" class="panel__note">{{ t('settings.agentAccess.cli.windows') }}</p>
      </div>

      <div v-else class="panel" role="tabpanel">
        <p class="panel__lead">{{ t('settings.agentAccess.cli.lead') }}</p>
        <div class="step">
          <span class="step__label">{{ t('settings.agentAccess.cli.read') }}</span>
          <div class="snippet">
            <pre><code>{{ readSnippet }}</code></pre>
            <button
              type="button"
              class="snippet__copy"
              :class="{ 'is-done': copied === 'read' }"
              @click="copy('read', readSnippet)"
            >
              {{ copied === 'read' ? t('settings.agentAccess.copied') : t('settings.agentAccess.copy') }}
            </button>
          </div>
        </div>
        <div class="step">
          <span class="step__label">{{ t('settings.agentAccess.cli.write') }}</span>
          <div class="snippet">
            <pre><code>{{ writeSnippet }}</code></pre>
            <button
              type="button"
              class="snippet__copy"
              :class="{ 'is-done': copied === 'write' }"
              @click="copy('write', writeSnippet)"
            >
              {{ copied === 'write' ? t('settings.agentAccess.copied') : t('settings.agentAccess.copy') }}
            </button>
          </div>
        </div>
      </div>
    </SettingGroup>

    <SettingGroup :title="t('settings.agentAccess.permissionsTitle')" :hint="t('settings.agentAccess.permissionsHint')">
      <SettingRow :title="t('settings.agentAccess.editsTitle')" :hint="t('settings.agentAccess.editsHint')">
        <SwitchControl
          :checked="enabled"
          :disabled="state !== 'ready'"
          :label="t('settings.agentAccess.editsTitle')"
          @toggle="onToggle"
        />
      </SettingRow>
      <SettingRow :title="t('settings.agentAccess.chatEditsTitle')" :hint="t('settings.agentAccess.chatEditsHint')">
        <SwitchControl
          :checked="chatEdits"
          :disabled="state !== 'ready'"
          :label="t('settings.agentAccess.chatEditsTitle')"
          @toggle="onToggleChatEdits"
        />
      </SettingRow>
      <p v-if="writeFailed" role="alert" class="write-error">{{ t('settings.agentAccess.writeError') }}</p>
    </SettingGroup>
  </div>
</template>

<style scoped>
.agent {
  display: flex;
  flex-direction: column;
  gap: 28px;
}

.agent__bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-top: 6px;
}

/* Small control: solid platform texture — a tinted track with a white thumb. */
.segmented {
  display: inline-flex;
  padding: 2px;
  border-radius: 8px;
  background: var(--dg-track-fill);
}

.segmented__item {
  min-width: 84px;
  padding: 5px 14px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--dg-text-secondary);
  font-size: 12px;
  font-weight: 550;
  transition:
    background var(--dg-motion-fast) ease,
    color var(--dg-motion-fast) ease,
    box-shadow var(--dg-motion-fast) ease;
}

.segmented__item:not(.is-active):hover {
  color: var(--dg-text-primary);
}

.segmented__item.is-active {
  background: var(--dg-segment-thumb);
  color: var(--dg-text-primary);
  box-shadow: var(--dg-switch-knob-shadow);
}

.segmented__item:focus-visible {
  outline: none;
  box-shadow: 0 0 0 3px var(--dg-focus-ring);
}

.status {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 4px 10px;
  border: 1px solid var(--dg-chip-border);
  border-radius: 999px;
  background: var(--dg-chip-fill);
  color: var(--dg-text-secondary);
  font-size: 12px;
}

.status__dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--dg-text-secondary);
}

.status--writable .status__dot {
  background: var(--dg-success);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--dg-success) 22%, transparent);
}

.status--readonly .status__dot {
  background: var(--dg-accent);
}

.status--inactive {
  color: var(--dg-danger);
}

.status--inactive .status__dot {
  background: var(--dg-danger);
}

.notice {
  padding: 10px 12px;
  border: 1px solid color-mix(in srgb, var(--dg-warning) 45%, transparent);
  border-radius: 10px;
  background: color-mix(in srgb, var(--dg-warning) 8%, transparent);
  color: var(--dg-text-primary);
  font-size: 12px;
  line-height: 1.5;
}

.panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding-top: 4px;
}

.panel__lead {
  color: var(--dg-text-primary);
  font-size: 13px;
}

.panel__note {
  color: var(--dg-text-secondary);
  font-size: 12px;
  line-height: 1.5;
}

.step {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.step__label {
  color: var(--dg-text-secondary);
  font-size: 12px;
}

.snippet {
  position: relative;
  border: 1px solid var(--dg-input-border);
  border-radius: 10px;
  background: var(--dg-input-fill);
}

.snippet pre {
  margin: 0;
  padding: 10px 76px 10px 12px;
  color: var(--dg-text-primary);
  font-family: var(--dg-font-mono);
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  user-select: text;
}

.snippet--block pre {
  padding-top: 12px;
  padding-bottom: 12px;
}

.snippet__copy {
  position: absolute;
  top: 6px;
  right: 6px;
  min-width: 58px;
  padding: 4px 10px;
  border: none;
  border-radius: 6px;
  background: var(--dg-button-secondary-fill);
  color: var(--dg-button-secondary-text);
  font-size: 12px;
  font-weight: 550;
  transition: background var(--dg-motion-fast) ease, color var(--dg-motion-fast) ease;
}

.snippet__copy:hover {
  background: var(--dg-button-secondary-hover);
}

.snippet__copy.is-done {
  color: var(--dg-success);
}

.snippet__copy:focus-visible {
  outline: none;
  box-shadow: 0 0 0 3px var(--dg-focus-ring);
}

.write-error {
  color: var(--dg-danger);
  font-size: 12px;
}

@media (prefers-reduced-motion: reduce) {
  .segmented__item,
  .snippet__copy {
    transition: none;
  }
}
</style>
