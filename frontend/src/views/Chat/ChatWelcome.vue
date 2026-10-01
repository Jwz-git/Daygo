<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { tm, rt } = useI18n()

const emit = defineEmits<{
  select: [prompt: string]
}>()
</script>

<template>
  <div class="welcome">
    <!-- Illustration / icon -->
    <div class="welcome__icon" aria-hidden="true">
      <svg width="48" height="48" viewBox="0 0 48 48" fill="none">
        <rect width="48" height="48" rx="12" fill="color-mix(in srgb, var(--dg-accent) 12%, transparent)"/>
        <path d="M14 20c0-2.2 1.8-4 4-4h12c2.2 0 4 1.8 4 4v8c0 2.2-1.8 4-4 4H18c-2.2 0-4-1.8-4-4v-8z" stroke="var(--dg-accent-text)" stroke-width="2" fill="none"/>
        <path d="M19 32h10M24 32v-8" stroke="var(--dg-accent-text)" stroke-width="2" stroke-linecap="round"/>
        <circle cx="24" cy="16" r="2" fill="var(--dg-accent-text)"/>
      </svg>
    </div>

    <h2 class="welcome__title">{{ $t('chat.welcome.title') }}</h2>
    <p class="welcome__subtitle">{{ $t('chat.welcome.subtitle') }}</p>

    <ul class="welcome__hints">
      <li v-for="(hint, idx) in (tm('chat.welcome.hints') as string[])" :key="idx">
        <button type="button" class="welcome__hint" @click="emit('select', rt(hint))">
          {{ rt(hint) }}
        </button>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.welcome {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 14px;
  flex: 1;
  padding: 32px 24px;
  text-align: center;
}

.welcome__icon {
  opacity: 0.8;
}

.welcome__title {
  margin: 0;
  color: var(--dg-text-primary);
  font-size: 18px;
  font-weight: 700;
}

.welcome__subtitle {
  margin: 0;
  color: var(--dg-text-secondary);
  font-size: 13px;
  max-width: 320px;
}

.welcome__hints {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin: 0;
  padding: 16px 20px;
  list-style: none;
  border: 1px solid var(--dg-card-border);
  border-radius: 10px;
  background: var(--dg-card-fill);
  max-width: 360px;
  width: 100%;
  text-align: left;
}

.welcome__hints li { list-style: none; }

.welcome__hint {
  display: block;
  width: 100%;
  padding: 7px 10px;
  border: 1px solid transparent;
  border-radius: 7px;
  color: var(--dg-text-secondary);
  font: inherit;
  font-size: 13px;
  text-align: left;
  background: var(--dg-track-fill);
  cursor: pointer;
  transition:
    background var(--dg-motion-base) ease,
    border-color var(--dg-motion-base) ease,
    color var(--dg-motion-base) ease;
}

.welcome__hint:hover {
  border-color: var(--dg-accent);
  color: var(--dg-text-primary);
  background: var(--dg-control-fill);
}

.welcome__hint:focus-visible {
  outline: none;
  border-color: var(--dg-accent);
  box-shadow: 0 0 0 3px var(--dg-focus-ring);
}
</style>
