/*
 * Session-local review statistics shared between the review flow and the
 * inspector's "你的回顾" panel. Judgments are not persisted backend-side yet,
 * so these reset with the page — the panel shows zeros until a review runs.
 */
export interface ReviewTotals {
  distractionMinutes: number
  neutralMinutes: number
  focusMinutes: number
  /** Cards with a stored verdict; the queue excludes them. */
  reviewedCardIds?: number[]
}

export const ZERO_REVIEW_TOTALS: ReviewTotals = {
  distractionMinutes: 0,
  neutralMinutes: 0,
  focusMinutes: 0,
}
