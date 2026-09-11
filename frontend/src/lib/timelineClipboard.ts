import type { TimelineCardDTO } from '@/api/dto'

export function formatTimelineForClipboard(day: string, cards: readonly TimelineCardDTO[]): string {
  const lines = cards
    .slice()
    .sort((left, right) => left.startTs - right.startTs || left.id - right.id)
    .flatMap((card) => {
      const header = `${card.start} – ${card.end}  ${card.title} · ${card.category}`
      const summary = card.summary.trim()
      return summary === '' ? [header] : [header, `  ${summary}`]
    })

  return [day, ...lines].join('\n')
}
