<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps<{
  /** Unix timestamp in milliseconds */
  timestamp: number
}>()

const { t, locale } = useI18n()

const label = computed(() => {
  const now = Date.now()
  const diff = now - props.timestamp
  const MS_PER_DAY = 86_400_000
  const msToday = now % MS_PER_DAY
  const todayStart = now - msToday
  const yesterdayStart = todayStart - MS_PER_DAY

  if (props.timestamp >= todayStart) return t('chat.dateDivider.today')
  if (props.timestamp >= yesterdayStart) return t('chat.dateDivider.yesterday')

  // format as locale date
  return new Intl.DateTimeFormat(
    locale.value,
    { month: 'short', day: 'numeric', year: 'numeric' },
  ).format(new Date(props.timestamp))
})
</script>

<template>
  <div class="date-divider" role="separator" :aria-label="label">
    <span class="date-divider__line" />
    <span class="date-divider__label">{{ label }}</span>
    <span class="date-divider__line" />
  </div>
</template>

<style scoped>
.date-divider {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 4px 0;
}

.date-divider__line {
  flex: 1;
  height: 1px;
  background: var(--dg-chip-border);
}

.date-divider__label {
  flex: none;
  padding: 2px 10px;
  border-radius: 999px;
  background: var(--dg-track-fill);
  color: var(--dg-text-muted);
  font-size: 11px;
  font-weight: 600;
  white-space: nowrap;
}
</style>
