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
.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: var(--dg-page-padding) var(--dg-page-padding) 18px;
}

.page-header__lead,
.page-header__trail {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.page-header__title {
  font-size: 26px;
  line-height: 1.1;
  white-space: nowrap;
}
</style>
