<script setup lang="ts">
const props = defineProps<{ title: string }>()
</script>

<template>
  <header class="page-header">
    <div class="page-header__lead">
      <h1 class="page-header__title dg-display">{{ props.title }}</h1>
      <slot name="lead" />
    </div>
    <div class="page-header__trail">
      <slot name="trail" />
    </div>
  </header>
</template>

<style scoped>
/*
 * The header is a window-drag surface so users can grab the window from the
 * top edge without the titlebar being visible. Anything interactive in the
 * header (lead / trail slots, status badges) must opt out with no-drag,
 * otherwise Wails swallows the click and the control does nothing.
 *
 * Slotted wrappers like <PeriodNav> set their own root no-drag, and that
 * declaration inherits into their internal buttons. The :slotted(*) rule
 * below is a belt-and-braces opt-out: any element the caller slots in is
 * no-drag by default, so a forgotten wrapper cannot accidentally turn the
 * whole header into a dead zone for clicks.
 */
.page-header {
  position: relative;
  z-index: 3;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  padding: 26px var(--dg-page-padding) 18px;
  -webkit-app-region: drag;
  --wails-draggable: drag;
}

.page-header :slotted(*) {
  -webkit-app-region: no-drag;
  --wails-draggable: no-drag;
}

.page-header__lead,
.page-header__trail {
  display: flex;
  align-items: center;
  gap: 14px;
  min-width: 0;
}

.page-header__title {
  color: var(--dg-text-primary);
  font-size: 25px;
  line-height: 1.08;
  white-space: nowrap;
}

@media (max-width: 760px) {
  .page-header {
    align-items: flex-start;
    flex-wrap: wrap;
  }

  .page-header__lead {
    flex-wrap: wrap;
  }
}
</style>
