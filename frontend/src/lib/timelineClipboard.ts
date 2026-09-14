import type { TimelineCardDTO } from '@/api/dto'
import { categoryLabel } from '@/lib/categoryLabel'

export function formatTimelineForClipboard(
  day: string,
  cards: readonly TimelineCardDTO[],
  translate: (key: string) => string,
): string {
  const lines = cards
    .slice()
    .sort((left, right) => left.startTs - right.startTs || left.id - right.id)
    .flatMap((card) => {
      const header = `${card.start} – ${card.end}  ${card.title} · ${categoryLabel(card.category, translate)}`
      const summary = card.summary.trim()
      return summary === '' ? [header] : [header, `  ${summary}`]
    })

  return [day, ...lines].join('\n')
}
