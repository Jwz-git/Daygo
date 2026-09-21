import assert from 'node:assert/strict'
import test from 'node:test'

import type { TimelineCardDTO, TimelineDayDTO } from '../src/api/dto'
import {
  boxesOverlap,
  cardBox,
  cardIntersectsRanges,
  cardIsCompact,
  cardShowsText,
  CARD_COMPACT_MINUTES,
  CARD_GAP,
  CARD_TEXT_MINUTES,
  coveredBy,
  isRegenerating,
  layoutTimelineCards,
  MIN_CARD_HEIGHT,
  PIXELS_PER_MINUTE,
  positionRange,
  resolveDisplaySpans,
  trackHeight,
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
const DAY_END = DAY_START + 24 * 60 * 60
const HEIGHT = 2400

function boxFor(startMinutes: number, endMinutes: number) {
  return cardBox(
    DAY_START + startMinutes * 60,
    DAY_START + endMinutes * 60,
    DAY_START,
    DAY_END,
    HEIGHT,
  )
}

test('boxes sharing vertical space overlap; neighbours that only touch do not', () => {
  assert.equal(boxesOverlap({ top: 100, height: 40 }, { top: 120, height: 40 }), true)
  assert.equal(boxesOverlap({ top: 130, height: 40 }, { top: 100, height: 40 }), true)
  assert.equal(boxesOverlap({ top: 100, height: 40 }, { top: 140, height: 40 }), false)
  assert.equal(boxesOverlap({ top: 100, height: 40 }, { top: 200, height: 40 }), false)
})

test('a window a card already covers loses its generating block', () => {
  const card = boxFor(600, 615)
  const block = positionRange(
    DAY_START + 600 * 60,
    DAY_START + 615 * 60,
    DAY_START,
    DAY_END,
    HEIGHT,
    34,
  )

  assert.deepEqual(uncoveredBy([block], [card]), [])
  assert.deepEqual(coveredBy([card], [block]), [card])
})

test('a window with no card keeps its generating block', () => {
  const card = boxFor(600, 615)
  const later = positionRange(
    DAY_START + 660 * 60,
    DAY_START + 675 * 60,
    DAY_START,
    DAY_END,
    HEIGHT,
    34,
  )

  assert.deepEqual(uncoveredBy([later], [card]), [later])
  assert.deepEqual(coveredBy([later], [card]), [])
})

/*
 * A card is drawn at its own minutes (floored at MIN_CARD_HEIGHT), so a short
 * one no longer reaches into the window below it. Before, a four-minute card
 * was inflated to 34px and swallowed the block of the next window — the block
 * vanished even though no card covered it.
 */
test('a short card stays inside its own minutes', () => {
  const short = boxFor(600, 604)
  const nextWindow = positionRange(
    DAY_START + 608 * 60,
    DAY_START + 623 * 60,
    DAY_START,
    DAY_END,
    HEIGHT,
    34,
  )
  assert.ok(short.top + short.height < nextWindow.top, 'fixture must not reach the next window')

  assert.deepEqual(uncoveredBy([nextWindow], [short]), [nextWindow])
})

test('cards that overlap in time are trimmed, never split into lanes', () => {
  const cards = layoutTimelineCards(
    [
      { id: 1, startTs: DAY_START + 600 * 60, endTs: DAY_START + 608 * 60 },
      { id: 2, startTs: DAY_START + 606 * 60, endTs: DAY_START + 614 * 60 },
    ],
    DAY_START,
    DAY_END,
    HEIGHT,
  )
  const block = positionRange(
    DAY_START + 600 * 60,
    DAY_START + 608 * 60,
    DAY_START,
    DAY_END,
    HEIGHT,
    34,
  )

  // The later card gives up the shared minutes (606–608) instead of moving to
  // a second column, so every row spans the full width.
  assert.deepEqual(cards.map((card) => [card.id, card.minutes]), [
    [1, 8],
    [2, 6],
  ])
  assert.equal(boxesOverlap(cards[0], cards[1]), false)

  const covered = coveredBy(cards, [block]).map((card) => card.id)
  assert.deepEqual(covered, [1, 2])
  assert.deepEqual(uncoveredBy([block], cards), [])
})

/*
 * The reported case: three consecutive short activities (5 / 7 / 5 minutes) at
 * the real day scale. Their drawn boxes used to overlap only because each was
 * inflated to a 34px minimum hit height, which packed them into three
 * one-third-width columns with truncated titles.
 */
test('consecutive short activities each get their own full-width row', () => {
  const dayHeight = trackHeight(DAY_START, DAY_END)
  const lengths = [5, 7, 5]
  let cursor = 600
  const cards = lengths.map((length, index) => {
    const card = {
      id: index + 1,
      startTs: DAY_START + cursor * 60,
      endTs: DAY_START + (cursor + length) * 60,
    }
    cursor += length
    return card
  })

  const placed = layoutTimelineCards(cards, DAY_START, DAY_END, dayHeight)

  assert.deepEqual(placed.map((card) => card.id), [1, 2, 3])
  placed.forEach((card, index) => {
    assert.equal(card.minutes, lengths[index], 'a row keeps its own minutes')
    assert.equal(
      card.height,
      Math.max(MIN_CARD_HEIGHT, lengths[index] * PIXELS_PER_MINUTE - CARD_GAP),
      'height follows the card own minutes',
    )
    if (index > 0) {
      assert.equal(boxesOverlap(placed[index - 1], card), false, 'rows do not overlap')
    }
  })
  // Five to seven minutes is shorter than the row that can carry a title, so
  // these draw as bare bars rather than one-third-width truncated cards.
  assert.deepEqual(placed.map((card) => cardShowsText(card.minutes)), [false, false, false])
  assert.deepEqual(placed.map((card) => cardIsCompact(card.minutes)), [true, true, true])
})

test('a row carries text only from ten minutes up', () => {
  assert.equal(cardShowsText(5), false)
  assert.equal(cardShowsText(9.9), false)
  assert.equal(cardShowsText(CARD_TEXT_MINUTES), true)
  assert.equal(cardIsCompact(CARD_COMPACT_MINUTES - 0.1), true)
  assert.equal(cardIsCompact(CARD_COMPACT_MINUTES), false)
})

test('an overlap trims the longer card and drops one that has no time left', () => {
  const span = (id: number, startMinutes: number, endMinutes: number) => ({
    id,
    startTs: DAY_START + startMinutes * 60,
    endTs: DAY_START + endMinutes * 60,
  })
  const asMinutes = (spans: ReturnType<typeof resolveDisplaySpans>) =>
    spans.map((card) => [card.id, (card.startTs - DAY_START) / 60, (card.endTs - DAY_START) / 60])

  // The short card sits inside the long one: the long one loses its longer side.
  assert.deepEqual(asMinutes(resolveDisplaySpans([span(1, 600, 640), span(2, 620, 625)])), [
    [1, 600, 620],
    [2, 620, 625],
  ])
  // The short card starts first: the long one is pushed behind it.
  assert.deepEqual(asMinutes(resolveDisplaySpans([span(1, 600, 620), span(2, 600, 640)])), [
    [1, 600, 620],
    [2, 620, 640],
  ])
  // Identical spans are a duplicate, not two rows.
  assert.deepEqual(asMinutes(resolveDisplaySpans([span(1, 600, 620), span(2, 600, 620)])), [
    [1, 600, 620],
  ])
  // Disjoint neighbours are untouched.
  assert.deepEqual(asMinutes(resolveDisplaySpans([span(1, 600, 615), span(2, 615, 630)])), [
    [1, 600, 615],
    [2, 615, 630],
  ])
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

function weekCard(id: number, startMinutes: number, endMinutes: number, batchId: number | null = null): TimelineCardDTO {
  return {
    id,
    batchId,
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
  processingRanges: Array<{ startTs: number; endTs: number; batchIds?: number[] }>,
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
    processingRanges: processingRanges.map((range) => ({ batchIds: [], ...range })),
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

/*
 * Reprocessing a card whose batch merged backwards: the batch's window is the
 * later part, while the cards it produced (and will replace) reach further back.
 * Keying the state to the window alone left the card the user had open showing
 * nothing while the card below it showed the regenerating tint.
 */
test('cards a running batch owns show as regenerating even before its window', () => {
  const batch = { startTs: DAY_START + 616 * 60, endTs: DAY_START + 631 * 60, batchIds: [7] }
  const owned = { id: 1, batchId: 7, startTs: DAY_START + 600 * 60, endTs: DAY_START + 616 * 60 }
  const windowCard = { id: 2, batchId: 7, startTs: DAY_START + 616 * 60, endTs: DAY_START + 631 * 60 }
  const other = { id: 3, batchId: 9, startTs: DAY_START + 631 * 60, endTs: DAY_START + 646 * 60 }

  // The merged card only touches the window; the batch owns it all the same.
  assert.equal(cardIntersectsRanges(owned, [batch]), false)
  assert.equal(isRegenerating(owned, [batch]), true)
  assert.equal(isRegenerating(windowCard, [batch]), true)
  assert.equal(isRegenerating(other, [batch]), false)

  const format = new Intl.DateTimeFormat('en-US', { hour: '2-digit', minute: '2-digit' })
  const [column] = buildWeekColumns(
    [weekDay([weekCard(1, 600, 616, 7), weekCard(2, 616, 631, 7), weekCard(3, 631, 646, 9)], [batch])],
    null,
    format,
  )
  assert.deepEqual(column.cards.map((c) => [c.id, c.regenerating]), [[1, true], [2, true], [3, false]])
})

/*
 * A per-card regeneration rewrites the card's own window synchronously: no
 * batch ever goes pending, so processingRanges cannot carry it and only the
 * in-flight card id marks the card the user just pressed.
 */
test('the in-flight card id marks exactly that card in the week column', () => {
  const format = new Intl.DateTimeFormat('en-US', { hour: '2-digit', minute: '2-digit' })
  const [column] = buildWeekColumns(
    [weekDay([weekCard(1, 600, 630), weekCard(2, 630, 660)], [])],
    null,
    format,
    2,
  )

  assert.deepEqual(column.cards.map((c) => [c.id, c.regenerating]), [[1, false], [2, true]])
  assert.equal(column.processing.length, 0)
})
