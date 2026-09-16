<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import type { CardMediaFrameDTO } from '@/api/dto'

/*
 * Card playback over the frames the recorder actually stored — discrete
 * screenshots, no video codec (the encoding decision is still pending).
 * Playback advances a virtual clock at `rate` × real time and shows the last
 * frame captured before it; the scrubber maps to that clock. Hovering reveals
 * the rate badge (cycles 20x → 40x → 60x → 120x); the expand button opens the
 * same playback in a full-window overlay with a ticked scrubber.
 *
 * Frames arrive as numeric IDs and render through /media/frame?id=, which the
 * backend resolves inside the recordings root only.
 */
const props = defineProps<{
  frames: CardMediaFrameDTO[]
  title: string
  timeLabel: string
  timeZone: string
}>()

const { t, locale } = useI18n()

const RATES = [20, 40, 60, 120] as const

const rateIndex = ref(1)
const playing = ref(false)
const progress = ref(0)
const frameIndex = ref(0)
const expanded = ref(false)
const hovered = ref(false)

const rate = computed(() => RATES[rateIndex.value])
const span = computed(() => {
  if (props.frames.length < 2) return 0
  return Math.max(1, props.frames[props.frames.length - 1]!.capturedAt - props.frames[0]!.capturedAt)
})
const currentFrame = computed(() => props.frames[frameIndex.value] ?? null)

const frameSrc = (id: number): string => `/media/frame?id=${id}`

function cycleRate(): void {
  rateIndex.value = (rateIndex.value + 1) % RATES.length
}

function frameAt(virtualTs: number): number {
  const frames = props.frames
  let low = 0
  let high = frames.length - 1
  let found = 0
  while (low <= high) {
    const mid = (low + high) >> 1
    if (frames[mid]!.capturedAt <= virtualTs) {
      found = mid
      low = mid + 1
    } else {
      high = mid - 1
    }
  }
  return found
}

function setProgress(virtualTs: number): void {
  if (span.value <= 0) {
    progress.value = 0
    frameIndex.value = 0
    return
  }
  progress.value = (virtualTs / span.value) * 100
  frameIndex.value = frameAt(clockStart.value + virtualTs)
}

function seekTo(fraction: number): void {
  const clamped = Math.min(1, Math.max(0, fraction))
  virtualTs = clockStart.value + clamped * span.value
  setProgress(clamped * span.value)
}

/* Scrubbing: pointer capture makes press-and-drag one continuous gesture on
   both the inline bar and the lightbox filmstrip. */
function startScrub(event: PointerEvent): void {
  const bar = event.currentTarget as HTMLElement | null
  if (bar === null || span.value <= 0) return
  bar.setPointerCapture(event.pointerId)
  const seekFrom = (clientX: number): void => {
    const rect = bar.getBoundingClientRect()
    seekTo((clientX - rect.left) / rect.width)
  }
  seekFrom(event.clientX)
  const onMove = (move: PointerEvent): void => { seekFrom(move.clientX) }
  const onUp = (): void => {
    bar.removeEventListener('pointermove', onMove)
    bar.removeEventListener('pointerup', onUp)
    bar.removeEventListener('pointercancel', onUp)
  }
  bar.addEventListener('pointermove', onMove)
  bar.addEventListener('pointerup', onUp)
  bar.addEventListener('pointercancel', onUp)
}

/* Playback loop: one requestAnimationFrame drives both the inline player and
   the lightbox copy; the virtual clock, not the <img>, is the state. */
const clockStart = computed(() => props.frames[0]?.capturedAt ?? 0)
let virtualTs = clockStart.value
let lastTick = 0
let rafID: number | null = null

function tick(now: number): void {
  rafID = null
  if (lastTick !== 0) {
    virtualTs += ((now - lastTick) / 1000) * rate.value
    if (virtualTs >= clockStart.value + span.value) {
      virtualTs = clockStart.value + span.value
      playing.value = false
    }
    setProgress(virtualTs - clockStart.value)
  }
  lastTick = playing.value ? now : 0
  if (playing.value) rafID = requestAnimationFrame(tick)
}

function togglePlay(): void {
  if (props.frames.length === 0) return
  playing.value = !playing.value
  if (playing.value) {
    if (virtualTs >= clockStart.value + span.value) virtualTs = clockStart.value
    lastTick = 0
    rafID = requestAnimationFrame(tick)
  }
}

watch(playing, (value) => {
  if (!value && rafID !== null) {
    cancelAnimationFrame(rafID)
    rafID = null
  }
})

