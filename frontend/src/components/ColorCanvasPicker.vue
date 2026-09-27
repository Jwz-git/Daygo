<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'

import {
  MAX_LIGHT as MAX_LIGHT_CONST,
  MIN_LIGHT as MIN_LIGHT_CONST,
  paletteLightness,
  paletteSpread,
} from '@/lib/colorPalette'
import { hslToHex } from '@/lib/hsl'

/*
 * Draggable color canvas, a port of the reference implementation's
 * ColorPickerView + color-wheel renderer:
 *
 *   hue        = drag angle (atan2 around the center)
 *   lightness  = maxLight * (radius / RADIUS)
 *   spread     = (minSpread + (maxSpread - minSpread) * normalized^3) * spreadFactor
 *
 * The wheel pixels and the bullet colors use the same HSL formulas as the
 * original, so the canvas reads identically; the bullets show where the
 * derived palette sits (center bullet, then ±spread and ±2·spread).
 * Dragging anywhere in the circle — including the center bullet — moves the
 * palette. The parent only consumes the resulting angle and radius.
 */
const props = withDefaults(defineProps<{
  size?: number
  padding?: number
  bulletRadius?: number
  numPoints?: number
}>(), {
  size: 224,
  padding: 20,
  bulletRadius: 24,
  numPoints: 3,
})

const minLight = MIN_LIGHT_CONST
const maxLight = MAX_LIGHT_CONST

const emit = defineEmits<{
  change: [value: { angle: number; radius: number; normalizedRadius: number }]
}>()

const rootEl = ref<HTMLElement | null>(null)
const canvasEl = ref<HTMLCanvasElement | null>(null)

const angle = ref(-Math.PI / 2)
const radius = ref(0)

const radiusMax = computed(() => props.size / 2 - props.padding)
const normalizedRadius = computed(() => (radiusMax.value > 0 ? radius.value / radiusMax.value : 0))
const hue = computed(() => (angle.value * 180) / Math.PI)
const lightness = computed(() => paletteLightness(normalizedRadius.value))
const centerColor = computed(() => hslToHex(hue.value, 100, lightness.value))

const spread = computed(() => paletteSpread(normalizedRadius.value))

function colorAt(delta: number): string {
  return hslToHex(((angle.value + delta) * 180) / Math.PI, 100, lightness.value)
}

interface Bullet {
  key: string
  color: string
  diameter: number
  x: number
  y: number
  opacity: number
  primary: boolean
}

const bullets = computed<Bullet[]>(() => {
  const center = props.size / 2
  const position = (delta: number): { x: number; y: number } => ({
    x: center + Math.cos(angle.value + delta) * radius.value,
    y: center + Math.sin(angle.value + delta) * radius.value,
  })
  const out: Bullet[] = []
  const secondary = props.bulletRadius * 1.2
  const offset = -props.bulletRadius / 1.7 + secondary / 2

  if (props.numPoints >= 2) {
    const at = position(spread.value)
    out.push({
      key: 'plus', color: colorAt(spread.value), diameter: secondary,
      x: at.x + offset, y: at.y + offset, opacity: 0.9, primary: false,
    })
  }
  if (props.numPoints >= 3) {
    const at = position(-spread.value)
    out.push({
      key: 'minus', color: colorAt(-spread.value), diameter: secondary,
      x: at.x + offset, y: at.y + offset, opacity: 0.9, primary: false,
    })
  }
  if (props.numPoints >= 4) {
    const at = position(-spread.value * 2)
    out.push({
      key: 'minus2', color: colorAt(-spread.value * 2), diameter: props.bulletRadius,
      x: at.x, y: at.y, opacity: 0.8, primary: false,
    })
  }
  if (props.numPoints >= 5) {
    const at = position(spread.value * 2)
    out.push({
      key: 'plus2', color: colorAt(spread.value * 2), diameter: props.bulletRadius,
      x: at.x, y: at.y, opacity: 0.8, primary: false,
    })
  }

  const centerAt = position(0)
  out.push({
    key: 'center', color: centerColor.value, diameter: props.bulletRadius * 2,
    x: centerAt.x, y: centerAt.y, opacity: 1, primary: true,
  })
  return out
})

/*
 * The wheel bitmap: for every pixel inside the circle the hue comes from the
 * pixel's angle and the lightness ramps from minLight at the center to
 * maxLight at the rim; outside the circle stays transparent.
 */
