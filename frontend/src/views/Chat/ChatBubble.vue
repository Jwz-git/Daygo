<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import hljs from 'highlight.js/lib/core'
import bash from 'highlight.js/lib/languages/bash'
import css from 'highlight.js/lib/languages/css'
import diff from 'highlight.js/lib/languages/diff'
import go from 'highlight.js/lib/languages/go'
import javascript from 'highlight.js/lib/languages/javascript'
import json from 'highlight.js/lib/languages/json'
import markdown from 'highlight.js/lib/languages/markdown'
import python from 'highlight.js/lib/languages/python'
import shell from 'highlight.js/lib/languages/shell'
import sql from 'highlight.js/lib/languages/sql'
import typescript from 'highlight.js/lib/languages/typescript'
import xml from 'highlight.js/lib/languages/xml'
import yaml from 'highlight.js/lib/languages/yaml'
import DOMPurify from 'dompurify'

// Register only the languages chat realistically renders. The full
// highlight.js build bundles ~190 languages (~1MB); registering a subset keeps
// the Chat chunk small. An unlisted fence falls back to plaintext below.
for (const [name, lang] of [
  ['bash', bash],
  ['css', css],
  ['diff', diff],
  ['go', go],
  ['javascript', javascript],
  ['json', json],
  ['markdown', markdown],
  ['python', python],
  ['shell', shell],
  ['sql', sql],
  ['typescript', typescript],
  ['xml', xml],
  ['yaml', yaml],
] as const) {
  hljs.registerLanguage(name, lang)
}

const props = defineProps<{
  content: string
  role: 'user' | 'assistant'
  /** Unix timestamp in ms */
  timestamp?: number
  /** For assistant messages: 'ok' | 'failed' | 'canceled' | '' */
  status?: 'ok' | 'failed' | 'canceled' | ''
}>()

const emit = defineEmits<{
  copy: [text: string]
}>()

const { t, locale } = useI18n()
const copied = ref(false)
const codeCopied = ref<Record<number, boolean>>({})

const html = computed(() => {
  // Track code block index for per-block "copy" affordances.
  let codeIdx = 0
  const renderer = new marked.Renderer()
  renderer.code = ({ text, lang }) => {
    const language = lang && hljs.getLanguage(lang) ? lang : 'plaintext'
    const highlighted = hljs.highlight(text, { language, ignoreIllegals: true }).value
    const idx = codeIdx++
    return `<div class="cb-code" data-cb-code="${idx}">
      <div class="cb-code__bar">
        <span class="cb-code__lang">${language}</span>
        <button type="button" class="cb-code__copy" data-cb-copy="${idx}">${t('chat.bubble.copy')}</button>
      </div>
      <pre class="cb-code__pre"><code class="hljs language-${language}">${highlighted}</code></pre>
    </div>`
  }

  if (props.role === 'user') {
    // Plain user content: no markdown, just preserve newlines.
    return `<p>${escapeHtml(props.content).replace(/\n/g, '<br>')}</p>`
  }
  try {
    // Model output is untrusted data: marked passes raw HTML through, so the
    // result must be sanitized before it reaches v-html (in the Wails WebView
    // injected script can reach the Go bindings).
    return DOMPurify.sanitize(marked.parse(props.content, { renderer, breaks: true }) as string)
  } catch {
    return `<p>${escapeHtml(props.content)}</p>`
  }
})

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

const timeLabel = computed(() => {
  if (!props.timestamp) return ''
  return new Intl.DateTimeFormat(locale.value, {
    hour: 'numeric',
    minute: '2-digit',
    hour12: true,
  }).format(new Date(props.timestamp))
})

async function copyContent(): Promise<void> {
  try {
    await navigator.clipboard.writeText(props.content)
    copied.value = true
    emit('copy', props.content)
    setTimeout(() => { copied.value = false }, 2000)
  } catch {
    // clipboard unavailable
  }
}

