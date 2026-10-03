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


/** LLMSettingsDTO — how recognition requests are sent to the model. Card and
 *  summary language is not configurable: it follows the interface language. */
export interface LLMSettingsDTO {
  /** Reveals the token-usage card in the daily and weekly reports. Default off. */
  showTokenUsage: boolean
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
  /** Start Daygo automatically when the user logs in (SMAppService on macOS). */
  launchAtLogin: boolean
  showDockIcon: boolean
  agentEditsEnabled: boolean
  /** Reveals the sidebar test page with the test-only features. */
  testToolsEnabled: boolean
}

/** AppearanceSettingsDTO is persisted independently from LLM output language. */
export interface AppearanceSettingsDTO {
  theme: AppTheme
  /** Empty means follow the system language. */
  language: LanguagePreference
}

/**
 * NotificationSettingsDTO — the daily journal reminder. `journalReminderTime` is
 * a local wall-clock "HH:mm"; the recurrence is owned by the Go scheduler, which
 * re-arms one platform notification at a time
 * (docs/decisions/notifications-journal-reminder.md).
 */
export interface NotificationSettingsDTO {
  journalReminderEnabled: boolean
  journalReminderTime: string
}

/** SettingsDTO groups the settings consumed by current frontend sections. */
export interface SettingsDTO {
  capture: CaptureSettingsDTO
  privacy: PrivacySettingsDTO
  storage: StorageSettingsDTO
  appearance: AppearanceSettingsDTO
  llm: LLMSettingsDTO
  chat: ChatSettingsDTO
  notifications: NotificationSettingsDTO
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
  showTokenUsage?: boolean
  journalReminderEnabled?: boolean
  journalReminderTime?: string
  launchAtLogin?: boolean
  showDockIcon?: boolean
  agentEditsEnabled?: boolean
  testToolsEnabled?: boolean
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
  /** The ordered models configured under this endpoint/key; at least one. */
  models: string[]
  /** Image parts per request cap; 0 means the built-in default. */
  maxImages: number
  hasSecret: boolean
}

/**
 * One fallback-chain entry: a (provider, model) pair. The same provider can
 * appear under different models as separate entries with independent ordering.
 * An empty `model` follows the provider's first configured model.
 */
export interface ProviderRoutingEntry {
  providerId: string
  model: string
}

/**
 * ProviderRoutingDTO: the ordered fallback chain. `chain[0]` is the primary;
 * the rest are fallbacks tried in order. The backend dedupes by (provider,
 * model) pair, drops empties, and caps the chain at 8 on write.
 */
export interface ProviderRoutingDTO {
  chain: ProviderRoutingEntry[]
}

/** ProviderInputDTO. `secret` "" means "keep the stored key", never "clear". */
export interface ProviderInput {
  displayName: string
  protocol: ProviderProtocol
  endpoint: string
  /** At least one model; trimmed and deduped by the backend, capped at 20. */
  models: string[]
  maxImages: number
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

export interface ActivityPointDTO {
  time: string
  description: string
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
  activityPoints: ActivityPointDTO[]
  isIdle: boolean
  durationMinutes: number
}

/** One playable screenshot frame: a numeric resource ID plus capture time. */
export interface CardMediaFrameDTO {
  id: number
  capturedAt: number
}

/** Frame listing covering a card's timespan, oldest first. */
export interface CardMediaDTO {
  cardId: number
  frames: CardMediaFrameDTO[]
}

export interface TimelineFailureDTO {
  batchIds: number[]
  startTs: number
  endTs: number
  kind: string
  message: string
  retryable: boolean
}

/**
 * One window on the day track plus the batches that own it. A card belongs to
 * the window its batch covers, which is not the same as overlapping it: an
 * ongoing rewrite extends a batch's span back over the card it continues, so a
 * card the rerun will replace can sit entirely before the window.
 */
export interface RangeDTO {
  startTs: number
  endTs: number
  batchIds: number[]
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

/** Plan block status: the explicit completion mark (docs/modules/plan.md). */
export type PlanBlockStatus = 'planned' | 'done' | 'skipped'

/** One plan block with its coverage by recorded cards up to now. */
export interface PlanBlockDTO {
  id: number
  day: string
  /** 24-hour HH:mm; times before 04:00 belong to the next calendar date. */
  start: string
  end: string
  startTs: number
  endTs: number
  title: string
  notes: string | null
  categoryId: string
  categoryName: string
  colorHex: string
  status: PlanBlockStatus
  completedAtTs: number | null
  remind: boolean
  /** Recorded card minutes in the block's own category, up to now. */
  matchedMinutes: number
  /** Distraction minutes inside the block, up to now. */
  distractionMinutes: number
}

export interface PlanDayDTO {
  day: string
  blocks: PlanBlockDTO[]
}

/** Adds a block (id 0) or replaces one's editable fields. */
export interface PlanBlockInputDTO {
  id: number
  day: string
  start: string
  end: string
  title: string
  notes: string | null
  categoryId: string
  remind: boolean
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
  colorHex: string
}

export interface WeeklySegmentDTO {
  startTs: number
  endTs: number
  category: string
  isIdle: boolean
  /** The card's raw app/site pair (same shape as on timeline cards); null when absent. */
  appSites: AppSitesDTO | null
  /** The card's distraction intervals, resolved by Go and clamped to this segment. */
  distractions: WeeklyIntervalDTO[]
}

/** Half-open [startTs, endTs) range in Unix seconds. */
export interface WeeklyIntervalDTO {
  startTs: number
  endTs: number
}

export interface WeeklyDayDTO {
  day: string
  trackedMinutes: number
  focusMinutes: number
  categories: CategoryTotalDTO[]
  segments: WeeklySegmentDTO[]
}

export interface WeeklyInsightsDTO {
  longestFocusMinutes: number
  longestFocusDay: string
  peakHour: number
  peakHourMinutes: number
  mostActiveDay: string
  mostActiveDayMinutes: number
  activeDays: number
  avgDailyFocusMinutes: number
}

export interface WeeklyDashboardDTO {
  weekStart: string
  weekStartTs: number
  weekEndTs: number
  trackedMinutes: number
  focusMinutes: number
  categories: CategoryTotalDTO[]
  days: WeeklyDayDTO[]
  insights: WeeklyInsightsDTO
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
