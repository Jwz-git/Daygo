import assert from 'node:assert/strict'
import test from 'node:test'

import type { CategoryDTO, TimelineCardDTO, TimelineDayDTO } from '../src/api/dto'
import {
  buildDailyPresentation,
  isDistractionCategoryKey,
  parseClockToMinutes,
} from '../src/stores/daily'

test('parseClockToMinutes parses 12-hour and 24-hour clock strings', () => {
  assert.equal(parseClockToMinutes('9:30 AM'), 570)
  assert.equal(parseClockToMinutes('09:30AM'), 570)
  assert.equal(parseClockToMinutes('1:15 PM'), 795)
  assert.equal(parseClockToMinutes('12:00 AM'), 0)
  assert.equal(parseClockToMinutes('12:30 PM'), 750)
  assert.equal(parseClockToMinutes('10:36'), 636)
  assert.equal(parseClockToMinutes('14:20'), 860)
  assert.equal(parseClockToMinutes('invalid'), null)
  assert.equal(parseClockToMinutes(''), null)
})

test('isDistractionCategoryKey matches distraction variations case-insensitively', () => {
  assert.equal(isDistractionCategoryKey('Distraction'), true)
  assert.equal(isDistractionCategoryKey('distractions'), true)
  assert.equal(isDistractionCategoryKey('  DISTRACTION  '), true)
  assert.equal(isDistractionCategoryKey('Work'), false)
  assert.equal(isDistractionCategoryKey('Coding'), false)
  assert.equal(isDistractionCategoryKey(''), false)
})

function makeCategory(name: string, isIdle = false): CategoryDTO {
  return {
    id: `cat-${name.toLowerCase()}`,
    name,
    colorHex: isDistractionCategoryKey(name) ? '#FF5950' : '#4F8CFF',
    details: '',
    sortOrder: 1,
    isSystem: false,
    isIdle,
    createdAtTs: 0,
    updatedAtTs: 0,
  }
}

function makeCard(partial: Partial<TimelineCardDTO> & { start: string; end: string; startTs: number; endTs: number }): TimelineCardDTO {
  return {
    id: 1,
    batchId: 1,
    day: '2026-09-18',
    category: 'Work',
    subcategory: '',
    title: 'Test Card',
    summary: 'Test summary',
    detailedSummary: '',
    videoSummaryUrl: null,
    otherVideoSummaryUrls: [],
    appSites: null,
    distractions: [],
    activityPoints: [],
    isIdle: false,
    durationMinutes: Math.round((partial.endTs - partial.startTs) / 60),
    ...partial,
  }
}

test('buildDailyPresentation collects macro distractions when Distraction category exists', () => {
  const dayStart = 1_700_000_000 // e.g. 4:00 AM
  const dayEnd = dayStart + 24 * 3600
  const day: TimelineDayDTO = {
    day: '2026-09-18',
    dayStartTs: dayStart,
    dayEndTs: dayEnd,
    categories: [makeCategory('Work'), makeCategory('Distraction')],
    cards: [
      makeCard({
        id: 1,
        category: 'Work',
        title: 'Deep Coding',
        start: '10:00 AM',
        end: '11:00 AM',
        startTs: dayStart + 6 * 3600, // 10:00
        endTs: dayStart + 7 * 3600, // 11:00
      }),
      makeCard({
        id: 2,
        category: 'Distraction',
        title: 'Browsing Social Feed',
        start: '11:00 AM',
        end: '11:30 AM',
        startTs: dayStart + 7 * 3600, // 11:00
        endTs: dayStart + 7.5 * 3600, // 11:30
      }),
    ],
  }

  const presentation = buildDailyPresentation(day)

  assert.equal(presentation.hasDistractionCategory, true)
  assert.equal(presentation.distractionMarkers.length, 1)
  assert.equal(presentation.distractionMarkers[0]!.title, 'Browsing Social Feed')
  assert.equal(presentation.distractionMarkers[0]!.startTs, dayStart + 7 * 3600)
  assert.equal(presentation.distractionMarkers[0]!.endTs, dayStart + 7.5 * 3600)
  assert.equal(presentation.distractionMarkers[0]!.durationMinutes, 30)

  // Distraction category counts toward distractedMinutes
  assert.equal(presentation.metrics.focusedMinutes, 60)
  assert.equal(presentation.metrics.distractedMinutes, 30)
  assert.equal(presentation.metrics.interruptions, 0)
})