// Event-delegated handler for per-code-block copy buttons rendered into the
// assistant markdown output. The template root of this component is the
// assistant body container.
function onAssistantClick(e: MouseEvent): void {
  const target = (e.target as HTMLElement | null)?.closest('[data-cb-copy]')
  if (!target) return
  const idx = Number(target.getAttribute('data-cb-copy'))
  const wrapper = (target as HTMLElement).closest('.cb-code') as HTMLElement | null
  const code = wrapper?.querySelector('code')
  if (!code) return
  const text = code.textContent ?? ''
  void navigator.clipboard.writeText(text).then(() => {
    codeCopied.value = { ...codeCopied.value, [idx]: true }
    setTimeout(() => {
      const next = { ...codeCopied.value }
      delete next[idx]
      codeCopied.value = next
    }, 2000)
  }).catch(() => { /* noop */ })
}
</script>

<template>
  <article
    class="cb"
    :class="`cb--${role}`"
  >
    <!-- Assistant avatar / role label (only on assistant messages) -->
    <div v-if="role === 'assistant'" class="cb__head" aria-hidden="true">
      <span class="cb__avatar">
        <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
          <path d="M8 1.5l5.5 3v3.5c0 3.5-2.4 6.4-5.5 7-3.1-.6-5.5-3.5-5.5-7V4.5l5.5-3z" stroke="currentColor" stroke-width="1.2" stroke-linejoin="round"/>
          <circle cx="8" cy="6.5" r="1.4" fill="currentColor"/>
        </svg>
      </span>
      <span class="cb__role">{{ t('chat.bubble.roleAssistant') }}</span>
    </div>

    <!-- Body -->
    <div class="cb__body" @click="onAssistantClick">
      <!-- eslint-disable-next-line vue/no-v-html -->
      <div v-if="role === 'assistant'" class="cb__md" v-html="html" />
      <p v-else class="cb__plain">{{ content }}</p>
    </div>

    <!-- Helper line: time + status + copy (visible on hover) -->
    <div class="cb__meta">
      <span
        v-if="role === 'assistant' && status && status !== 'ok'"
        class="cb__status"
        :class="`cb__status--${status}`"
      >
        {{ t(`chat.status.${status}`) }}
      </span>
      <span v-if="timestamp" class="cb__time">{{ timeLabel }}</span>
      <button
        type="button"
        class="cb__copy"
        :aria-label="copied ? t('chat.bubble.copied') : t('chat.bubble.copy')"
        @click="copyContent"
      >
        <svg v-if="copied" width="12" height="12" viewBox="0 0 16 16" fill="none" aria-hidden="true">
          <path d="M3 8l3 3 7-7" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
        <svg v-else width="12" height="12" viewBox="0 0 16 16" fill="none" aria-hidden="true">
          <rect x="5" y="5" width="9" height="9" rx="1.5" stroke="currentColor" stroke-width="1.5"/>
          <path d="M11 5V3a1 1 0 0 0-1-1H3a1 1 0 0 0-1 1v7a1 1 0 0 0 1 1h2" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
        </svg>
        <span class="cb__copy-text">{{ copied ? t('chat.bubble.copied') : t('chat.bubble.copy') }}</span>
      </button>
    </div>
  </article>
</template>

<style scoped>
/* =========================================================
   Layout shell — no boxed card; rely on alignment + spacing
   ========================================================= */
.cb {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-width: 720px;
  width: 100%;
}

/* User aligns to the right edge of the column */
.cb--user {
  align-self: flex-end;
  align-items: flex-end;
}

/* Assistant aligns to the left */
.cb--assistant {
  align-self: flex-start;
  align-items: flex-start;
}

/* =========================================================
   Assistant head (avatar + role)
   ========================================================= */
.cb__head {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 2px;
}

.cb__avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: 6px;
  background: color-mix(in srgb, var(--dg-accent) 14%, transparent);
  color: var(--dg-accent-text);
}

.cb__role {
  color: var(--dg-text-tertiary);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.01em;
}

/* Body needs a stable basis so the inner pill can sit at the right edge
   and grow with content; without this, flex would let the user bubble
   collapse to a single character wide and wrap "hi" into two lines. */
.cb__body {
  display: flex;
  width: 100%;
  min-width: 0;
}

