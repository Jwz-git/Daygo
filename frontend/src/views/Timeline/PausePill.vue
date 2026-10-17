<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { useRecordingStore } from '@/stores/recording'

/*
 * The timeline's pause control, ported from Dayflow's PausePillView: a
 * Dynamic Island-style pill that morphs between three phases.
 *
 *   idle → menu    the pill springs open and the duration chips cascade in
 *                  from the right (50 ms apart)
 *   menu → idle    chips leave, the pill springs shut a beat later
 *   menu → paused  chips leave, the pill shrinks into an orange "Resume"
 *                  capsule and the pause status rises in beside it
 *   paused → idle  status leaves, the pill settles back with a livelier bounce
 *
 * Widths are measured from the rendered content rather than fixed, so the
 * spring targets fit every language. Hover / press scale the whole pill as
 * one composited layer (will-change), which keeps the label from shaking.
 */
const { t } = useI18n()
const recording = useRecordingStore()

type Phase = 'idle' | 'menu' | 'paused'
type Motion = 'open' | 'close' | 'toPaused' | 'toIdle' | 'none'

const CHIPS = [
  { minutes: 0, label: '∞', aria: 'recording.menuBar.pauseIndefinite', infinite: true },
  { minutes: 60, label: 'recording.menuBar.pause60', aria: 'recording.menuBar.pause60', infinite: false },
  { minutes: 30, label: 'recording.menuBar.pause30', aria: 'recording.menuBar.pause30', infinite: false },
  { minutes: 15, label: 'recording.menuBar.pause15', aria: 'recording.menuBar.pause15', infinite: false },
] as const

const PAD = 12
const CHIP_GAP = 8

const root = ref<HTMLElement | null>(null)
const primaryEl = ref<HTMLElement | null>(null)
const resumeEl = ref<HTMLElement | null>(null)
const chipsEl = ref<HTMLElement | null>(null)

const phase = ref<Phase>(recording.lifecycle === 'paused' ? 'paused' : 'idle')
const motion = ref<Motion>('none')
const primaryWidth = ref(46)
const resumeWidth = ref(58)
const chipsWidth = ref(176)

const statusShown = ref(false)
let statusTimer = 0

const now = ref(Date.now())
let clock = 0

const busy = computed(() => recording.pendingAction !== null)

const pillWidth = computed(() => {
  if (phase.value === 'paused') return PAD + resumeWidth.value + PAD
  if (phase.value === 'menu') return PAD + primaryWidth.value + CHIP_GAP + chipsWidth.value + PAD
  return PAD + primaryWidth.value + PAD
})

const remaining = computed(() => {
  const ends = recording.snapshot?.pauseEndsAtTs
  if (ends == null) return null
  return Math.max(0, Math.round(ends - now.value / 1000))
})

function clockText(seconds: number): string {
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = seconds % 60
  const mm = String(m).padStart(2, '0')
  const ss = String(s).padStart(2, '0')
  return h > 0 ? `${h}:${mm}:${ss}` : `${mm}:${ss}`
}

const statusText = computed(() =>
  remaining.value === null
    ? t('recording.pill.pausedIndefinitely')
    : t('recording.pill.pausedFor', { time: clockText(remaining.value) }),
)

function measure(): void {
  if (primaryEl.value) primaryWidth.value = Math.ceil(primaryEl.value.offsetWidth)
  if (resumeEl.value) resumeWidth.value = Math.ceil(resumeEl.value.offsetWidth)
  if (chipsEl.value) chipsWidth.value = Math.ceil(chipsEl.value.offsetWidth)
}

let observer: ResizeObserver | null = null

function presentStatus(autoHide: boolean): void {
  window.clearTimeout(statusTimer)
  statusShown.value = true
  if (autoHide) statusTimer = window.setTimeout(() => { statusShown.value = false }, 3350)
}

function hideStatus(): void {
  window.clearTimeout(statusTimer)
  statusShown.value = false
}

function toPaused(showStatus: boolean): void {
  motion.value = 'toPaused'
  phase.value = 'paused'
  if (showStatus) presentStatus(remaining.value === null)
}

function toIdle(): void {
  hideStatus()
  motion.value = 'toIdle'
  phase.value = 'idle'
}

function openMenu(): void {
  motion.value = 'open'
  phase.value = 'menu'
}

function closeMenu(): void {
  motion.value = 'close'
  phase.value = 'idle'
}

function onPillClick(): void {
  if (busy.value) return
  if (phase.value === 'idle') openMenu()
  else if (phase.value === 'menu') closeMenu()
  else void recording.perform('resume')
}

