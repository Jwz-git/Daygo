import { GetCardMedia } from '../../wailsjs/go/app/Backend'

import { WAILS_UNAVAILABLE } from '@/api/settings'
import type { CardMediaDTO } from '@/api/dto'

function hasBridge(): boolean {
  return (window as { go?: unknown }).go !== undefined
}

/**
 * Frame listing covering a card's timespan: numeric screenshot IDs the player
 * turns into `/media/frame?id=` requests. The backend samples to a cap, so a
 * long card never produces an oversized response.
 */
export async function getCardMedia(cardID: number): Promise<CardMediaDTO> {
  if (hasBridge()) return (await GetCardMedia(cardID)) as unknown as CardMediaDTO
  throw new Error(WAILS_UNAVAILABLE)
}
