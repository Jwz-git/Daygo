import assert from 'node:assert/strict'
import test from 'node:test'

import type { TimelineCardDTO, TimelineDayDTO } from '../src/api/dto'
import {
  boxesOverlap,
  cardIntersectsRanges,
  coveredBy,
  layoutTimelineCards,
  MIN_CARD_HEIGHT,
  positionRange,
  uncoveredBy,
} from '../src/views/Timeline/layout'
import { buildWeekColumns } from '../src/views/Timeline/weekLayout'

/*
 * Reprocessing a day puts its batches back to pending while their cards are
 * still stored, so a card and a "generating" block claim the same window and
 * paint on top of each other. These fixtures pin the display rule that fixes
 * it: a window belongs to the card that covers it, and only the windows with
 * no card fall back to a block. Both boxes are the ones actually drawn, so the
 * minimum hit height counts as much as the timestamps do.
 */

const DAY_START = 1_700_000_000
const HEIGHT = 2400

function cardBox(startMinutes: number, endMinutes: number) {
  return positionRange(
    DAY_START + startMinutes * 60,
    DAY_START + endMinutes * 60,
    DAY_START,
    DAY_START + 24 * 60 * 60,
    HEIGHT,
    MIN_CARD_HEIGHT,
  )
}

test('boxes sharing vertical space overlap; neighbours that only touch do not', () => {
  assert.equal(boxesOverlap({ top: 100, height: 40 }, { top: 120, height: 40 }), true)
  assert.equal(boxesOverlap({ top: 130, height: 40 }, { top: 100, height: 40 }), true)
  assert.equal(boxesOverlap({ top: 100, height: 40 }, { top: 140, height: 40 }), false)
  assert.equal(boxesOverlap({ top: 100, height: 40 }, { top: 200, height: 40 }), false)
})

test('a window a card already covers loses its generating block', () => {
  const card = cardBox(600, 615)
  const block = positionRange(
    DAY_START + 600 * 60,
    DAY_START + 615 * 60,
    DAY_START,
    DAY_START + 24 * 60 * 60,
    HEIGHT,
    34,
  )

  assert.deepEqual(uncoveredBy([block], [card]), [])
  assert.deepEqual(coveredBy([card], [block]), [card])
})

test('a window with no card keeps its generating block', () => {
  const card = cardBox(600, 615)
  const later = positionRange(
    DAY_START + 660 * 60,
    DAY_START + 675 * 60,
    DAY_START,
    DAY_START + 24 * 60 * 60,
    HEIGHT,
    34,
  )

  assert.deepEqual(uncoveredBy([later], [card]), [later])
  assert.deepEqual(coveredBy([later], [card]), [])
})

/*
 * A four-minute card is drawn at MIN_CARD_HEIGHT, which reaches about ten
 * minutes further down the track than the card's own timestamps. The block for
 * the window right after it collides with that inflated box, so the overlap
 * test cannot look at timestamps alone.
 */
test('the minimum hit height, not just the timestamps, decides coverage', () => {
  const short = cardBox(600, 604)
  const nextWindow = positionRange(
    DAY_START + 608 * 60,
    DAY_START + 623 * 60,
    DAY_START,
    DAY_START + 24 * 60 * 60,
    HEIGHT,
    34,
  )
  assert.ok(short.top + short.height > nextWindow.top, 'fixture must collide on screen')

  assert.deepEqual(uncoveredBy([nextWindow], [short]), [])
})

test('a card packed into a second lane still covers its own window', () => {
  const cards = layoutTimelineCards(
    [
      { id: 1, startTs: DAY_START + 600 * 60, endTs: DAY_START + 608 * 60 },
      { id: 2, startTs: DAY_START + 606 * 60, endTs: DAY_START + 614 * 60 },
    ],
    DAY_START,
    DAY_START + 24 * 60 * 60,
    HEIGHT,
  )
  const block = positionRange(
    DAY_START + 600 * 60,
    DAY_START + 608 * 60,
    DAY_START,
    DAY_START + 24 * 60 * 60,
    HEIGHT,
    34,
  )

  const covered = coveredBy(cards, [block]).map((card) => card.id)
  assert.deepEqual(covered, [1, 2])
  assert.deepEqual(uncoveredBy([block], cards), [])
})

