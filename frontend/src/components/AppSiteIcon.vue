<script setup lang="ts">
import { computed, type CSSProperties } from 'vue'

import { resolveAppSiteIdentity } from '@/lib/appSiteIcon'

const props = withDefaults(defineProps<{
  site: string
  size?: number
  accent?: string
}>(), {
  size: 18,
  accent: 'var(--dg-accent)',
})

const identity = computed(() => resolveAppSiteIdentity(props.site))
const iconStyle = computed<CSSProperties>(() => ({
  width: `${props.size}px`,
  height: `${props.size}px`,
  '--app-site-accent': props.accent,
}))
</script>

<template>
  <span
    class="app-site-icon"
    :class="`app-site-icon--${identity.kind}`"
    :style="iconStyle"
    role="img"
    :aria-label="identity.label"
    :title="identity.label"
  >
    <svg v-if="identity.kind === 'daygo'" viewBox="0 0 24 24" aria-hidden="true">
      <rect x="6" y="12" width="2.8" height="6" rx="1.4" opacity=".68" />
      <rect x="10.6" y="5" width="2.8" height="13" rx="1.4" />
      <rect x="15.2" y="8.5" width="2.8" height="9.5" rx="1.4" opacity=".82" />
    </svg>

    <svg v-else-if="identity.kind === 'vscode'" viewBox="0 0 24 24" aria-hidden="true">
      <path d="M16.8 3.4 9.1 9.2 5.8 6.7 3.4 8.2l3.8 3.8-3.8 3.8 2.4 1.5 3.3-2.5 7.7 5.8 3.8-1.8V5.2l-3.8-1.8Zm0 4.6v8l-5.1-4 5.1-4Z" />
    </svg>

    <svg v-else-if="identity.kind === 'github'" viewBox="0 0 24 24" aria-hidden="true">
      <path d="M7 8.2 8.4 5l2.3 1.7c.4-.1.9-.1 1.3-.1s.9 0 1.3.1L15.6 5 17 8.2c1.1 1 1.7 2.4 1.7 4 0 4-2.5 6.7-6.7 6.7s-6.7-2.7-6.7-6.7c0-1.6.6-3 1.7-4Zm2.1 4.2c0 1.8 1.2 3.1 2.9 3.1s2.9-1.3 2.9-3.1c-.7.4-1.7.6-2.9.6s-2.2-.2-2.9-.6Z" />
    </svg>

    <svg v-else-if="identity.kind === 'chatgpt'" viewBox="0 0 24 24" aria-hidden="true">
      <g fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
        <path d="M12 4.1a4.2 4.2 0 0 1 6.2 3.6 4.2 4.2 0 0 1 1 7.1 4.2 4.2 0 0 1-6.2 5.1" />
        <path d="M12 19.9a4.2 4.2 0 0 1-6.2-3.6 4.2 4.2 0 0 1-1-7.1A4.2 4.2 0 0 1 11 4.1" />
        <path d="m8.3 8.5 7.4 7M15.7 8.5l-7.4 7" />
      </g>
    </svg>

    <span v-else-if="identity.kind === 'chrome'" class="chrome-mark" aria-hidden="true"><i></i></span>

    <svg v-else-if="identity.kind === 'safari'" viewBox="0 0 24 24" aria-hidden="true">
      <circle cx="12" cy="12" r="8.5" fill="none" stroke="currentColor" stroke-width="1.5" />
      <path class="safari-needle" d="m14.8 8.2-1.6 5-4 2.6 1.6-5 4-2.6Z" />
    </svg>

    <svg v-else-if="identity.kind === 'messages'" viewBox="0 0 24 24" aria-hidden="true">
      <path d="M4 11.3c0-4 3.5-7.1 8-7.1s8 3.1 8 7.1-3.5 7.1-8 7.1c-1 0-2-.2-2.9-.5L5.4 20l1-3.6A6.7 6.7 0 0 1 4 11.3Z" />
    </svg>

    <svg v-else-if="identity.kind === 'notes'" viewBox="0 0 24 24" aria-hidden="true">
      <rect x="4.5" y="3.5" width="15" height="17" rx="3" />
      <path d="M5 8h14" />
      <path class="notes-line" d="M8 12h8M8 15h6" />
    </svg>

    <svg v-else-if="identity.kind === 'terminal'" viewBox="0 0 24 24" aria-hidden="true">
      <rect x="3.5" y="4.5" width="17" height="15" rx="3" />
      <path d="m7.5 9 3 3-3 3M12.5 15h4" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
    </svg>

    <svg v-else-if="identity.kind === 'xcode'" viewBox="0 0 24 24" aria-hidden="true">
      <path d="m8 5.1 3.3 3.3-6.2 8.9a1.5 1.5 0 0 0 .4 2.1 1.5 1.5 0 0 0 2.1-.4l6.3-8.9 3.5 1.2 2-2.8-4.3-3.2-2.2 1.1-3.1-3.1L8 5.1Z" />
    </svg>

    <svg v-else-if="identity.kind === 'youtube'" viewBox="0 0 24 24" aria-hidden="true">
      <rect x="2.5" y="5.5" width="19" height="13" rx="4" />
      <path class="youtube-play" d="m10 9 5 3-5 3V9Z" />
    </svg>

    <svg v-else-if="identity.kind === 'google-docs'" viewBox="0 0 24 24" aria-hidden="true">
      <path d="M6 3h8l4 4v14H6V3Z" />
      <path class="docs-fold" d="M14 3v5h5" />
      <path class="docs-line" d="M9 12h6M9 15h6M9 18h4" />
    </svg>

    <span v-else-if="identity.kind === 'figma'" class="figma-mark" aria-hidden="true">
      <i></i><i></i><i></i><i></i><i></i>
    </span>

    <span v-else-if="identity.kind === 'slack'" class="slack-mark" aria-hidden="true">
      <i></i><i></i><i></i><i></i>
    </span>

    <svg v-else-if="identity.kind === 'discord'" viewBox="0 0 24 24" aria-hidden="true">
      <path d="M6.4 6.6A13 13 0 0 1 9.2 5l.7 1.3a9.7 9.7 0 0 1 4.2 0l.7-1.3a13 13 0 0 1 2.8 1.6c1.6 2.4 2.1 4.8 1.8 7.2a11 11 0 0 1-3.4 2.5l-.9-1.2c.6-.2 1.1-.5 1.6-.9-3 1.4-6.4 1.4-9.4 0 .5.4 1 .7 1.6.9L8 16.3a11 11 0 0 1-3.4-2.5c-.3-2.4.2-4.8 1.8-7.2Zm3.1 6.2c.7 0 1.2-.7 1.2-1.5s-.5-1.5-1.2-1.5-1.2.7-1.2 1.5.5 1.5 1.2 1.5Zm5 0c.7 0 1.2-.7 1.2-1.5s-.5-1.5-1.2-1.5-1.2.7-1.2 1.5.5 1.5 1.2 1.5Z" />
    </svg>

    <svg v-else-if="identity.kind === 'notion'" viewBox="0 0 24 24" aria-hidden="true">
      <rect x="3.5" y="3.5" width="17" height="17" rx="2" fill="none" stroke="currentColor" stroke-width="1.5" />
      <path d="M8 17V7.5h2.5l5 6.6V7.5H18V17h-2.4l-5.1-6.7V17H8Z" />
    </svg>

    <span v-else class="app-site-icon__monogram" aria-hidden="true">{{ identity.monogram }}</span>
  </span>