function pause(minutes: number): void {
  if (busy.value) return
  toPaused(true)
  void recording.perform('pause', minutes)
}

// External changes (menu bar, timer expiry, system resume) drive the pill too.
watch(() => recording.lifecycle, (lifecycle) => {
  if (lifecycle === 'paused' && phase.value !== 'paused') toPaused(true)
  else if (lifecycle === 'capturing' && phase.value === 'paused') toIdle()
})

watch(phase, (next) => {
  window.clearInterval(clock)
  if (next === 'paused') {
    now.value = Date.now()
    clock = window.setInterval(() => { now.value = Date.now() }, 1000)
  }
}, { immediate: true })

function onDocumentPointer(event: PointerEvent): void {
  if (phase.value === 'menu' && root.value && !root.value.contains(event.target as Node)) closeMenu()
}

function onKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape' && phase.value === 'menu') closeMenu()
}

onMounted(async () => {
  await nextTick()
  measure()
  observer = new ResizeObserver(measure)
  for (const el of [primaryEl.value, resumeEl.value, chipsEl.value]) if (el) observer.observe(el)
  document.addEventListener('pointerdown', onDocumentPointer)
  document.addEventListener('keydown', onKeydown)
  if (phase.value === 'paused') presentStatus(remaining.value === null)
})

