import type { LanguagePreference } from '@/i18n/locales'

/*
 * Hand-written subset of the DTOs in docs/05-interface-contract.md §5.5.2.
 *
 * TEMPORARY. §5.5.5 rule 2 makes the generated api/generated/models.ts the one
 * source of DTO types; `wails generate module` cannot run yet because no
 * bindings exist. Field names below are deliberately identical to the Go JSON
 * tags so that swapping the import is a delete, not a rewrite.
 *
 * Where the Go side writes `string` but the contract comment lists a closed set,
 * the type here is narrowed to that set. That narrowing is what makes vue-tsc
 * reject an unhandled theme or protocol instead of letting it reach the UI.
 */

/** AppearanceSettingsDTO.Theme. Three-state; resolved to two before it reaches the DOM. */
export const APP_THEMES = ['system', 'light', 'dark'] as const

export type AppTheme = (typeof APP_THEMES)[number]


/** LLMSettingsDTO — how recognition requests are sent to the model. */
export interface LLMSettingsDTO {
  /** BCP 47; "" follows the interface language. */
  outputLanguage: string
  /**
   * Recognition enhancement: each image is split in memory into four
   * overlapping tiles before it is sent, so small text survives provider-side
   * downscaling. Off sends the original image unchanged. Increases usage.
   */
  recognitionEnhancementEnabled: boolean
}

/**
 * The closed sets the backend clamps reads and writes to (internal/settings).
 * They drive the UI options; the DTO fields themselves stay `number` because
 * the generated bindings map Go's int to plain number.
 */
export const CAPTURE_INTERVAL_SECONDS = [1, 5, 10, 20, 30, 60] as const

export const CAPTURE_HEIGHTS = [720, 1080] as const

export interface CaptureSettingsDTO {
  intervalSeconds: number
  captureHeight: number
}

/** PrivacySettingsDTO — bundle IDs of apps excluded from screenshots. */
export interface PrivacySettingsDTO {
  blockedApplicationIds: string[]
}

/** StorageSettingsDTO. recordingsLimitBytes: 0 means no limit. */
export interface StorageSettingsDTO {
  recordingsLimitBytes: number
}
/** Settings exposes the system settings consumed by current sections. */
export interface SystemSettingsDTO {
  agentEditsEnabled: boolean
}

/** AppearanceSettingsDTO is persisted independently from LLM output language. */
export interface AppearanceSettingsDTO {
  theme: AppTheme
  /** Empty means follow the system language. */
  language: LanguagePreference
}

/** SettingsDTO groups the settings consumed by current frontend sections. */
export interface SettingsDTO {
  capture: CaptureSettingsDTO
  privacy: PrivacySettingsDTO
  storage: StorageSettingsDTO
  appearance: AppearanceSettingsDTO
  llm: LLMSettingsDTO
  chat: ChatSettingsDTO
  system: SystemSettingsDTO
}

/** SettingsPatchDTO subset — every field optional; absent means leave unchanged. */
export interface SettingsPatch {
  intervalSeconds?: number
  captureHeight?: number
  blockedApplicationIds?: string[]
  recordingsLimitBytes?: number
  theme?: AppTheme
  language?: LanguagePreference
  outputLanguage?: string
  recognitionEnhancementEnabled?: boolean
  agentEditsEnabled?: boolean
  chatMemory?: string
  chatEditMode?: 'readonly' | 'edits'
}

/**
 * ChatSettingsDTO — the global chat memory, injected into every conversation,
 * plus the agent sandbox gate. `editMode` is "readonly" (default) or "edits";
 * it gates the in-app chat assistant only, independent of the external
 * agent.sock channel.
 */
export interface ChatSettingsDTO {
  memory: string
  editMode: 'readonly' | 'edits'
}


/** The wire protocol a custom endpoint speaks. */
export const PROVIDER_PROTOCOLS = ['openai', 'openai_responses', 'anthropic'] as const

export type ProviderProtocol = (typeof PROVIDER_PROTOCOLS)[number]

/**
 * One user-defined provider. `id` is an opaque generated identifier; the wire
 * protocol is its own field rather than something the id implies.
 *
 * `hasSecret` is the whole of what the UI may learn about the key: it is
 * write-only, and no field on this type ever carries the key itself.
 */
export interface ProviderDTO {
  id: string
  displayName: string
  protocol: ProviderProtocol
  endpoint: string
  model: string
  hasSecret: boolean
}

/**
 * ProviderRoutingDTO: the ordered fallback chain. `chain[0]` is the primary;
 * the rest are fallbacks tried in order. The backend dedupes, drops empties,
 * and caps the chain at 8 on write.
 */
export interface ProviderRoutingDTO {
  chain: string[]
}

/** ProviderInputDTO. `secret` "" means "keep the stored key", never "clear". */
export interface ProviderInput {
  displayName: string
  protocol: ProviderProtocol
  endpoint: string
  model: string
  secret: string
}