</template>

<style scoped>
.app-site-icon {
  position: relative;
  display: inline-grid;
  flex: none;
  overflow: hidden;
  border: 1px solid color-mix(in srgb, var(--dg-timeline-card-border) 86%, transparent);
  border-radius: 5px;
  background: color-mix(in srgb, var(--dg-panel-fill) 88%, var(--dg-track-fill));
  box-shadow: 0 1px 1px rgba(0, 0, 0, 0.08);
  color: var(--dg-text-primary);
  place-items: center;
}

.app-site-icon svg { width: 100%; height: 100%; }
.app-site-icon--daygo { background: #4b79a6; color: white; }
.app-site-icon--vscode { background: #2489ca; color: white; }
.app-site-icon--github { background: #25292e; color: white; }
.app-site-icon--chatgpt { background: #f7f7f5; color: #202123; }
.app-site-icon--messages { background: #43c95b; color: white; }
.app-site-icon--terminal { background: #202126; color: #f5f5f5; }
.app-site-icon--xcode { background: linear-gradient(145deg, #4fb5ef, #1677c8); color: white; }
.app-site-icon--youtube { background: #ff0033; color: white; }
.app-site-icon--google-docs { background: #4285f4; color: white; }
.app-site-icon--discord { background: #5865f2; color: white; }
.app-site-icon--notion { background: #fff; color: #111; }

.safari-needle { fill: #f25454; stroke: white; stroke-width: .7; }
.app-site-icon--safari { background: #3aa9e8; color: white; }

.app-site-icon--notes { background: #fff; color: #c6c8cc; }
.app-site-icon--notes rect { fill: #fff; stroke: #d8dadd; stroke-width: 1; }
.app-site-icon--notes path:first-of-type { stroke: #f3c842; stroke-width: 4.5; }
.app-site-icon--notes .notes-line { fill: none; stroke: #b9bdc3; stroke-width: 1.2; stroke-linecap: round; }

.youtube-play { fill: white; }
.docs-fold, .docs-line { fill: none; stroke: white; stroke-width: 1.35; stroke-linecap: round; stroke-linejoin: round; }

.chrome-mark {
  display: grid;
  width: 100%;
  height: 100%;
  background: conic-gradient(from 30deg, #e94235 0 33%, #fabb05 0 66%, #34a853 0);
  place-items: center;
}
.chrome-mark i { width: 42%; height: 42%; border: 1.5px solid white; border-radius: 50%; background: #4285f4; }

.figma-mark {
  display: grid;
  grid-template-columns: repeat(2, 42%);
  grid-template-rows: repeat(3, 29%);
  justify-content: center;
  align-content: center;
  width: 100%;
  height: 100%;
}
.figma-mark i { border-radius: 45%; }
.figma-mark i:nth-child(1) { background: #f24e1e; }
.figma-mark i:nth-child(2) { background: #ff7262; }
.figma-mark i:nth-child(3) { background: #a259ff; }
.figma-mark i:nth-child(4) { border-radius: 50%; background: #1abcfe; }
.figma-mark i:nth-child(5) { border-radius: 45% 45% 50% 50%; background: #0acf83; }

.slack-mark {
  position: relative;
  width: 76%;
  height: 76%;
}
.slack-mark i { position: absolute; width: 38%; height: 12%; border-radius: 99px; }
.slack-mark i:nth-child(1) { top: 18%; left: 8%; background: #36c5f0; transform: rotate(90deg); }
.slack-mark i:nth-child(2) { top: 18%; right: 8%; background: #2eb67d; }
.slack-mark i:nth-child(3) { right: 8%; bottom: 18%; background: #ecb22e; transform: rotate(90deg); }
.slack-mark i:nth-child(4) { bottom: 18%; left: 8%; background: #e01e5a; }

.app-site-icon--generic {
  background: color-mix(in srgb, var(--app-site-accent) 14%, var(--dg-panel-fill));
  color: color-mix(in srgb, var(--app-site-accent) 72%, var(--dg-text-primary));
}
.app-site-icon__monogram { font-size: 46%; font-weight: 720; letter-spacing: -.02em; line-height: 1; }
</style>
