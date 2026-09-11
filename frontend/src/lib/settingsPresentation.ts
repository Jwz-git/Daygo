export interface StorageLimitOption {
  bytes: number
  labelKey: 'unlimited' | 'oneGB' | 'twoGB' | 'fiveGB' | 'tenGB' | 'twentyGB'
}

export const STORAGE_LIMIT_OPTIONS: readonly StorageLimitOption[] = [
  { bytes: 0, labelKey: 'unlimited' },
  { bytes: 1_000_000_000, labelKey: 'oneGB' },
  { bytes: 2_000_000_000, labelKey: 'twoGB' },
  { bytes: 5_000_000_000, labelKey: 'fiveGB' },
  { bytes: 10_000_000_000, labelKey: 'tenGB' },
  { bytes: 20_000_000_000, labelKey: 'twentyGB' },
]

export function formatByteSize(bytes: number, locale: string): string {
  const safeBytes = Number.isFinite(bytes) ? Math.max(0, bytes) : 0
  const units = ['B', 'KB', 'MB', 'GB', 'TB'] as const
  let value = safeBytes
  let unitIndex = 0
  while (value >= 1000 && unitIndex < units.length - 1) {
    value /= 1000
    unitIndex += 1
  }
  const maximumFractionDigits = unitIndex === 0 || value >= 10 ? 0 : 1
  return `${new Intl.NumberFormat(locale, { maximumFractionDigits }).format(value)} ${units[unitIndex]}`
}

export function storageUsageRatio(usedBytes: number, limitBytes: number): number | null {
  if (!Number.isFinite(usedBytes) || !Number.isFinite(limitBytes) || limitBytes <= 0) {
    return null
  }
  return Math.min(Math.max(usedBytes / limitBytes, 0), 1)
}
