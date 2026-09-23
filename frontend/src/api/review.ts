import {
  ClearCardRating,
  ClearCardReview,
  GetCardRating,
  GetCardVerdict,
  GetReviewTotals,
  SaveCardRating,
  SaveCardReview,
} from '../../wailsjs/go/app/Backend'

import { WAILS_UNAVAILABLE } from '@/api/settings'
import type { ReviewTotals, ReviewVerdict, SummaryRating } from '@/views/Timeline/review'

function hasBridge(): boolean {
  return (window as { go?: unknown }).go !== undefined
}

/** Judged minutes per verdict for one logical day; zeros when never reviewed. */
export async function getReviewTotals(day: string): Promise<ReviewTotals> {
  if (!hasBridge()) throw new Error(WAILS_UNAVAILABLE)
  return (await GetReviewTotals(day)) as unknown as ReviewTotals
}

/** Record (or overwrite) the verdict for one card; minutes snapshot server-side. */
export async function saveCardReview(cardID: number, verdict: ReviewVerdict): Promise<void> {
  if (!hasBridge()) throw new Error(WAILS_UNAVAILABLE)
  await SaveCardReview(cardID, verdict)
}

/** Stored verdict for one card, or null when it has not been judged. */
export async function getCardVerdict(cardID: number): Promise<ReviewVerdict | null> {
  if (!hasBridge()) throw new Error(WAILS_UNAVAILABLE)
  const verdict = await GetCardVerdict(cardID)
  return verdict === '' ? null : (verdict as ReviewVerdict)
}

/** Remove the verdict for one card (撤销). */
export async function clearCardReview(cardID: number): Promise<void> {
  if (!hasBridge()) throw new Error(WAILS_UNAVAILABLE)
  await ClearCardReview(cardID)
}

/** Record (or overwrite) the thumbs up/down on one card's summary. */
export async function saveCardRating(cardID: number, rating: SummaryRating): Promise<void> {
  if (!hasBridge()) throw new Error(WAILS_UNAVAILABLE)
  await SaveCardRating(cardID, rating)
}

/** Stored summary rating for one card, or null when it has not been rated. */
export async function getCardRating(cardID: number): Promise<SummaryRating | null> {
  if (!hasBridge()) throw new Error(WAILS_UNAVAILABLE)
  const rating = await GetCardRating(cardID)
  return rating === '' ? null : (rating as SummaryRating)
}

/** Remove the summary rating for one card (tapping the active thumb again). */
export async function clearCardRating(cardID: number): Promise<void> {
  if (!hasBridge()) throw new Error(WAILS_UNAVAILABLE)
  await ClearCardRating(cardID)
}
