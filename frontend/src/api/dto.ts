import type * as app from '../../bindings/github.com/Jwz-git/Daygo/internal/app/models'
import type { LanguagePreference } from '@/i18n/locales'

/** JSON boundary types derived from generated interfaces; optional Go pointer
 * fields accept null as well as omission. */
export type WireDTO<T> = T extends readonly (infer Item)[] ? WireDTO<Item>[]
  : T extends object ? {
    [K in keyof T as T[K] extends (...args: never[]) => unknown ? never : K]:
      WireDTO<T[K]> | (undefined extends T[K] ? null : never)
  } : T

/** UI collections are normalised to [] at API entry points; scalar pointers
 * retain null. Generated v3 models describe Go nil slices explicitly. */
export type ViewDTO<T> = [NonNullable<T>] extends [readonly (infer Item)[]] ? ViewDTO<Item>[]
 : T extends object ? { [K in keyof T]: ViewDTO<T[K]> } : T

type NonNullFields<T> = { [K in keyof T]: Exclude<T[K], null | undefined> }

type Replace<T, Fields> = Omit<T, keyof Fields> & {
  [K in keyof Fields]: K extends keyof T ? Fields[K] : never
}
type Nullable<T, K extends keyof ViewDTO<T>> = Omit<ViewDTO<T>, K> & {
  [P in K]-?: NonNullable<ViewDTO<T>[P]> | null
}

/** Runtime wrappers validate closed strings; every other field derives from Go. */
export const APP_THEMES = ['system', 'light', 'dark'] as const
export type AppTheme = (typeof APP_THEMES)[number]
export const CAPTURE_INTERVAL_SECONDS = [1, 5, 10, 20, 30, 60] as const
export const CAPTURE_HEIGHTS = [720, 1080] as const
export const PROVIDER_PROTOCOLS = ['openai', 'openai_responses', 'anthropic', 'gemini', 'chatgpt'] as const
export type ProviderProtocol = (typeof PROVIDER_PROTOCOLS)[number]
export type PlanBlockStatus = 'planned' | 'done' | 'skipped'
/** A category's closed visual role (docs/03 §3.3.3); never sent to the model. */
export const CATEGORY_ARCHETYPES = ['forge', 'sanctum', 'nexus', 'camp', 'order'] as const
export type CategoryArchetype = (typeof CATEGORY_ARCHETYPES)[number]

export type LLMSettingsDTO = ViewDTO<app.LLMSettingsDTO>
export type CaptureSettingsDTO = ViewDTO<app.CaptureSettingsDTO>
export type PrivacySettingsDTO = ViewDTO<app.PrivacySettingsDTO>
export type StorageSettingsDTO = ViewDTO<app.StorageSettingsDTO>
export type SystemSettingsDTO = ViewDTO<app.SystemSettingsDTO>
export type NotificationSettingsDTO = ViewDTO<app.NotificationSettingsDTO>
export type AppearanceSettingsDTO = Replace<ViewDTO<app.AppearanceSettingsDTO>, {
  theme: AppTheme
  language: LanguagePreference
}>
export type ChatSettingsDTO = Replace<ViewDTO<app.ChatSettingsDTO>, { editMode: 'readonly' | 'edits' }>
/** Telemetry settings remain reserved in Go and have no current UI consumer. */
export type SettingsDTO = Replace<Omit<ViewDTO<app.SettingsDTO>, 'telemetry'>, {
  appearance: AppearanceSettingsDTO
  chat: ChatSettingsDTO
}>
/** Absent means unchanged; the UI does not offer the reserved telemetry fields. */
export type SettingsPatch = Replace<Omit<Partial<NonNullFields<ViewDTO<app.SettingsPatchDTO>>>, 'analyticsOptIn' | 'crashReportingOptIn'>, {
  theme?: AppTheme
  language?: LanguagePreference
  chatEditMode?: 'readonly' | 'edits'
}>

