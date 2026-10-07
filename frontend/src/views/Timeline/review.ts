import type { ReviewTotalsDTO } from '@/api/dto'

/*
 * Persisted review statistics shared between the review flow and the
 * inspector's "你的回顾" panel. The backend returns the day totals and the
 * reviewed-card ids; the frontend only caches that authoritative snapshot.
 */
export type ReviewVerdict = 'distraction' | 'neutral' | 'focus'

/**
 * A thumbs up/down on one card's AI-written summary. Feedback on the summary
 * text only: it never rewrites the summary or the card's category.
 */
export type SummaryRating = 'up' | 'down'

export type ReviewTotals = Omit<ReviewTotalsDTO, 'reviewedCardIds'> & Partial<Pick<ReviewTotalsDTO, 'reviewedCardIds'>>

export const ZERO_REVIEW_TOTALS: ReviewTotals = {
  distractionMinutes: 0,
  neutralMinutes: 0,
  focusMinutes: 0,
}