.cb--user .cb__body {
  justify-content: flex-end;
}

/* User message — iMessage-style filled pill */
.cb--user .cb__plain {
  margin: 0;
  padding: 9px 14px;
  border-radius: 18px;
  background: linear-gradient(
    135deg,
    color-mix(in srgb, var(--dg-accent) 22%, transparent),
    color-mix(in srgb, var(--dg-accent) 14%, transparent)
  );
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.08),
    0 1px 2px rgba(0, 0, 0, 0.08);
  color: var(--dg-text-primary);
  font-size: 14px;
  line-height: 1.5;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  word-break: break-word;
  max-width: 70%;
  width: fit-content;
}

/* =========================================================
   Assistant message — clean, borderless, document-like
   ========================================================= */
.cb--assistant .cb__body {
  width: 100%;
  min-width: 0;
}

.cb__md {
  color: var(--dg-text-primary);
  font-size: 14px;
  line-height: 1.65;
  word-break: break-word;
}

.cb__md :deep(p) {
  margin: 0 0 12px;
}

.cb__md :deep(p:last-child) {
  margin-bottom: 0;
}

.cb__md :deep(h1),
.cb__md :deep(h2),
.cb__md :deep(h3),
.cb__md :deep(h4) {
  margin: 18px 0 8px;
  color: var(--dg-text-primary);
  font-weight: 600;
  line-height: 1.3;
}

.cb__md :deep(h1) { font-size: 18px; }
.cb__md :deep(h2) { font-size: 16px; }
.cb__md :deep(h3) { font-size: 15px; }
.cb__md :deep(h4) { font-size: 14px; }

.cb__md :deep(strong) {
  font-weight: 600;
  color: var(--dg-text-primary);
}

.cb__md :deep(em) {
  font-style: italic;
}

.cb__md :deep(ul),
.cb__md :deep(ol) {
  margin: 0 0 12px;
  padding-left: 22px;
}

/* The global reset (styles/reset.css) strips list-style; restore markers here
   so assistant bullet and numbered lists still read as lists. */
.cb__md :deep(ul) { list-style: disc; }
.cb__md :deep(ul ul) { list-style: circle; }
.cb__md :deep(ol) { list-style: decimal; }

/* GFM task-list items carry their own checkbox — drop the redundant marker. */
.cb__md :deep(li:has(> input[type="checkbox"])) {
  list-style: none;
}

.cb__md :deep(li) {
  margin-bottom: 4px;
}

.cb__md :deep(li::marker) {
  color: var(--dg-text-muted);
}

.cb__md :deep(a) {
  color: var(--dg-accent-text);
  text-decoration: none;
  border-bottom: 1px solid color-mix(in srgb, var(--dg-accent) 28%, transparent);
  transition: border-color var(--dg-motion-fast) ease;
}

.cb__md :deep(a:hover) {
  border-bottom-color: var(--dg-accent);
}

.cb__md :deep(blockquote) {
  margin: 0 0 12px;
  padding: 8px 14px;
  border-left: 3px solid var(--dg-card-border);
  color: var(--dg-text-secondary);
}

.cb__md :deep(hr) {
  margin: 16px 0;
  border: none;
  border-top: 1px solid var(--dg-card-border);
}

.cb__md :deep(table) {
  width: 100%;
  margin: 12px 0;
  border-collapse: collapse;
  font-size: 13px;
}

.cb__md :deep(th),
.cb__md :deep(td) {
  padding: 8px 12px;
  border: 1px solid var(--dg-card-border);
  text-align: left;
}

.cb__md :deep(th) {
  background: var(--dg-track-fill);
  font-weight: 600;
}

/* inline code */
.cb__md :deep(code):not(.hljs) {
  padding: 1px 6px;
  border-radius: 5px;
  background: var(--dg-track-fill);
  color: var(--dg-accent-text);
  font-size: 13px;
  font-family: var(--dg-font-mono);
}

/* =========================================================
   Code block — terminal feel
   ========================================================= */
.cb__md :deep(.cb-code) {
  margin: 14px 0;
  border-radius: 10px;
  background: rgb(0 0 0 / 0.55);
  border: 1px solid rgb(255 255 255 / 0.06);
  overflow: hidden;
}

