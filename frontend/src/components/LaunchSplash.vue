<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

import daygoIcon from '@/assets/favicons/daygo.png'

/*
 * Launch animation, after Dayflow's splash. The icon settles in on a ring of
 * warm light; the wordmark — Instrument Serif italic — writes itself in
 * letter by letter, each rising out of a soft blur; a band of light then
 * sweeps across it and a hairline draws out underneath; finally the sheet
 * fades to the app (or the first-run guide) already painted beneath.
 *
 * It runs once per page load (the resident agent hides and shows the same
 * webview, so reopening the window does not replay it), stops taking input
 * once it starts leaving, and shortens to a plain fade under reduced motion.
 */
const emit = defineEmits<{ done: [] }>()

const WORD = 'Daygo'
const letters = [...WORD]

const leaving = ref(false)
const timers: number[] = []

onMounted(() => {
  const reduced = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false
  const hold = reduced ? 250 : 1500
  const fade = reduced ? 150 : 420
  timers.push(window.setTimeout(() => { leaving.value = true }, hold))
  timers.push(window.setTimeout(() => emit('done'), hold + fade))
})

onBeforeUnmount(() => timers.forEach((timer) => window.clearTimeout(timer)))
</script>

<template>
  <div class="splash" :class="{ 'is-leaving': leaving }" aria-hidden="true">
    <div class="splash__mark">
      <span class="splash__ring"></span>
      <img class="splash__icon" :src="daygoIcon" alt="" width="84" height="84" />
    </div>

    <div class="splash__word">
      <span class="splash__letters">
        <span
          v-for="(letter, index) in letters"
          :key="index"
          class="splash__letter"
          :style="{ '--i': index }"
        >{{ letter }}</span>
      </span>
      <!-- The same word again, clipped to a moving band of light. -->
      <span class="splash__sheen">{{ WORD }}</span>
      <span class="splash__rule"></span>
    </div>
  </div>
</template>

<style scoped>
.splash {
  position: fixed;
  inset: 0;
  z-index: 1000;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 14px;
  background-color: var(--dg-window-bg);
  background-image: var(--dg-window-gradient);
  transition: opacity 420ms ease;
  --wails-draggable: drag;
}

.splash.is-leaving {
  opacity: 0;
  pointer-events: none;
}

.splash__mark {
  position: relative;
  display: grid;
  width: 84px;
  height: 84px;
  place-items: center;
  transition: transform 420ms ease;
}

.splash.is-leaving .splash__mark { transform: translateY(-4px); }

.splash__icon {
  position: relative;
  width: 84px;
  height: 84px;
  filter: drop-shadow(0 12px 26px rgba(99, 71, 56, 0.28));
  animation: splash-icon 640ms cubic-bezier(0.2, 1.25, 0.4, 1) both;
}

/* A ring of warm light that blooms out from behind the icon once. */
.splash__ring {
  position: absolute;
  inset: -6px;
  border-radius: 50%;
  background: radial-gradient(circle, color-mix(in srgb, var(--dg-accent) 38%, transparent) 0%, transparent 70%);
  animation: splash-ring 900ms 160ms ease-out both;
}

/* ---- Wordmark ---- */
.splash__word {
  position: relative;
  padding: 0 6px 18px;
  font-family: 'Instrument Serif', var(--dg-font-serif);
  font-size: 56px;
  font-style: italic;
  font-weight: 400;
  letter-spacing: -0.01em;
  line-height: 1;
  /* Per-letter spans cannot kern, so the sheen copy must not either, or the
     two layers drift apart and the sweep ghosts. */
  font-kerning: none;
}

.splash__letters {
  display: inline-flex;
  /* Warm ink: deep brown at the stem, easing into the accent at the tail. */
  color: var(--dg-text-primary);
}

.splash__letter {
  display: inline-block;
  /* background-clip: text paints only inside the box, and at line-height 1 the
     descenders of y and g and the italic overhang sit outside it. Pad the box
     out to the full ink and pull the neighbours back so spacing is unchanged. */
  padding: 0.12em 0.08em 0.26em;
  margin: -0.12em -0.08em -0.26em;
  background: linear-gradient(
    180deg,
    var(--dg-text-primary) 35%,
    color-mix(in srgb, var(--dg-accent) 55%, var(--dg-text-primary)) 115%
  );
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
  animation: splash-letter 640ms cubic-bezier(0.22, 1, 0.36, 1) both;
  animation-delay: calc(280ms + var(--i) * 75ms);
}

/* Overlay copy of the word, painted only where the light band passes. */
.splash__sheen {
  position: absolute;
  top: -0.12em;
  left: 6px;
  /* Same ink padding as the letters, so the sweep reaches the descenders. */
  padding: 0.12em 0.08em 0.26em;
  margin-left: -0.08em;
  background: linear-gradient(
    100deg,
    transparent 30%,
    rgba(255, 244, 232, 0.95) 48%,
    color-mix(in srgb, var(--dg-accent) 70%, #ffffff) 52%,
    transparent 70%
  );
  background-size: 260% 100%;
  background-position: 120% 0;
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
  pointer-events: none;
  animation: splash-sheen 760ms 860ms ease-in-out both;
}

/* Hairline that draws outward from the centre under the word. */
.splash__rule {
  position: absolute;
  right: 22%;
  bottom: 0;
  left: 22%;
  height: 1px;
  background: linear-gradient(90deg, transparent, color-mix(in srgb, var(--dg-accent) 70%, transparent), transparent);
  transform: scaleX(0);
  animation: splash-rule 620ms 820ms cubic-bezier(0.22, 1, 0.36, 1) both;
}

@keyframes splash-icon {
  from { opacity: 0; transform: scale(0.78) translateY(6px); }
  to { opacity: 1; transform: scale(1) translateY(0); }
}

@keyframes splash-ring {
  0% { opacity: 0; transform: scale(0.6); }
  35% { opacity: 1; }
  100% { opacity: 0; transform: scale(2.1); }
}

@keyframes splash-letter {
  from { opacity: 0; filter: blur(8px); transform: translateY(16px) rotate(4deg); }
  60% { filter: blur(0); }
  to { opacity: 1; filter: blur(0); transform: translateY(0) rotate(0); }
}

@keyframes splash-sheen {
  from { background-position: 120% 0; opacity: 1; }
  to { background-position: -20% 0; opacity: 1; }
}

@keyframes splash-rule {
  from { opacity: 0; transform: scaleX(0); }
  to { opacity: 1; transform: scaleX(1); }
}

@media (prefers-reduced-motion: reduce) {
  .splash { transition-duration: 150ms; }
  .splash__icon,
  .splash__ring,
  .splash__letter,
  .splash__sheen,
  .splash__rule { animation: none; }
  .splash__ring,
  .splash__sheen { display: none; }
  .splash__rule { transform: none; }
  .splash.is-leaving .splash__mark { transform: none; }
}
</style>
