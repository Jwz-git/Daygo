<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { WeeklyState } from '@/stores/weekly'

const props = defineProps<{ state: Exclude<WeeklyState, 'populated'> }>()
const emit = defineEmits<{ retry: [] }>()
const { t } = useI18n()

const title = computed(() => t(`weekly.state.${props.state}.title`))
const description = computed(() => t(`weekly.state.${props.state}.description`))
</script>

<template>
  <section
    :class="['weekly-state', 'dg-card', { 'weekly-state--failure': state === 'failure' }]"
    role="status"
    aria-live="polite"
  >
    <div class="weekly-state__mark" aria-hidden="true" />
    <div>
      <h2>{{ title }}</h2>
      <p>{{ description }}</p>
      <button
        v-if="state === 'failure'"
        type="button"
        class="dg-button"
        @click="emit('retry')"
      >
        {{ t('common.action.retry') }}
      </button>
    </div>
  </section>
</template>

<style scoped>
.weekly-state {
  display: grid;
  grid-template-columns: 7px minmax(0, 1fr);
  gap: 16px;
  min-height: 150px;
  padding: 28px;
}

.weekly-state__mark {
  width: 7px;
  height: 7px;
  margin-top: 8px;
  border-radius: 50%;
  background: var(--dg-text-muted);
}

.weekly-state h2 {
  color: var(--dg-text-primary);
  font-size: 16px;
  font-weight: 650;
}

.weekly-state p {
  max-width: 560px;
  margin-top: 6px;
  color: var(--dg-text-tertiary);
  font-size: 12px;
  line-height: 1.6;
}

.weekly-state .dg-button { margin-top: 16px; }

.weekly-state--failure .weekly-state__mark {
  background: var(--dg-danger);
}
</style>
