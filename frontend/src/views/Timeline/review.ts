/*
 * Persisted review statistics shared between the review flow and the
 * inspector's "你的回顾" panel. The backend returns the day totals and the
 * reviewed-card ids; the frontend only caches that authoritative snapshot.
 */
export type ReviewVerdict = 'distraction' | 'neutral' | 'focus'

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