// A new card means a fresh clock.
watch(() => props.frames, () => {
  playing.value = false
  virtualTs = clockStart.value
  setProgress(0)
})

function openExpanded(): void {
  expanded.value = true
}

const clockLabel = (ts: number): string =>
  new Intl.DateTimeFormat(locale.value, { hour: '2-digit', minute: '2-digit' }).format(new Date(ts * 1000))

const currentClock = computed(() =>
  currentFrame.value === null ? '' : clockLabel(currentFrame.value.capturedAt),
)

/* Warm the next few frames so stepping stays instant. */
watch(frameIndex, (index) => {
  for (let offset = 1; offset <= 3; offset += 1) {
    const next = props.frames[index + offset]
    if (next !== undefined) {
      const image = new Image()
      image.src = frameSrc(next.id)
    }
  }
})

/* Evenly sampled thumbnails for the lightbox filmstrip. */
const STRIP_COUNT = 14
const stripFrames = computed(() => {
  const frames = props.frames
  if (frames.length <= STRIP_COUNT) return frames
  const stride = frames.length / STRIP_COUNT
  return Array.from({ length: STRIP_COUNT }, (_, index) =>
    frames[Math.min(frames.length - 1, Math.floor(index * stride))]!,
  )
})

function onKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape') expanded.value = false
}

watch(expanded, (value) => {
  if (value) document.addEventListener('keydown', onKeydown)
  else document.removeEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKeydown)
  if (rafID !== null) cancelAnimationFrame(rafID)
})
</script>

<template>
  <div class="player" :class="{ 'is-hovered': hovered, 'is-empty': frames.length === 0 }" @mouseenter="hovered = true" @mouseleave="hovered = false">
    <template v-if="frames.length > 0">
      <img
        class="player__frame"
        :src="frameSrc(currentFrame!.id)"
        :alt="title"
        draggable="false"
        @click="togglePlay"
      >

      <span v-if="currentClock !== ''" class="player__stage-clock">{{ currentClock }}</span>

      <button
        v-if="!playing"
        type="button"
        class="player__big-play"
        :aria-label="t('timeline.player.play')"
        @click="togglePlay"
      >
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M8 5.4v13.2L19 12Z" fill="currentColor" /></svg>
      </button>

      <button
        type="button"
        class="player__expand"
        :aria-label="t('timeline.player.expand')"
        @click="openExpanded"
      >
        <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M6 2H2v4M10 14h4v-4M2 10v4h4M14 6V2h-4" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" /></svg>
      </button>

      <button type="button" class="player__rate" :aria-label="t('timeline.player.rate')" @click="cycleRate">
        {{ rate }}x
      </button>

      <div class="player__bar" role="slider" :aria-valuenow="Math.round(progress)" aria-valuemin="0" aria-valuemax="100" @pointerdown="startScrub">
        <span class="player__bar-fill" :style="{ width: `${progress}%` }"></span>
      </div>
    </template>

    <div v-else class="player__placeholder" aria-hidden="true">
      <svg viewBox="0 0 24 24"><path d="M8 5.4v13.2L19 12Z" fill="currentColor" /></svg>
    </div>
  </div>

  <Teleport to="body">
    <Transition name="drop">
      <div v-if="expanded && frames.length > 0" class="lightbox" @click.self="expanded = false">
      <div class="lightbox__panel" role="dialog" :aria-label="title">
        <header class="lightbox__head">
          <div class="lightbox__meta">
            <h2>{{ title }}</h2>
            <span>{{ timeLabel }}</span>
          </div>
          <button type="button" class="lightbox__close" :aria-label="t('timeline.player.close')" @click="expanded = false">×</button>
        </header>

        <div class="lightbox__stage" @click="togglePlay">
          <img class="lightbox__frame" :src="frameSrc(currentFrame!.id)" :alt="title" draggable="false">
          <button
            v-if="!playing"
            type="button"
            class="player__big-play"
            :aria-label="t('timeline.player.play')"
            @click.stop="togglePlay"
          >
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M8 5.4v13.2L19 12Z" fill="currentColor" /></svg>
          </button>
          <span class="lightbox__clock">{{ currentClock }}</span>
          <button
            type="button"
            class="player__rate player__rate--lightbox"
            :aria-label="t('timeline.player.rate')"
            @click.stop="cycleRate"
          >{{ rate }}x</button>
        </div>

        <div class="lightbox__scrub">
          <div
            class="filmstrip"
            role="slider"
            :aria-valuenow="Math.round(progress)"
            aria-valuemin="0"
            aria-valuemax="100"
            @pointerdown="startScrub"
          >
            <img
              v-for="frame in stripFrames"
              :key="frame.id"
              class="filmstrip__thumb"
              :src="frameSrc(frame.id)"
              alt=""
              loading="lazy"
              draggable="false"
            >
            <span class="filmstrip__played" :style="{ width: `${progress}%` }" aria-hidden="true"></span>
            <span class="filmstrip__cursor" :style="{ left: `${progress}%` }" aria-hidden="true"></span>
          </div>
          <div class="lightbox__controls">
            <button type="button" class="lightbox__play" :aria-label="playing ? t('timeline.player.pause') : t('timeline.player.play')" @click="togglePlay">
              <svg v-if="!playing" viewBox="0 0 24 24" aria-hidden="true"><path d="M8 5.4v13.2L19 12Z" fill="currentColor" /></svg>
              <svg v-else viewBox="0 0 24 24" aria-hidden="true"><rect x="6" y="5" width="4" height="14" rx="1.2" fill="currentColor" /><rect x="14" y="5" width="4" height="14" rx="1.2" fill="currentColor" /></svg>
            </button>
            <span class="lightbox__clock-chip">{{ currentClock }}</span>
          </div>
        </div>
      </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.player {
  position: relative;
  overflow: hidden;
  border-radius: 10px;
  background: #0d0d12;
  /* The player is a control surface, not text: no accidental selections or
     native image drags stealing the gesture. */
  user-select: none;
  -webkit-user-select: none;
  transition:
    transform 220ms var(--dg-ease-glide),
    box-shadow 220ms ease;
}