/** ListProviderModels request: a saved provider id, or a draft's fields. */
export interface ProviderModelsRequest {
  providerId?: string
  protocol?: ProviderProtocol
  endpoint?: string
  secret?: string
}

/** One model-listing outcome. A failed listing is a result, not an exception. */
export interface ProviderModelsResult {
  ok: boolean
  models: string[]
  errorCode: string
  message: string
}

/**
 * Draft for one connection probe. The secret crosses the Wails boundary for
 * this call only and is never persisted, logged or echoed back.
 */
export interface ProviderTestDraft {
  protocol: ProviderProtocol
  endpoint: string
  model: string
  secret: string
}

/** One probe outcome. A failed probe is a result, not an exception. */
export interface ProviderTestResult {
  ok: boolean
  model: string
  latencyMs: number
  capabilities: string[]
  errorCode: string
  message: string
}

// Timeline DTOs mirror docs/05-interface-contract.md §5.5.2. They stay
// hand-written only until these target bindings exist and Wails can generate
// the same shapes into api/generated/models.ts.
export interface DayContextDTO {
  day: string
  standupDay: string
  /** Monday yyyy-MM-dd of the week containing the logical day; the backend owns week boundaries. */
  weekStart: string
  dayStartTs: number
  dayEndTs: number
  nowTs: number
  timeZone: string
  dayBoundaryHour: number
}

export interface CategoryDTO {
  id: string
  name: string
  colorHex: string
  details: string
  sortOrder: number
  isSystem: boolean
  isIdle: boolean
  createdAtTs: number
  updatedAtTs: number
}

export interface AppSitesDTO {
  primary: string | null
  secondary: string | null
}

export interface DistractionDTO {
  id: string
  startTime: string
  endTime: string
  title: string
  summary: string
  videoSummaryUrl: string | null
}

export interface TimelineCardDTO {
  id: number
  batchId: number | null
  day: string
  start: string
  end: string
  startTs: number
  endTs: number
  category: string
  subcategory: string
  title: string
  summary: string
  detailedSummary: string
  videoSummaryUrl: string | null
  otherVideoSummaryUrls: string[]
  appSites: AppSitesDTO | null
  distractions: DistractionDTO[]
  isIdle: boolean
  durationMinutes: number
}

export interface TimelineFailureDTO {
  batchIds: number[]
  startTs: number
  endTs: number
  kind: string
  message: string
  retryable: boolean
}

export interface RangeDTO {
  startTs: number
  endTs: number
}

export interface TimelineDayDTO {
  day: string
  dayStartTs: number
  dayEndTs: number
  cards: TimelineCardDTO[]
  categories: CategoryDTO[]
  trackedMinutes: number
  idleMinutes: number
  failures: TimelineFailureDTO[]
  processingRanges: RangeDTO[]
  generatedAtTs: number
}

export interface DailyRecapDTO {
  standupDay: string
  highlightsTitle: string
  highlights: string[]
  tasksTitle: string
  tasks: string[]
  blockersTitle: string
  blockersBody: string
  generatedAtTs: number | null
}

export interface JournalDayDTO {
  day: string
  intentions: string | null
  notes: string | null
  goals: string | null
  reflections: string | null
  /** AI generated, read-only for users. */
  summary: string | null
  /** draft | intentions_set | complete; empty means no entry exists yet. */
  status: string
  updatedAtTs: number | null
}

export interface GoalCategoryRefDTO {
  categoryId: string
  name: string
  colorHex: string
  sortOrder: number
}

export interface DayGoalDTO {
  day: string
  focusTargetMinutes: number
  distractionLimitMinutes: number
  isSkipped: boolean
  focusCategories: GoalCategoryRefDTO[]
  distractionCategories: GoalCategoryRefDTO[]
  /** false when no goal is set for the day yet. */
  exists: boolean
}

export interface CapabilitiesDTO {
  canWrite: boolean
  isCaptureOwner: boolean
  features: string[]
  appVersion: string
  apiRevision: number
}

export interface CategoryTotalDTO {
  name: string
  minutes: number
  share: number
}

export interface WeeklyDashboardDTO {
  weekStart: string
  weekStartTs: number
  weekEndTs: number
  trackedMinutes: number
  focusMinutes: number
  categories: CategoryTotalDTO[]
}

/** ChatConversationDTO — one thread in the sidebar list. */
export interface ChatConversationDTO {
  id: string
  title: string
  /** "" = no provider selected yet; the user must pick one before sending. */
  providerId: string
  /** "" = follow the provider's configured model; otherwise an override. */
  model: string
  updatedAt: number
}

/** ChatMessageDTO — one transcript row. Status is set on assistant messages.
 * tool_call rows carry the tool name and its arguments JSON in toolName /
 * toolArguments; the following tool_result row pairs by toolName with the
 * result envelope JSON in content. */
export interface ChatMessageDTO {
  id: number
  role: 'user' | 'assistant' | 'tool_call' | 'tool_result'
  content: string
  status: 'ok' | 'failed' | 'canceled' | ''
  toolName: string
  toolArguments: string
  createdAt: number
}