test('buildDailyPresentation collects and anchors mini distractions', () => {
  const dayStart = 1_700_000_000
  const dayEnd = dayStart + 24 * 3600
  const day: TimelineDayDTO = {
    day: '2026-09-18',
    dayStartTs: dayStart,
    dayEndTs: dayEnd,
    categories: [makeCategory('Coding'), makeCategory('Distraction')],
    cards: [
      makeCard({
        id: 1,
        category: 'Coding',
        title: 'Feature Work',
        start: '10:00 AM',
        end: '11:00 AM',
        startTs: dayStart + 6 * 3600, // 10:00
        endTs: dayStart + 7 * 3600, // 11:00
        distractions: [
          {
            id: 'd1',
            startTime: '10:20 AM',
            endTime: '10:24 AM',
            title: 'Checked Messages',
            summary: 'Quick reply',
            videoSummaryUrl: null,
          },
        ],
      }),
    ],
  }

  const presentation = buildDailyPresentation(day)

  assert.equal(presentation.hasDistractionCategory, true)
  assert.equal(presentation.distractionMarkers.length, 1)
  assert.equal(presentation.distractionMarkers[0]!.title, 'Checked Messages')
  assert.equal(presentation.distractionMarkers[0]!.startTs, dayStart + 6 * 3600 + 20 * 60)
  assert.equal(presentation.distractionMarkers[0]!.endTs, dayStart + 6 * 3600 + 24 * 60)
  assert.equal(presentation.distractionMarkers[0]!.durationMinutes, 4)

  // Interruptions count increments
  assert.equal(presentation.metrics.interruptions, 1)
  assert.equal(presentation.metrics.focusedMinutes, 60)
})

test('buildDailyPresentation merges overlapping or adjacent distractions within 2 minutes', () => {
  const dayStart = 1_700_000_000
  const dayEnd = dayStart + 24 * 3600
  const day: TimelineDayDTO = {
    day: '2026-09-18',
    dayStartTs: dayStart,
    dayEndTs: dayEnd,
    categories: [makeCategory('Coding'), makeCategory('Distraction')],
    cards: [
      makeCard({
        id: 1,
        category: 'Coding',
        title: 'Debugging',
        start: '2:00 PM',
        end: '3:00 PM',
        startTs: dayStart + 10 * 3600,
        endTs: dayStart + 11 * 3600,
        distractions: [
          {
            id: 'd1',
            startTime: '2:15 PM',
            endTime: '2:18 PM',
            title: 'Twitter',
            summary: '',
            videoSummaryUrl: null,
          },
          {
            id: 'd2',
            startTime: '2:19 PM', // 1 minute after d1 ends (<= 2 min threshold)
            endTime: '2:22 PM',
            title: 'YouTube',
            summary: '',
            videoSummaryUrl: null,
          },
        ],
      }),
    ],
  }

  const presentation = buildDailyPresentation(day)

  assert.equal(presentation.distractionMarkers.length, 1)
  assert.equal(presentation.distractionMarkers[0]!.title, 'Twitter, YouTube')
  assert.equal(presentation.distractionMarkers[0]!.startTs, dayStart + 10 * 3600 + 15 * 60)
  assert.equal(presentation.distractionMarkers[0]!.endTs, dayStart + 10 * 3600 + 22 * 60)
  assert.equal(presentation.distractionMarkers[0]!.durationMinutes, 7)
})

test('buildDailyPresentation suppresses distraction markers when user has no Distraction category', () => {
  const dayStart = 1_700_000_000
  const dayEnd = dayStart + 24 * 3600
  const day: TimelineDayDTO = {
    day: '2026-09-18',
    dayStartTs: dayStart,
    dayEndTs: dayEnd,
    categories: [makeCategory('Coding'), makeCategory('Personal')], // No Distraction category
    cards: [
      makeCard({
        id: 1,
        category: 'Coding',
        title: 'Project',
        start: '10:00 AM',
        end: '11:00 AM',
        startTs: dayStart + 6 * 3600,
        endTs: dayStart + 7 * 3600,
        distractions: [
          {
            id: 'd1',
            startTime: '10:15 AM',
            endTime: '10:18 AM',
            title: 'Reddit',
            summary: '',
            videoSummaryUrl: null,
          },
        ],
      }),
    ],
  }

  const presentation = buildDailyPresentation(day)

  assert.equal(presentation.hasDistractionCategory, false)
  assert.equal(presentation.distractionMarkers.length, 0)
})