/* Hover lift-zoom on every inline instance of the player. */
.player:hover {
  transform: scale(1.02);
  box-shadow: 0 14px 34px rgba(15, 15, 25, 0.3);
}

.player__frame,
.player__placeholder {
  -webkit-user-drag: none;
  display: block;
  width: 100%;
  max-height: 240px;
  object-fit: cover;
}

.player__placeholder {
  display: grid;
  height: 170px;
  place-items: center;
  background: linear-gradient(120deg, color-mix(in srgb, var(--dg-accent) 24%, #0d0d12), #0d0d12);
}

.player__placeholder svg { width: 40px; height: 40px; color: rgba(255, 255, 255, 0.7); }

.player__big-play {
  position: absolute;
  top: calc(50% - 7px);
  left: 50%;
  display: grid;
  width: 60px;
  height: 60px;
  place-items: center;
  border: none;
  border-radius: 50%;
  background: rgba(15, 15, 20, 0.55);
  color: #ffffff;
  transform: translate(-50%, -50%);
  cursor: pointer;
  backdrop-filter: blur(6px);
  transition: transform var(--dg-motion-fast) var(--dg-ease-out), background var(--dg-motion-fast) ease;
}

/* Orange clock chip on the stage's bottom-left, like the reference card. */
.player__stage-clock {
  position: absolute;
  bottom: 18px;
  left: 10px;
  padding: 3px 10px;
  border-radius: 999px;
  background: #e8804a;
  color: #ffffff;
  font-size: 12px;
  font-weight: 650;
  font-variant-numeric: tabular-nums;
  pointer-events: none;
}

.player__big-play:hover { background: rgba(15, 15, 20, 0.72); transform: translate(-50%, -50%) scale(1.05); }
.player__big-play:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }
.player__big-play svg { width: 28px; height: 28px; }

.player__expand {
  position: absolute;
  top: 8px;
  right: 8px;
  display: grid;
  width: 28px;
  height: 28px;
  place-items: center;
  border: none;
  border-radius: 7px;
  background: rgba(15, 15, 20, 0.5);
  color: #ffffff;
  opacity: 0;
  cursor: pointer;
  transition: opacity var(--dg-motion-fast) ease;
}

.player.is-hovered .player__expand,
.player__expand:focus-visible { opacity: 1; }
.player__expand svg { width: 13px; height: 13px; }

.player__rate {
  position: absolute;
  right: 10px;
  bottom: 24px;
  padding: 4px 9px;
  border: none;
  border-radius: 8px;
  background: rgba(15, 15, 20, 0.62);
  color: #ffffff;
  font-size: 12px;
  font-weight: 650;
  font-variant-numeric: tabular-nums;
  opacity: 0;
  cursor: pointer;
  transition: opacity var(--dg-motion-fast) ease;
}

.player.is-hovered .player__rate,
.player__rate:focus-visible,
.player__rate--visible { opacity: 1; }

.player__bar {
  position: relative;
  height: 6px;
  border-radius: 999px;
  cursor: pointer;
  background: rgba(255, 255, 255, 0.14);
}

.player__bar-fill {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 0;
  border-radius: inherit;
  background: var(--dg-accent);
}

/* Expanded lightbox */
.lightbox {
  position: fixed;
  inset: 0;
  z-index: 120;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px;
  background: rgba(12, 12, 16, 0.55);
  backdrop-filter: blur(10px);
}

.lightbox__panel {
  display: flex;
  flex-direction: column;
  width: min(1120px, 100%);
  max-height: 100%;
  overflow: hidden;
  border-radius: 16px;
  background: var(--dg-popover-fill, var(--dg-surface));
  box-shadow: var(--lg-shadow-dense, var(--dg-shadow-lg));
}

.lightbox__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 20px 12px;
}

