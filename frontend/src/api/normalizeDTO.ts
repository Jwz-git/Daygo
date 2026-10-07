import type { app } from '../../wailsjs/go/models'
import { APP_THEMES, PROVIDER_PROTOCOLS } from '@/api/dto'
import type {
  AppSitesDTO, ChatMessageDTO, DailyRecapDTO, JournalDayDTO, PlanDayDTO,
  ProviderDTO, SettingsDTO, TimelineDayDTO, WeeklyDashboardDTO, WireDTO,
} from '@/api/dto'
import { SUPPORTED_LOCALES } from '@/i18n/locales'

export function oneOf<const Values extends readonly string[]>(value: string, values: Values, field: string): Values[number] {
  const match = values.find((candidate) => candidate === value)
  // The field is a fixed label, never the returned value or payload content.
  if (match === undefined) throw new Error(`invalid_binding_payload: ${field}`)
  return match
}

export function normalizeSettings(value: WireDTO<app.SettingsDTO>): SettingsDTO {
  return {
    ...value,
    appearance: {
      ...value.appearance,
      theme: oneOf(value.appearance.theme, APP_THEMES, 'theme'),
      language: oneOf(value.appearance.language, ['', ...SUPPORTED_LOCALES], 'language'),
    },
    chat: { ...value.chat, editMode: oneOf(value.chat.editMode, ['readonly', 'edits'], 'editMode') },
  }
}

export function normalizeProvider(value: WireDTO<app.ProviderDTO>): ProviderDTO {
  return { ...value, protocol: oneOf(value.protocol, PROVIDER_PROTOCOLS, 'protocol') }
}

export function normalizeChatMessage(value: WireDTO<app.ChatMessageDTO>): ChatMessageDTO {
  return {
    ...value,
    role: oneOf(value.role, ['user', 'assistant', 'tool_call', 'tool_result'], 'role'),
    status: oneOf(value.status, ['ok', 'failed', 'canceled', ''], 'status'),
  }
}

function normalizeAppSites(value: WireDTO<app.AppSitesDTO> | null | undefined): AppSitesDTO | null {
  return value == null ? null : { primary: value.primary ?? null, secondary: value.secondary ?? null }
}

export function normalizeTimelineDay(value: WireDTO<app.TimelineDayDTO>): TimelineDayDTO {
  return {
    ...value,
    cards: value.cards.map((card) => ({
      ...card,
      batchId: card.batchId ?? null,
      videoSummaryUrl: card.videoSummaryUrl ?? null,
      appSites: normalizeAppSites(card.appSites),
      distractions: card.distractions.map((item) => ({ ...item, videoSummaryUrl: item.videoSummaryUrl ?? null })),
    })),
  }
}

export function normalizeRecap(value: WireDTO<app.DailyRecapDTO>): DailyRecapDTO {
  return { ...value, generatedAtTs: value.generatedAtTs ?? null }
}

export function normalizeJournal(value: WireDTO<app.JournalDayDTO>): JournalDayDTO {
  return {
    ...value, intentions: value.intentions ?? null, notes: value.notes ?? null,
    goals: value.goals ?? null, reflections: value.reflections ?? null, updatedAtTs: value.updatedAtTs ?? null,
  }
}

export function normalizePlanDay(value: WireDTO<app.PlanDayDTO>): PlanDayDTO {
  return {
    ...value,
    blocks: value.blocks.map((block) => ({
      ...block,
      notes: block.notes ?? null,
      completedAtTs: block.completedAtTs ?? null,
      status: oneOf(block.status, ['planned', 'done', 'skipped'], 'status'),
    })),
  }
}

export function normalizeWeeklyDashboard(value: WireDTO<app.WeeklyDashboardDTO>): WeeklyDashboardDTO {
  return {
    ...value,
    days: value.days.map((day) => ({
      ...day, segments: day.segments.map((segment) => ({ ...segment, appSites: normalizeAppSites(segment.appSites) })),
    })),
  }
}
