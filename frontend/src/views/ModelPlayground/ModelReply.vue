<script setup lang="ts">
defineProps<{ text: string }>()
</script>

<template>
  <!-- Untrusted model output stays text, including HTML and remote image syntax. -->
  <pre class="model-response">{{ text }}</pre>
</template>

<style scoped>
.model-response { white-space: pre-wrap; overflow-wrap: anywhere; font: inherit; line-height: 1.7; margin: 0; min-height: 160px; user-select: text; }
</style>
