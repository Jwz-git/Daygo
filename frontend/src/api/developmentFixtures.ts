import type {
  CapabilitiesDTO,
  DailyRecapDTO,
  DayContextDTO,
  SettingsDTO,
  SettingsPatch,
  TimelineDayDTO,
  WeeklyDashboardDTO,
} from '@/api/dto'

export interface TimelineDevelopmentFixture {
  context: DayContextDTO
  day: TimelineDayDTO
  capabilities: CapabilitiesDTO
}

export interface DailyDevelopmentFixture {
  context: DayContextDTO
  timeline: TimelineDayDTO
  recap: DailyRecapDTO
  capabilities: CapabilitiesDTO
}

export interface WeeklyDevelopmentFixture {
  dashboard: WeeklyDashboardDTO
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

function isTimelineFixture(value: unknown): value is TimelineDevelopmentFixture {
  if (!isRecord(value) || !isRecord(value.context) || !isRecord(value.day)) return false
  if (!isRecord(value.capabilities)) return false

  return (
    typeof value.context.day === 'string' &&
    typeof value.context.dayStartTs === 'number' &&
    typeof value.context.dayEndTs === 'number' &&
    Array.isArray(value.day.cards) &&
    Array.isArray(value.day.categories) &&
    Array.isArray(value.day.failures) &&
    Array.isArray(value.day.processingRanges) &&
    typeof value.capabilities.canWrite === 'boolean'
  )
}

function isDailyFixture(value: unknown): value is DailyDevelopmentFixture {
  if (!isRecord(value) || !isRecord(value.context) || !isRecord(value.timeline)) return false
  if (!isRecord(value.recap) || !isRecord(value.capabilities)) return false

  return (
    typeof value.context.day === 'string' &&
    typeof value.context.standupDay === 'string' &&
    typeof value.context.dayStartTs === 'number' &&
    typeof value.context.dayEndTs === 'number' &&
    Array.isArray(value.timeline.cards) &&
    Array.isArray(value.timeline.categories) &&
    typeof value.recap.standupDay === 'string' &&
    Array.isArray(value.recap.highlights) &&
    Array.isArray(value.recap.tasks) &&
    typeof value.recap.blockersBody === 'string' &&
    typeof value.capabilities.canWrite === 'boolean'
  )
}

function isWeeklyFixture(value: unknown): value is WeeklyDevelopmentFixture {
  if (!isRecord(value) || !isRecord(value.dashboard)) return false

  const dashboard = value.dashboard
  if (!Array.isArray(dashboard.categories)) return false

  return (
    typeof dashboard.weekStart === 'string' &&
    typeof dashboard.weekStartTs === 'number' &&
    typeof dashboard.weekEndTs === 'number' &&
    typeof dashboard.trackedMinutes === 'number' &&
    typeof dashboard.focusMinutes === 'number' &&
    dashboard.categories.every(
      (category) =>
        isRecord(category) &&
        typeof category.name === 'string' &&
        typeof category.minutes === 'number' &&
        typeof category.share === 'number',
    )
  )
}

function isSettingsFixture(value: unknown): value is SettingsDTO {
  if (!isRecord(value) || !isRecord(value.capture) || !isRecord(value.llm)) return false
  if (!isRecord(value.privacy) || !isRecord(value.storage) || !isRecord(value.appearance) || !isRecord(value.system)) return false

  return (
    typeof value.capture.intervalSeconds === 'number' &&
    typeof value.capture.captureHeight === 'number' &&
    Array.isArray(value.privacy.blockedApplicationIds) &&
    typeof value.storage.recordingsLimitBytes === 'number' &&
    typeof value.appearance.theme === 'string' &&
    typeof value.appearance.language === 'string' &&
    typeof value.llm.outputLanguage === 'string' &&
    typeof value.llm.recognitionEnhancementEnabled === 'boolean' &&
    typeof value.system.agentEditsEnabled === 'boolean'
  )
}

/**
 * Applies a patch to a development fixture state. Dev-browser only: it stands in
 * for the backend's normalize-and-clamp step, but does not replicate it — the
 * real authority is always the value returned by the actual bindings.
 */
export function applyDevelopmentSettingsPatch(
  current: SettingsDTO,
  patch: SettingsPatch,
): SettingsDTO {
  const next: SettingsDTO = {
    capture: { ...current.capture },
    privacy: { blockedApplicationIds: [...current.privacy.blockedApplicationIds] },
    storage: { ...current.storage },
    appearance: { ...current.appearance },
    llm: { ...current.llm },
    chat: { ...current.chat },
    system: { ...current.system },
  }
  if (patch.intervalSeconds !== undefined) next.capture.intervalSeconds = patch.intervalSeconds
  if (patch.captureHeight !== undefined) next.capture.captureHeight = patch.captureHeight
  if (patch.blockedApplicationIds !== undefined) {
    next.privacy.blockedApplicationIds = [...patch.blockedApplicationIds]
  }
  if (patch.theme !== undefined) next.appearance.theme = patch.theme
  if (patch.language !== undefined) next.appearance.language = patch.language
  if (patch.recordingsLimitBytes !== undefined) {
    next.storage.recordingsLimitBytes = patch.recordingsLimitBytes
  }
  if (patch.outputLanguage !== undefined) next.llm.outputLanguage = patch.outputLanguage
  if (patch.recognitionEnhancementEnabled !== undefined) {
    next.llm.recognitionEnhancementEnabled = patch.recognitionEnhancementEnabled
  }
  if (patch.agentEditsEnabled !== undefined) next.system.agentEditsEnabled = patch.agentEditsEnabled
  if (patch.chatMemory !== undefined) next.chat.memory = patch.chatMemory
  return next
}

async function fetchDevelopmentFixture<T>(
  path: string,
  validate: (value: unknown) => value is T,
): Promise<T | null> {
  if (!import.meta.env.DEV) return null

  try {
    const response = await fetch(path, { cache: 'no-store' })
    if (!response.ok) return null
    const raw: unknown = await response.json()
    return validate(raw) ? raw : null
  } catch {
    return null
  }
}

/**
 * Browser/Wails development fallback only. Vite serves the payload from
 * frontend/dev-fixtures; that directory is outside src and never enters the
 * production bundle. A malformed or missing fixture quietly preserves the
 * normal "capability unavailable" state.
 */
export async function getTimelineDevelopmentFixture(): Promise<TimelineDevelopmentFixture | null> {
  return fetchDevelopmentFixture('/__daygo_dev__/timeline', isTimelineFixture)
}

export async function getDailyDevelopmentFixture(): Promise<DailyDevelopmentFixture | null> {
  return fetchDevelopmentFixture('/__daygo_dev__/daily', isDailyFixture)
}

export async function getWeeklyDevelopmentFixture(): Promise<WeeklyDevelopmentFixture | null> {
  return fetchDevelopmentFixture('/__daygo_dev__/weekly', isWeeklyFixture)
}

export async function getSettingsDevelopmentFixture(): Promise<SettingsDTO | null> {
  return fetchDevelopmentFixture('/__daygo_dev__/settings', isSettingsFixture)
}
