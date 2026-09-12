<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { clearFatalError, fatalError } from '@/lib/fatalError'

/**
 * The last line of defence against a blank window: whenever a fatal error is
 * recorded, this shows it on top of everything with actions to recover. Kept
 * dependency-free beyond i18n so it can render even when a page is broken.
 */
const { t } = useI18n()
const copied = ref(false)

interface WailsReloadRuntime {
  runtime?: { WindowReload?: () => void }
}

function reloadApp(): void {
  const reload = (window as WailsReloadRuntime).runtime?.WindowReload
  if (typeof reload === 'function') reload()
  else window.location.reload()
}

async function copyDetails(): Promise<void> {
  const value = fatalError.value
  if (value === null) return
  const text = [value.source, value.message, '', value.detail].join('\n').trim()
  try {
    await navigator.clipboard.writeText(text)
    copied.value = true
    window.setTimeout(() => {
      copied.value = false
    }, 1600)
  } catch {
    // Clipboard blocked: the text is already on screen to copy by hand.
  }
}
</script>

<template>
  <Transition name="fatal">
    <div v-if="fatalError !== null" class="fatal" role="alertdialog" aria-modal="true">
      <div class="fatal__panel">
        <p class="fatal__eyebrow">{{ fatalError.source }}</p>
        <h1 class="fatal__title">{{ t('common.fatal.title') }}</h1>
        <p class="fatal__description">{{ t('common.fatal.description') }}</p>
        <p class="fatal__message">{{ fatalError.message }}</p>
        <pre v-if="fatalError.detail" class="fatal__detail">{{ fatalError.detail }}</pre>
        <div class="fatal__actions">
          <button type="button" class="fatal__button fatal__button--primary" @click="reloadApp">
            {{ t('common.fatal.reload') }}
          </button>
          <button type="button" class="fatal__button" @click="copyDetails">
            {{ copied ? t('common.fatal.copied') : t('common.fatal.copy') }}
          </button>
          <button type="button" class="fatal__button" @click="clearFatalError">
            {{ t('common.fatal.dismiss') }}
          </button>
        </div>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.fatal {
  position: fixed;
  z-index: 9999;
  inset: 0;
  display: grid;
  place-items: center;
  padding: 24px;
  background: color-mix(in srgb, var(--dg-window-bg) 78%, transparent);
  backdrop-filter: blur(6px);
}

.fatal__panel {
  display: flex;
  flex-direction: column;
  gap: 10px;
  width: min(560px, 100%);
  max-height: min(80vh, 640px);
  padding: 24px;
  overflow-y: auto;
  border: 1px solid color-mix(in srgb, var(--dg-danger) 32%, transparent);
  border-radius: 14px;
  background: var(--dg-panel-fill);
  box-shadow: var(--dg-panel-shadow);
}

.fatal__eyebrow {
  color: var(--dg-danger);
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.fatal__title {
  color: var(--dg-text-primary);
  font-size: 20px;
  font-weight: 650;
  line-height: 1.2;
}

.fatal__description {
  color: var(--dg-text-secondary);
  font-size: 12px;
  line-height: 1.6;
}

.fatal__message {
  padding: 10px 12px;
  border-radius: 8px;
  background: var(--dg-danger-fill);
  color: var(--dg-danger);
  font-size: 12px;
  font-weight: 550;
  word-break: break-word;
}

.fatal__detail {
  max-height: 240px;
  margin: 0;
  padding: 12px;
  overflow: auto;
  border: 1px solid var(--dg-chip-border);
  border-radius: 8px;
  background: var(--dg-hover-fill);
  color: var(--dg-text-muted);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 11px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word;
}

.fatal__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 4px;
}

.fatal__button {
  min-height: 32px;
  padding: 6px 14px;
  border: 1px solid var(--dg-chip-border);
  border-radius: 8px;
  color: var(--dg-text-primary);
  font-size: 12px;
  font-weight: 550;
}

.fatal__button:hover {
  background: var(--dg-hover-fill);
}

.fatal__button:focus-visible {
  outline: none;
  box-shadow: 0 0 0 3px var(--dg-focus-ring);
}

.fatal__button--primary {
  border-color: transparent;
  background: var(--dg-accent);
  color: #fff;
}

.fatal-enter-active,
.fatal-leave-active {
  transition: opacity var(--dg-motion-base, 200ms) ease;
}

.fatal-enter-from,
.fatal-leave-to {
  opacity: 0;
}

@media (prefers-reduced-motion: reduce) {
  .fatal-enter-active,
  .fatal-leave-active {
    transition: none;
  }
}
</style>