test('coverage keeps the order of the boxes it filters', () => {
  const ranges = [
    { top: 0, height: 10, id: 'a' },
    { top: 100, height: 10, id: 'b' },
    { top: 200, height: 10, id: 'c' },
  ]
  const cards = [
    { top: 100, height: 10 },
    { top: 205, height: 10 },
  ]

  assert.deepEqual(coveredBy(ranges, cards).map((range) => range.id), ['b', 'c'])
  assert.deepEqual(uncoveredBy(ranges, cards), [ranges[0]])
})

test('an empty day has nothing to cover', () => {
  const ranges = [{ top: 10, height: 20 }]
  assert.deepEqual(coveredBy(ranges, []), [])
  assert.deepEqual(uncoveredBy(ranges, []), ranges)
  assert.deepEqual(coveredBy([], ranges), [])
  assert.deepEqual(uncoveredBy([], ranges), [])
})

const WEEK_DAY_START = 1_700_000_000
const WEEK_DAY_SPAN = 24 * 60 * 60

function weekCard(id: number, startMinutes: number, endMinutes: number): TimelineCardDTO {
  return {
    id,
    batchId: null,
    day: '2026-09-16',
    start: '10:00 AM',
    end: '10:15 AM',
    startTs: WEEK_DAY_START + startMinutes * 60,
    endTs: WEEK_DAY_START + endMinutes * 60,
    category: 'Development',
    subcategory: '',
    title: `card ${id}`,
    summary: '',
    detailedSummary: '',
    videoSummaryUrl: null,
    otherVideoSummaryUrls: [],
    appSites: null,
    distractions: [],
    activityPoints: [],
    isIdle: false,
    durationMinutes: endMinutes - startMinutes,
  }
}

function weekDay(
  cards: TimelineCardDTO[],
  processingRanges: Array<{ startTs: number; endTs: number }>,
): TimelineDayDTO {
  return {
    day: '2026-09-16',
    dayStartTs: WEEK_DAY_START,
    dayEndTs: WEEK_DAY_START + WEEK_DAY_SPAN,
    cards,
    categories: [],
    trackedMinutes: 0,
    idleMinutes: 0,
    failures: [],
    processingRanges,
    generatedAtTs: WEEK_DAY_START,
  }
}

/* The week grid draws cards and generating blocks into the same column, so the
   day track's rule has to hold there too: the covered window keeps its card and
   loses its block, the empty one keeps the block. */
test('the week column applies the same window ownership as the day track', () => {
  const covered = { startTs: WEEK_DAY_START + 600 * 60, endTs: WEEK_DAY_START + 615 * 60 }
  const empty = { startTs: WEEK_DAY_START + 720 * 60, endTs: WEEK_DAY_START + 735 * 60 }
  const format = new Intl.DateTimeFormat('en-US', { hour: '2-digit', minute: '2-digit' })

  const [column] = buildWeekColumns(
    [weekDay([weekCard(1, 600, 615), weekCard(2, 660, 675)], [covered, empty])],
    null,
    format,
  )

  assert.deepEqual(column.cards.map((card) => [card.id, card.regenerating]), [[1, true], [2, false]])
  assert.equal(column.processing.length, 1)
  assert.ok(
    boxesOverlap(column.processing[0], {
      top: column.cards[1].top,
      height: column.cards[1].height,
    }) === false,
    'the surviving block must not sit on the card that kept its own position',
  )
})

test('an adjacent card below a regenerating card does not become regenerating', () => {
  // Card 1 is a 4-minute card (600..604) that gets stretched to MIN_CARD_HEIGHT (34px)
  // Card 2 is immediately adjacent or begins at 615 (615..630)
  // Batch covers 600..615
  const card1 = { startTs: DAY_START + 600 * 60, endTs: DAY_START + 604 * 60 }
  const card2 = { startTs: DAY_START + 615 * 60, endTs: DAY_START + 630 * 60 }
  const batch = [{ startTs: DAY_START + 600 * 60, endTs: DAY_START + 615 * 60 }]

  assert.equal(cardIntersectsRanges(card1, batch), true)
  assert.equal(cardIntersectsRanges(card2, batch), false)

  // Week column with consecutive cards
  const format = new Intl.DateTimeFormat('en-US', { hour: '2-digit', minute: '2-digit' })
  const [column] = buildWeekColumns(
    [weekDay([weekCard(1, 600, 615), weekCard(2, 615, 630)], [batch[0]])],
    null,
    format,
  )
  assert.deepEqual(column.cards.map((c) => [c.id, c.regenerating]), [[1, true], [2, false]])
})