function renderWheel(): void {
  const canvas = canvasEl.value
  if (canvas === null) return
  const scale = window.devicePixelRatio || 1
  const pixels = Math.round(props.size * scale)
  canvas.width = pixels
  canvas.height = pixels

  const context = canvas.getContext('2d')
  if (context === null) return
  const image = context.createImageData(pixels, pixels)
  const data = image.data

  const center = pixels / 2
  const outer = (props.size / 2 - props.padding) * scale
  const deltaLight = maxLight - minLight

  for (let y = 0; y < pixels; y += 1) {
    for (let x = 0; x < pixels; x += 1) {
      const offset = (y * pixels + x) * 4
      const dx = x - center
      const dy = y - center
      const distance = Math.sqrt(dx * dx + dy * dy)
      if (distance > outer) continue

      let pixelAngle = Math.atan2(dy, dx)
      if (pixelAngle < 0) pixelAngle += Math.PI * 2
      const pixelHue = (pixelAngle * 180) / Math.PI
      const pixelLight = minLight + deltaLight * (distance / outer)

      const hex = hslToHex(pixelHue, 100, pixelLight)
      data[offset] = Number.parseInt(hex.slice(1, 3), 16)
      data[offset + 1] = Number.parseInt(hex.slice(3, 5), 16)
      data[offset + 2] = Number.parseInt(hex.slice(5, 7), 16)
      data[offset + 3] = 255
    }
  }
  context.putImageData(image, 0, 0)
}

function emitChange(): void {
  emit('change', {
    angle: angle.value,
    radius: radius.value,
    normalizedRadius: normalizedRadius.value,
  })
}

function setFrom(clientX: number, clientY: number): void {
  const element = rootEl.value
  if (element === null) return
  const rect = element.getBoundingClientRect()
  const vx = clientX - (rect.left + rect.width / 2)
  const vy = clientY - (rect.top + rect.height / 2)
  let next = Math.atan2(vy, vx)
  if (next < 0) next += Math.PI * 2
  angle.value = next
  radius.value = Math.min(radiusMax.value, Math.max(0, Math.hypot(vx, vy)))
  emitChange()
}

function onPointerDown(event: PointerEvent): void {
  const element = event.currentTarget as HTMLElement | null
  if (element === null) return
  element.setPointerCapture(event.pointerId)
  setFrom(event.clientX, event.clientY)
}

function onPointerMove(event: PointerEvent): void {
  // Only track while the captured pointer is down (buttons held).
  if (event.buttons === 0) return
  setFrom(event.clientX, event.clientY)
}

onMounted(() => {
  radius.value = radiusMax.value * 0.7
  renderWheel()
  emitChange()
})

watch(() => [props.size, props.padding], renderWheel)
watch(() => props.numPoints, emitChange)
</script>

<template>
  <div
    ref="rootEl"
    class="color-canvas"
    :style="{ width: `${props.size}px`, height: `${props.size}px` }"
    @pointerdown="onPointerDown"
    @pointermove="onPointerMove"
  >
    <span class="color-canvas__dots" aria-hidden="true"></span>
    <canvas
      ref="canvasEl"
      class="color-canvas__wheel"
      :style="{ width: `${props.size}px`, height: `${props.size}px` }"
      aria-hidden="true"
    ></canvas>
    <span
      v-for="bullet in bullets"
      :key="bullet.key"
      class="color-canvas__bullet"
      :class="{ 'is-primary': bullet.primary }"
      :style="{
        width: `${bullet.diameter}px`,
        height: `${bullet.diameter}px`,
        background: bullet.color,
        opacity: bullet.opacity,
        left: `${bullet.x}px`,
        top: `${bullet.y}px`,
      }"
      aria-hidden="true"
    ></span>
  </div>
</template>

<style scoped>
.color-canvas {
  position: relative;
  border-radius: 16px;
  cursor: crosshair;
  touch-action: none;
}

/* Dot grid behind the wheel, faded out toward the corners. Kept on its own
   layer: masking the container would clip the wheel and the bullets too. */
.color-canvas__dots {
  position: absolute;
  inset: 0;
  border-radius: inherit;
  background-image: radial-gradient(circle, rgba(107, 114, 128, 0.22) 1px, transparent 1px);
  background-size: 10px 10px;
  -webkit-mask-image: radial-gradient(circle at center, #000 0%, transparent 100%);
  mask-image: radial-gradient(circle at center, #000 0%, transparent 100%);
  pointer-events: none;
}

.color-canvas__wheel {
  position: relative;
  display: block;
  border-radius: 50%;
}

.color-canvas__bullet {
  position: absolute;
  border-radius: 50%;
  box-shadow: 0 2px 4px rgba(15, 15, 25, 0.28);
  transform: translate(-50%, -50%);
  pointer-events: none;
}

.color-canvas__bullet.is-primary {
  border: 3px solid rgba(255, 255, 255, 0.9);
  box-shadow: 0 2px 8px rgba(15, 15, 25, 0.32);
}

.color-canvas__bullet:not(.is-primary) {
  border: 2px solid rgba(255, 255, 255, 0.8);
}
</style>
