<script setup lang="ts">
import type { Component } from 'vue'
import { RouterLink, type RouteLocationRaw } from 'vue-router'

const props = defineProps<{
  to: RouteLocationRaw
  label: string
  icon: Component
  active: boolean
}>()
</script>

<template>
  <RouterLink
    class="rail-item"
    :class="{ 'is-active': props.active }"
    :to="props.to"
    :aria-current="props.active ? 'page' : undefined"
  >
    <span class="rail-item__glyph">
      <component :is="props.icon" class="rail-item__icon" />
    </span>
    <span class="rail-item__label">{{ props.label }}</span>
  </RouterLink>
</template>

<style scoped>
.rail-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 5px;
  width: 100%;
  height: var(--dg-rail-item-size);
  color: var(--dg-text-secondary);
  text-decoration: none;
}

.rail-item__glyph {
  display: grid;
  place-items: center;
  width: var(--dg-rail-selection-size);
  height: var(--dg-rail-selection-size);
  border: 1px solid transparent;
  border-radius: var(--dg-rail-selection-radius);
  transition:
    background 140ms ease,
    border-color 140ms ease;
}

.rail-item__icon {
  width: var(--dg-rail-icon-size);
  height: var(--dg-rail-icon-size);
}

.rail-item__label {
  font-size: var(--dg-rail-label-size);
  line-height: 1.1;
}

.rail-item:hover .rail-item__glyph {
  background: var(--dg-hover-fill);
}

.rail-item.is-active {
  color: var(--dg-text-primary);
}

.rail-item.is-active .rail-item__glyph {
  border-color: var(--dg-rail-selection-border);
  background: var(--dg-rail-selection-fill);
}

.rail-item:focus-visible {
  outline: 2px solid var(--dg-accent);
  outline-offset: 2px;
  border-radius: var(--dg-rail-selection-radius);
}
</style>
