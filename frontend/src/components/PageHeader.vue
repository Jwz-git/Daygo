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
  position: relative;
  z-index: 3;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  padding: 30px var(--dg-page-padding) 20px;
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
  font-size: 28px;
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
