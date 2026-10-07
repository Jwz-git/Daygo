import type { app } from '../../wailsjs/go/models'
import type { LanguagePreference } from '@/i18n/locales'

/** JSON data from generated models: omit constructor helpers and accept null
 * for Go pointers, which Wails currently generates as optional fields. */
export type WireDTO<T> = T extends readonly (infer Item)[] ? WireDTO<Item>[]
  : T extends object ? {
    [K in keyof T as T[K] extends (...args: never[]) => unknown ? never : K]:
      WireDTO<T[K]> | (undefined extends T[K] ? null : never)
  } : T

type NonNullFields<T> = { [K in keyof T]: Exclude<T[K], null> }

type Replace<T, Fields> = Omit<T, keyof Fields> & {
  [K in keyof Fields]: K extends keyof T ? Fields[K] : never
}
type Nullable<T, K extends keyof WireDTO<T>> = Omit<WireDTO<T>, K> & {
  [P in K]-?: NonNullable<WireDTO<T>[P]> | null
}

/** Runtime wrappers validate closed strings; every other field derives from Go. */
export const APP_THEMES = ['system', 'light', 'dark'] as const
export type AppTheme = (typeof APP_THEMES)[number]
export const CAPTURE_INTERVAL_SECONDS = [1, 5, 10, 20, 30, 60] as const
export const CAPTURE_HEIGHTS = [720, 1080] as const
export const PROVIDER_PROTOCOLS = ['openai', 'openai_responses', 'anthropic'] as const
export type ProviderProtocol = (typeof PROVIDER_PROTOCOLS)[number]
export type PlanBlockStatus = 'planned' | 'done' | 'skipped'

export type LLMSettingsDTO = WireDTO<app.LLMSettingsDTO>
export type CaptureSettingsDTO = WireDTO<app.CaptureSettingsDTO>
export type PrivacySettingsDTO = WireDTO<app.PrivacySettingsDTO>
export type StorageSettingsDTO = WireDTO<app.StorageSettingsDTO>
export type SystemSettingsDTO = WireDTO<app.SystemSettingsDTO>
export type NotificationSettingsDTO = WireDTO<app.NotificationSettingsDTO>
export type AppearanceSettingsDTO = Replace<WireDTO<app.AppearanceSettingsDTO>, {
  theme: AppTheme
  language: LanguagePreference
}>
export type ChatSettingsDTO = Replace<WireDTO<app.ChatSettingsDTO>, { editMode: 'readonly' | 'edits' }>
/** Telemetry settings remain reserved in Go and have no current UI consumer. */
export type SettingsDTO = Replace<Omit<WireDTO<app.SettingsDTO>, 'telemetry'>, {
  appearance: AppearanceSettingsDTO
  chat: ChatSettingsDTO
}>
/** Absent means unchanged; the UI does not offer the reserved telemetry fields. */
export type SettingsPatch = Replace<Omit<NonNullFields<WireDTO<app.SettingsPatchDTO>>, 'analyticsOptIn' | 'crashReportingOptIn'>, {
  theme?: AppTheme
  language?: LanguagePreference
  chatEditMode?: 'readonly' | 'edits'
}>

export type ProviderDTO = Replace<WireDTO<app.ProviderDTO>, { protocol: ProviderProtocol }>
export type ProviderRoutingEntry = WireDTO<app.ProviderRoutingEntryDTO>
export type ProviderRoutingDTO = WireDTO<app.ProviderRoutingDTO>
/** Empty secret means keep the stored key, never clear it. */
export type ProviderInput = Replace<WireDTO<app.ProviderInputDTO>, { protocol: ProviderProtocol }>
export type ProviderModelsRequest = Replace<Partial<WireDTO<app.ProviderModelsRequestDTO>>, { protocol?: ProviderProtocol }>
export type ProviderModelsResult = WireDTO<app.ProviderModelsResultDTO>

export type DayContextDTO = WireDTO<app.DayContextDTO>
export type CategoryDTO = WireDTO<app.CategoryDTO>
export type AppSitesDTO = Nullable<app.AppSitesDTO, 'primary' | 'secondary'>
export type DistractionDTO = Nullable<app.DistractionDTO, 'videoSummaryUrl'>
export type ActivityPointDTO = WireDTO<app.ActivityPointDTO>
export type TimelineCardDTO = Replace<Nullable<app.TimelineCardDTO, 'batchId' | 'videoSummaryUrl' | 'appSites'>, {
  appSites: AppSitesDTO | null
  distractions: DistractionDTO[]
}>
export type CardMediaFrameDTO = WireDTO<app.CardMediaFrameDTO>
export type CardMediaDTO = WireDTO<app.CardMediaDTO>
export type TimelineFailureDTO = WireDTO<app.TimelineFailureDTO>
export type RangeDTO = WireDTO<app.RangeDTO>
export type TimelineDayDTO = Replace<WireDTO<app.TimelineDayDTO>, { cards: TimelineCardDTO[] }>
export type DailyRecapDTO = Nullable<app.DailyRecapDTO, 'generatedAtTs'>
export type JournalDayDTO = Nullable<app.JournalDayDTO, 'intentions' | 'notes' | 'goals' | 'reflections' | 'updatedAtTs'>
export type GoalCategoryRefDTO = WireDTO<app.GoalCategoryRefDTO>
export type DayGoalDTO = WireDTO<app.DayGoalDTO>

export type PlanBlockDTO = Replace<Nullable<app.PlanBlockDTO, 'notes' | 'completedAtTs'>, { status: PlanBlockStatus }>
export type PlanDayDTO = Replace<WireDTO<app.PlanDayDTO>, { blocks: PlanBlockDTO[] }>
export type PlanBlockInputDTO = Nullable<app.PlanBlockInputDTO, 'notes'>
export type CapabilitiesDTO = WireDTO<app.CapabilitiesDTO>
export type ReviewTotalsDTO = WireDTO<app.ReviewTotalsDTO>
export type CategoryTotalDTO = WireDTO<app.CategoryTotalDTO>
export type WeeklyIntervalDTO = WireDTO<app.WeeklyIntervalDTO>
export type WeeklySegmentDTO = Replace<WireDTO<app.WeeklySegmentDTO>, { appSites: AppSitesDTO | null }>
export type WeeklyDayDTO = Replace<WireDTO<app.WeeklyDayDTO>, { segments: WeeklySegmentDTO[] }>
export type WeeklyInsightsDTO = WireDTO<app.WeeklyInsightsDTO>
export type WeeklyDashboardDTO = Replace<WireDTO<app.WeeklyDashboardDTO>, { days: WeeklyDayDTO[] }>

export type ChatConversationDTO = WireDTO<app.ChatConversationDTO>
export type ChatMessageDTO = Replace<WireDTO<app.ChatMessageDTO>, {
  role: 'user' | 'assistant' | 'tool_call' | 'tool_result'
  status: 'ok' | 'failed' | 'canceled' | ''
}>