onBeforeUnmount(() => {
  observer?.disconnect()
  window.clearTimeout(statusTimer)
  window.clearInterval(clock)
  document.removeEventListener('pointerdown', onDocumentPointer)
  document.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <div ref="root" class="pp" :class="[`is-${phase}`, `motion-${motion}`]">
    <Transition name="pp-status">
      <span v-if="statusShown && phase === 'paused'" class="pp__status" aria-live="polite">{{ statusText }}</span>
    </Transition>

    <div class="pp__pill" :style="{ width: `${pillWidth}px` }">
      <span class="pp__fill" aria-hidden="true"></span>
      <span class="pp__fill pp__fill--paused" aria-hidden="true"></span>
      <span class="pp__shine" aria-hidden="true"></span>

      <button
        type="button"
        class="pp__hit"
        :disabled="busy"
        :aria-expanded="phase === 'menu'"
        :aria-label="phase === 'paused' ? t('recording.action.resume') : t('recording.action.pause')"
        @click="onPillClick"
      ></button>

      <span ref="primaryEl" class="pp__content pp__content--primary" aria-hidden="true">
        <svg viewBox="0 0 12 12" width="12" height="12"><rect x="3" y="2.5" width="2" height="7" /><rect x="7" y="2.5" width="2" height="7" /></svg>
        {{ t('recording.action.pause') }}
      </span>
      <span ref="resumeEl" class="pp__content pp__content--resume" aria-hidden="true">
        <svg viewBox="0 0 12 12" width="12" height="12"><path d="M4.5 2.5v7L10 6Z" /></svg>
        {{ t('recording.action.resume') }}
      </span>

      <span ref="chipsEl" class="pp__chips" :style="{ left: `${PAD + primaryWidth + CHIP_GAP}px` }">
        <button
          v-for="(chip, index) in CHIPS"
          :key="chip.minutes"
          type="button"
          class="pp__chip"
          :class="{ 'is-infinite': chip.infinite }"
          :style="{ '--d': `${60 + (CHIPS.length - 1 - index) * 50}ms` }"
          :tabindex="phase === 'menu' ? 0 : -1"
          :disabled="busy"
          :aria-label="t(chip.aria)"
          @click.stop="pause(chip.minutes)"
        >{{ chip.infinite ? chip.label : t(chip.label) }}</button>
      </span>

      <span class="pp__border" aria-hidden="true"></span>
    </div>
  </div>
</template>

<style scoped>
/* Dayflow's glass control and paused capsule (DayflowTheme.swift), toned
   down: the reference #FFD2B9 fill and its #FF8D40 gradient read too deep
   and too bright beside Daygo's surfaces, so both sit lower in saturation
   and nearer Daygo's own accent (#ff9f6f). Kept local: no token carries
   these hues. */
.pp {
  --pp-fill: #fbe3d6;
  --pp-border: #ecdcd4;
  --pp-glow: #fff3ed;
  --pp-text: #606060;
  --pp-paused: linear-gradient(180deg, #f7bea4 0%, #ec9c74 50%, #e8946a 76%, #f0b294 100%);
  --pp-paused-border: #f6d6c4;
  --pp-paused-shine: rgba(255, 236, 222, 0.42);
  --pp-chip-border: rgba(225, 225, 225, 0.8);
  --pp-chip-fill: linear-gradient(320deg, rgba(255, 255, 255, 0.72) 0%, rgba(255, 255, 255, 0.56) 38%, rgba(255, 255, 255, 0.44) 100%);
  --pp-chip-text: #333333;
  /* Spring curves standing in for SwiftUI's .spring(duration:bounce:). */
  --pp-spring-soft: cubic-bezier(0.3, 1.18, 0.55, 1);
  --pp-spring: cubic-bezier(0.32, 1.3, 0.56, 1);
  --pp-spring-lively: cubic-bezier(0.34, 1.56, 0.64, 1);

  display: inline-flex;
  align-items: center;
  gap: 10px;
  height: 32px;
}

:root[data-dg-appearance='dark'] .pp {
  --pp-fill: rgba(226, 160, 121, 0.26);
  --pp-border: #6f5446;
  --pp-glow: rgba(243, 182, 152, 0.32);
  --pp-text: #ffffff;
  --pp-paused: linear-gradient(180deg, #c98a6c 0%, #b8704f 50%, #b26a49 76%, #c18468 100%);
  --pp-paused-border: #8f6450;
  --pp-paused-shine: rgba(255, 220, 196, 0.22);
  --pp-chip-border: rgba(136, 136, 136, 0.8);
  --pp-chip-fill: rgba(255, 255, 255, 0.15);
  --pp-chip-text: #ffffff;
}

/* ---- Status text beside the pill ---- */
.pp__status {
  color: var(--dg-accent);
  font-size: 12px;
  font-weight: 500;
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.03em;
  white-space: nowrap;
}

.pp-status-enter-active { transition: opacity 300ms ease, transform 300ms ease, filter 300ms ease; transition-delay: 350ms; }
.pp-status-leave-active { transition: opacity 200ms ease, transform 200ms ease, filter 200ms ease; }
.pp-status-enter-from,
.pp-status-leave-to { opacity: 0; transform: translateY(6px); filter: blur(6px); }

/* ---- Pill ---- */
.pp__pill {
  position: relative;
  height: 32px;
  overflow: hidden;
  border-radius: 999px;
  will-change: transform, width;
  backface-visibility: hidden;
  transition:
    width 500ms var(--pp-spring),
    transform 200ms ease;
}

.pp.motion-open .pp__pill { transition: width 520ms var(--pp-spring-soft), transform 200ms ease; }
.pp.motion-close .pp__pill { transition: width 450ms var(--pp-spring) 120ms, transform 200ms ease; }
.pp.motion-toPaused .pp__pill { transition: width 500ms var(--pp-spring), transform 200ms ease; }
.pp.motion-toIdle .pp__pill { transition: width 420ms var(--pp-spring-lively), transform 200ms ease; }

.pp.is-idle .pp__pill:hover { transform: scale(1.02); }
.pp:not(.is-menu) .pp__pill:active { transform: scale(0.97); }

.pp__fill,
.pp__shine,
.pp__border {
  position: absolute;
  inset: 0;
  border-radius: inherit;
  pointer-events: none;
}

.pp__fill {
  background: var(--pp-fill);
  box-shadow: inset 0 0 3px 1px var(--pp-glow);
  transition: opacity 350ms ease-in-out;
}

.pp__fill--paused {
  opacity: 0;
  background: var(--pp-paused);
  box-shadow: none;
}

/* The paused capsule's inner shine runs along the left and right edges only. */
.pp__shine {
  opacity: 0;
  box-shadow:
    inset 3px 0 5px var(--pp-paused-shine),
    inset -3px 0 5px var(--pp-paused-shine);
  transition: opacity 350ms ease-in-out;
}

.pp__border {
  box-shadow: inset 0 0 0 0.75px var(--pp-border);
  transition: box-shadow 350ms ease-in-out;
}

.pp.is-paused .pp__fill { opacity: 0; }
.pp.is-paused .pp__fill--paused,
.pp.is-paused .pp__shine { opacity: 1; }
.pp.is-paused .pp__border { box-shadow: inset 0 0 0 0.75px var(--pp-paused-border); }

.pp__hit {
  position: absolute;
  inset: 0;
  z-index: 1;
  padding: 0;
  border: none;
  border-radius: inherit;
  background: transparent;
  cursor: pointer;
}

.pp__hit:disabled { cursor: default; }
.pp__hit:focus-visible { outline: none; box-shadow: inset 0 0 0 2px var(--dg-focus-ring); }

/* ---- Content morph ---- */
.pp__content {
  position: absolute;
  top: 0;
  left: 12px;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  height: 32px;
  font-size: 12px;
  font-weight: 500;
  white-space: nowrap;
  pointer-events: none;
  transform-origin: left center;
}

.pp__content svg { flex: none; fill: currentColor; }

.pp__content--primary {
  color: var(--pp-text);
  transition: opacity 200ms ease, transform 200ms ease, filter 200ms ease;
}

.pp__content--resume {
  color: #ffffff;
  opacity: 0;
  transform: scale(0.8);
  filter: blur(5px);
  transition: opacity 180ms ease, transform 180ms ease, filter 180ms ease;
}

.pp.is-paused .pp__content--primary {
  opacity: 0;
  transform: scale(0.7);
  filter: blur(5px);
}

.pp.is-paused .pp__content--resume {
  opacity: 1;
  transform: scale(1);
  filter: blur(0);
  transition: opacity 350ms ease, transform 350ms var(--pp-spring), filter 350ms ease;
  transition-delay: 280ms;
}

/* Pause content returns once the capsule has mostly settled. */
.pp.motion-toIdle .pp__content--primary {
  transition: opacity 300ms ease 280ms, transform 300ms var(--pp-spring-lively) 280ms, filter 300ms ease 280ms;
}

/* ---- Duration chips ---- */
.pp__chips {
  position: absolute;
  top: 6px;
  z-index: 2;
  display: inline-flex;
  gap: 2px;
  pointer-events: none;
}

.pp.is-menu .pp__chips { pointer-events: auto; }

.pp__chip {
  position: relative;
  min-width: 42px;
  height: 20px;
  padding: 0 6px;
  overflow: hidden;
  border: none;
  border-radius: 999px;
  background: var(--pp-chip-fill);
  box-shadow: inset 0 0 0 1px var(--pp-chip-border);
  color: var(--pp-chip-text);
  font-size: 9px;
  font-weight: 600;
  white-space: nowrap;
  cursor: pointer;
  opacity: 0;
  transform: translateX(10px) scale(0.92);
  filter: blur(3px);
  will-change: transform;
  transition: opacity 200ms ease, transform 200ms ease, filter 200ms ease, color 150ms ease;
}

.pp__chip.is-infinite { font-size: 12px; font-weight: 500; }

/* Hover lays Dayflow's orange gradient over the chip. */
.pp__chip::before {
  position: absolute;
  inset: 0;
  z-index: -1;
  border-radius: inherit;
  background: linear-gradient(320deg, rgba(247, 190, 164, 0.9) 0%, rgba(236, 156, 116, 0.9) 42%, rgba(232, 148, 106, 0.9) 100%);
  content: '';
  opacity: 0;
  transition: opacity 150ms ease;
}

.pp__chip { isolation: isolate; }

.pp.is-menu .pp__chip {
  opacity: 1;
  transform: translateX(0) scale(1);
  filter: blur(0);
  transition:
    opacity 260ms ease var(--d),
    transform 320ms cubic-bezier(0.3, 1.35, 0.5, 1) var(--d),
    filter 260ms ease var(--d),
    color 150ms ease;
}

.pp.is-menu .pp__chip:hover:not(:disabled) { color: #ffffff; transform: scale(1.05); transition-delay: 0s; }
.pp.is-menu .pp__chip:hover:not(:disabled)::before { opacity: 1; }
.pp.is-menu .pp__chip:active:not(:disabled) { transform: scale(0.95); }
.pp__chip:focus-visible { outline: none; box-shadow: inset 0 0 0 1px var(--pp-chip-border), 0 0 0 2px var(--dg-focus-ring); }

@media (prefers-reduced-motion: reduce) {
  .pp__pill,
  .pp.motion-open .pp__pill,
  .pp.motion-close .pp__pill,
  .pp.motion-toPaused .pp__pill,
  .pp.motion-toIdle .pp__pill { transition: width 150ms ease; }
  .pp.is-idle .pp__pill:hover,
  .pp:not(.is-menu) .pp__pill:active { transform: none; }
  .pp__content,
  .pp__chip,
  .pp.is-menu .pp__chip,
  .pp.is-paused .pp__content--resume,
  .pp.motion-toIdle .pp__content--primary { transition: opacity 120ms ease; transform: none; filter: none; }
  .pp-status-enter-active,
  .pp-status-leave-active { transition: opacity 120ms ease; }
}
</style>