export type ProviderDTO = Replace<ViewDTO<app.ProviderDTO>, { protocol: ProviderProtocol }>
export type ProviderRoutingEntry = ViewDTO<app.ProviderRoutingEntryDTO>
export type ProviderRoutingDTO = ViewDTO<app.ProviderRoutingDTO>
/** Empty secret means keep the stored key, never clear it. */
export type ProviderInput = Replace<ViewDTO<app.ProviderInputDTO>, { protocol: ProviderProtocol }>
export type ProviderModelsRequest = Replace<Partial<ViewDTO<app.ProviderModelsRequestDTO>>, { protocol?: ProviderProtocol }>
export type ProviderModelsResult = ViewDTO<app.ProviderModelsResultDTO>

export type DayContextDTO = ViewDTO<app.DayContextDTO>
export type CategoryDTO = Replace<ViewDTO<app.CategoryDTO>, { archetype: CategoryArchetype }>
export type CategoryPresetDTO = Replace<ViewDTO<app.CategoryPresetDTO>, { archetype: CategoryArchetype }>
export type AppSitesDTO = Nullable<app.AppSitesDTO, 'primary' | 'secondary'>
export type DistractionDTO = Nullable<app.DistractionDTO, 'videoSummaryUrl'>
export type ActivityPointDTO = ViewDTO<app.ActivityPointDTO>
export type TimelineCardDTO = Replace<Nullable<app.TimelineCardDTO, 'batchId' | 'videoSummaryUrl' | 'appSites'>, {
  appSites: AppSitesDTO | null
  distractions: DistractionDTO[]
}>
export type CardMediaFrameDTO = ViewDTO<app.CardMediaFrameDTO>
export type CardMediaDTO = ViewDTO<app.CardMediaDTO>
export type TimelineFailureDTO = ViewDTO<app.TimelineFailureDTO>
export type RangeDTO = ViewDTO<app.RangeDTO>
export type TimelineDayDTO = Replace<ViewDTO<app.TimelineDayDTO>, { cards: TimelineCardDTO[]; categories: CategoryDTO[] }>
export type DailyRecapDTO = Nullable<app.DailyRecapDTO, 'generatedAtTs'>
export type JournalDayDTO = Nullable<app.JournalDayDTO, 'intentions' | 'notes' | 'goals' | 'reflections' | 'updatedAtTs'>
export type GoalCategoryRefDTO = ViewDTO<app.GoalCategoryRefDTO>
export type DayGoalDTO = ViewDTO<app.DayGoalDTO>

export type PlanBlockDTO = Replace<Nullable<app.PlanBlockDTO, 'notes' | 'completedAtTs'>, { status: PlanBlockStatus }>
export type PlanDayDTO = Replace<ViewDTO<app.PlanDayDTO>, { blocks: PlanBlockDTO[] }>
export type PlanBlockInputDTO = Nullable<app.PlanBlockInputDTO, 'notes'>
export type CapabilitiesDTO = ViewDTO<app.CapabilitiesDTO>
export type ReviewTotalsDTO = ViewDTO<app.ReviewTotalsDTO>
export type CategoryTotalDTO = ViewDTO<app.CategoryTotalDTO>
export type WeeklyIntervalDTO = ViewDTO<app.WeeklyIntervalDTO>
export type WeeklySegmentDTO = Replace<ViewDTO<app.WeeklySegmentDTO>, { appSites: AppSitesDTO | null }>
export type WeeklyDayDTO = Replace<ViewDTO<app.WeeklyDayDTO>, { segments: WeeklySegmentDTO[] }>
export type WeeklyInsightsDTO = ViewDTO<app.WeeklyInsightsDTO>
export type WeeklyDashboardDTO = Replace<ViewDTO<app.WeeklyDashboardDTO>, { days: WeeklyDayDTO[] }>

export type ChatConversationDTO = ViewDTO<app.ChatConversationDTO>
export type ChatMessageDTO = Replace<ViewDTO<app.ChatMessageDTO>, {
  role: 'user' | 'assistant' | 'tool_call' | 'tool_result'
  status: 'ok' | 'failed' | 'canceled' | ''
}>

export type ChatGPTConnection = ViewDTO<app.ChatGPTConnectionDTO>