.cb__md :deep(.cb-code__bar) {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 6px 12px;
  background: rgb(255 255 255 / 0.03);
  border-bottom: 1px solid rgb(255 255 255 / 0.05);
}

.cb__md :deep(.cb-code__lang) {
  color: rgb(255 255 255 / 0.45);
  font-size: 10.5px;
  text-transform: lowercase;
  letter-spacing: 0.04em;
  font-family: var(--dg-font-mono);
}

.cb__md :deep(.cb-code__copy) {
  padding: 2px 8px;
  border: none;
  border-radius: 4px;
  background: rgb(255 255 255 / 0.06);
  color: rgb(255 255 255 / 0.7);
  font-size: 10.5px;
  cursor: pointer;
  transition: background var(--dg-motion-fast) ease, color var(--dg-motion-fast) ease;
}

.cb__md :deep(.cb-code__copy:hover) {
  background: rgb(255 255 255 / 0.12);
  color: rgb(255 255 255 / 0.95);
}

.cb__md :deep(.cb-code__pre) {
  margin: 0;
  padding: 12px 14px;
  overflow-x: auto;
  font-family: var(--dg-font-mono);
  font-size: 12.5px;
  line-height: 1.55;
}

.cb__md :deep(.cb-code__pre code) {
  background: none;
  padding: 0;
  color: rgb(232 232 232);
  font-family: var(--dg-font-mono);
}

/* highlight.js — neutral terminal */
.cb__md :deep(.hljs-keyword)  { color: #c678dd; }
.cb__md :deep(.hljs-string)    { color: #98c379; }
.cb__md :deep(.hljs-number)   { color: #d19a66; }
.cb__md :deep(.hljs-comment)  { color: #5c6370; font-style: italic; }
.cb__md :deep(.hljs-function) { color: #61afef; }
.cb__md :deep(.hljs-built_in) { color: #e5c07b; }
.cb__md :deep(.hljs-literal)  { color: #56b6c2; }
.cb__md :deep(.hljs-title)    { color: #61afef; }
.cb__md :deep(.hljs-attr)     { color: #e5c07b; }
.cb__md :deep(.hljs-tag)      { color: #e06c75; }
.cb__md :deep(.hljs-name)     { color: #e06c75; }
.cb__md :deep(.hljs-variable) { color: #e06c75; }

/* =========================================================
   Meta line — time + status + copy, only on hover
   ========================================================= */
.cb__meta {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 16px;
  padding: 0 4px;
  opacity: 0;
  transform: translateY(2px);
  transition: opacity var(--dg-motion-base) ease, transform var(--dg-motion-base) ease;
}

.cb:hover .cb__meta,
.cb:focus-within .cb__meta {
  opacity: 1;
  transform: translateY(0);
}

.cb--user .cb__meta {
  flex-direction: row-reverse;
}

.cb__status {
  padding: 1px 6px;
  border-radius: 4px;
  font-size: 10px;
  font-weight: 500;
}

.cb__status--failed {
  background: color-mix(in srgb, var(--dg-danger) 16%, transparent);
  color: var(--dg-danger-text);
}

.cb__status--canceled {
  background: var(--dg-track-fill);
  color: var(--dg-text-tertiary);
}

.cb__time {
  color: var(--dg-text-muted);
  font-size: 10.5px;
  letter-spacing: 0.01em;
}

.cb__copy {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 6px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--dg-text-tertiary);
  font-size: 10.5px;
  cursor: pointer;
  transition:
    background var(--dg-motion-fast) ease,
    color var(--dg-motion-fast) ease;
}

.cb__copy:hover {
  background: var(--dg-hover-fill);
  color: var(--dg-text-secondary);
}

/* =========================================================
   Entry animation
   ========================================================= */
@media (prefers-reduced-motion: no-preference) {
  .cb {
    animation: cb-in 220ms var(--dg-ease-glide) both;
  }
}

@keyframes cb-in {
  from {
    opacity: 0;
    transform: translateY(6px);
  }
}
</style>
