import assert from 'node:assert/strict'
import test from 'node:test'

import { getSettings } from '../src/api/settings'
import { listProviders } from '../src/api/providers'
import { getPlanDay } from '../src/api/plan'
import { getCardRating, getCardVerdict } from '../src/api/review'
import { normalizeChatMessage, normalizeJournal, normalizeRecap, normalizeTimelineDay, normalizeWeeklyDashboard } from '../src/api/normalizeDTO'

const settings = {
  capture: { intervalSeconds: 10, captureHeight: 1080 }, privacy: { blockedApplicationIds: [] },
  storage: { recordingsLimitBytes: 0 }, appearance: { theme: 'system', language: '' },
  llm: { showTokenUsage: false }, chat: { memory: '', editMode: 'readonly' },
  notifications: { journalReminderEnabled: false, journalReminderTime: '18:00' },
  system: { launchAtLogin: false, showDockIcon: true, agentEditsEnabled: false, testToolsEnabled: false },
  telemetry: { analyticsOptIn: false, crashReportingOptIn: false },
}

async function withBackend(backend: Record<string, unknown>, run: () => Promise<void>): Promise<void> {
  const previousWindow = globalThis.window
  globalThis.window = { go: { app: { Backend: backend } } } as unknown as Window & typeof globalThis
  try { await run() } finally { globalThis.window = previousWindow }
}

test('settings boundary rejects unknown themes, languages and edit modes', async () => {
  for (const patch of [
    { appearance: { theme: 'invented', language: '' } },
    { appearance: { theme: 'system', language: 'invented' } },
    { chat: { memory: '', editMode: 'invented' } },
  ]) {
    await withBackend({ GetSettings: async () => ({ ...settings, ...patch }) }, async () => {
      await assert.rejects(getSettings(), /invalid_binding_payload/)
    })
  }
})

test('provider boundary rejects an unknown protocol without exposing payload text', async () => {
  await withBackend({ ListProviders: async () => [{
    id: 'anonymous', displayName: 'Anonymous', protocol: 'anonymous-private-value', endpoint: '',
    models: ['anonymous'], maxImages: 0, userAgent: '', hasSecret: false,
  }] }, async () => {
    await assert.rejects(listProviders(), (error: unknown) =>
      error instanceof Error && error.message === 'invalid_binding_payload: protocol')
  })
})

test('plan boundary normalizes nullable fields and rejects unknown statuses', async () => {
  const block = {
    id: 1, day: '2026-10-05', start: '09:00', end: '10:00', startTs: 100, endTs: 3700,
    title: 'Anonymous', categoryId: '', categoryName: '', colorHex: '', status: 'planned',
    remind: false, matchedMinutes: 0, distractionMinutes: 0,
  }
  await withBackend({ GetPlanDay: async () => ({ day: block.day, blocks: [block] }) }, async () => {
    const result = await getPlanDay(block.day)
    assert.equal(result.blocks[0]?.notes, null)
    assert.equal(result.blocks[0]?.completedAtTs, null)
    block.status = 'invented'
    await assert.rejects(getPlanDay(block.day), /invalid_binding_payload: status/)
  })
})

test('chat enum guards retain tool roles and reject unknown role or status', () => {
  const row = { id: 1, role: 'tool_call', content: '', status: '', errorCode: '', toolName: 'plan', toolArguments: '{}', createdAt: 1 }
  assert.equal(normalizeChatMessage(row).role, 'tool_call')
  assert.throws(() => normalizeChatMessage({ ...row, role: 'invented' }), /invalid_binding_payload: role/)
  assert.throws(() => normalizeChatMessage({ ...row, status: 'invented' }), /invalid_binding_payload: status/)
})

test('absent pointer fields mean no value, while zero timestamps remain zero', () => {
  const journal = normalizeJournal({ day: '2026-10-05', status: '' })
  assert.equal(journal.notes, null)
  assert.equal(journal.updatedAtTs, null)
  const recap = { standupDay: '2026-10-05', highlightsTitle: '', highlights: [], tasksTitle: '', tasks: [], blockersTitle: '', blockersBody: '' }
  assert.equal(normalizeRecap(recap).generatedAtTs, null)
  assert.equal(normalizeRecap({ ...recap, generatedAtTs: 0 }).generatedAtTs, 0)
  const timeline = normalizeTimelineDay({
    day: '2026-10-05', dayStartTs: 1, dayEndTs: 2, trackedMinutes: 1, idleMinutes: 0,
    failures: [], processingRanges: [], categories: [], generatedAtTs: 1,
    cards: [{ id: 1, day: '2026-10-05', start: '09:00', end: '09:01', startTs: 1, endTs: 61,
      category: 'Work', subcategory: '', title: '', summary: '', detailedSummary: '',
      otherVideoSummaryUrls: [], distractions: [], activityPoints: [], isIdle: false, durationMinutes: 1 }],
  })
  assert.equal(timeline.cards[0]?.batchId, null)
  assert.equal(timeline.cards[0]?.appSites, null)
  const weekly = normalizeWeeklyDashboard({
    weekStart: '2026-10-05', weekStartTs: 1, weekEndTs: 2, trackedMinutes: 1, focusMinutes: 1,
    categories: [], days: [{ day: '2026-10-05', trackedMinutes: 1, focusMinutes: 1, categories: [],
      segments: [{ startTs: 1, endTs: 61, category: 'Work', isIdle: false, distractions: [], appSites: { primary: 'Anonymous' } }] }],
    insights: { longestFocusMinutes: 1, longestFocusDay: '', peakHour: -1, peakHourMinutes: 0,
      mostActiveDay: '', mostActiveDayMinutes: 0, activeDays: 1, avgDailyFocusMinutes: 1 },
  })
  assert.deepEqual(weekly.days[0]?.segments[0]?.appSites, { primary: 'Anonymous', secondary: null })
})

test('review read boundaries distinguish no verdict from an unknown value', async () => {
  let value = ''
  await withBackend({ GetCardVerdict: async () => value, GetCardRating: async () => value }, async () => {
    assert.equal(await getCardVerdict(1), null)
    assert.equal(await getCardRating(1), null)
    value = 'invented'
    await assert.rejects(getCardVerdict(1), /invalid_binding_payload: verdict/)
    await assert.rejects(getCardRating(1), /invalid_binding_payload: rating/)
  })
})