.lightbox__meta { min-width: 0; }
.lightbox__meta h2 {
  margin: 0 0 3px;
  overflow: hidden;
  color: var(--dg-text-primary);
  font-size: 16px;
  font-weight: 650;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.lightbox__meta span { color: var(--dg-text-secondary); font-size: 12px; }

.lightbox__close {
  display: grid;
  flex: none;
  width: 30px;
  height: 30px;
  place-items: center;
  border: none;
  border-radius: 50%;
  background: var(--dg-hover-fill);
  color: var(--dg-text-secondary);
  font-size: 18px;
  cursor: pointer;
}

.lightbox__close:hover { background: var(--dg-hover-fill-strong); }

.lightbox__stage {
  position: relative;
  margin: 0 20px;
  overflow: hidden;
  border-radius: 10px;
  background: #0d0d12;
  cursor: pointer;
}

.lightbox__frame {
  -webkit-user-drag: none;
  display: block;
  width: 100%;
  max-height: 62vh;
  object-fit: contain;
}

.lightbox__clock {
  position: absolute;
  bottom: 10px;
  left: 50%;
  padding: 3px 10px;
  border-radius: 999px;
  background: rgba(15, 15, 20, 0.62);
  color: #ffffff;
  font-size: 11px;
  font-variant-numeric: tabular-nums;
  transform: translateX(-50%);
}

.lightbox__scrub {
  padding: 12px 20px 16px;
}

.filmstrip {
  position: relative;
  display: flex;
  gap: 2px;
  overflow: hidden;
  border-radius: 8px;
  cursor: pointer;
}

.filmstrip__thumb {
  -webkit-user-drag: none;
  display: block;
  flex: 1 1 0;
  min-width: 0;
  height: 48px;
  object-fit: cover;
  background: #0d0d12;
}

.filmstrip__played {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 0;
  background: color-mix(in srgb, var(--dg-accent) 22%, transparent);
  pointer-events: none;
}

.filmstrip__cursor {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 2px;
  background: #ffffff;
  box-shadow: 0 0 6px rgba(0, 0, 0, 0.6);
  pointer-events: none;
}

.lightbox__controls {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 10px;
}

/* The rate badge sits on the video's bottom-right corner, same as the inline
   player; hover on the stage keeps it revealed. */
.player__rate--lightbox {
  right: 10px;
  bottom: 12px;
  opacity: 1;
}

.lightbox__stage:hover .player__rate--lightbox,
.player__rate--lightbox:focus-visible { opacity: 1; }

.lightbox__play {
  display: grid;
  width: 34px;
  height: 34px;
  place-items: center;
  border: none;
  border-radius: 50%;
  background: var(--dg-accent);
  color: #ffffff;
  cursor: pointer;
  transition: transform var(--dg-motion-fast) var(--dg-ease-out);
}

.lightbox__play:hover { transform: scale(1.06); }
.lightbox__play:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--dg-focus-ring); }
.lightbox__play svg { width: 16px; height: 16px; }

.lightbox__clock-chip {
  margin-left: auto;
  padding: 3px 10px;
  border-radius: 999px;
  background: var(--dg-track-fill);
  color: var(--dg-text-primary);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}

/* Lightbox entrance: drops in from above while the scrim fades. */
.drop-enter-active {
  transition: opacity 220ms ease;
}

.drop-leave-active {
  transition: opacity 170ms ease;
}

.drop-enter-active .lightbox__panel {
  transition: transform 300ms var(--dg-ease-glide);
}

.drop-leave-active .lightbox__panel {
  transition: transform 200ms ease;
}

.drop-enter-from {
  opacity: 0;
}

.drop-enter-from .lightbox__panel {
  transform: translateY(-56px) scale(0.985);
}

.drop-leave-to {
  opacity: 0;
}

.drop-leave-to .lightbox__panel {
  transform: translateY(-28px);
}

</style>
