import {
  ClearCardReview,
  GetReviewTotals,
  SaveCardReview,
} from '../../wailsjs/go/app/Backend'

import { WAILS_UNAVAILABLE } from '@/api/settings'
import type { ReviewTotals } from '@/views/Timeline/review'

function hasBridge(): boolean {
  return (window as { go?: unknown }).go !== undefined
}

/** Judged minutes per verdict for one logical day; zeros when never reviewed. */
export async function getReviewTotals(day: string): Promise<ReviewTotals> {
  if (!hasBridge()) throw new Error(WAILS_UNAVAILABLE)
  return (await GetReviewTotals(day)) as unknown as ReviewTotals
}

/** Record (or overwrite) the verdict for one card; minutes snapshot server-side. */
export async function saveCardReview(cardID: number, verdict: 'distraction' | 'neutral' | 'focus'): Promise<void> {
  if (!hasBridge()) throw new Error(WAILS_UNAVAILABLE)
  await SaveCardReview(cardID, verdict)
}

/** Remove the verdict for one card (撤销). */
export async function clearCardReview(cardID: number): Promise<void> {
  if (!hasBridge()) throw new Error(WAILS_UNAVAILABLE)
  await ClearCardReview(cardID)
}
