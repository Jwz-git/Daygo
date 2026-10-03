/*
 * Side-by-side lanes for overlapping plan blocks. Blocks are swept in start
 * order; each takes the first lane whose previous block has ended, and every
 * block of a connected overlap cluster reports the cluster's lane count, so
 * the cluster splits its width evenly while lone blocks keep the full width.
 * Pure, so the day track and the week grid share it and it is unit-tested.
 */
export interface PlanSpan {
  id: number
  startTs: number
  endTs: number
}

export interface PlanLane<T extends PlanSpan> {
  block: T
  /** Zero-based lane inside the cluster. */
  lane: number
  /** Lanes the block's cluster needs. */
  lanes: number
}

export function planLanes<T extends PlanSpan>(blocks: readonly T[]): PlanLane<T>[] {
  const sorted = [...blocks].sort((a, b) => a.startTs - b.startTs || a.endTs - b.endTs || a.id - b.id)
  const out: PlanLane<T>[] = []
  let cluster: PlanLane<T>[] = []
  let laneEnds: number[] = []
  let clusterEnd = -Infinity

  const flush = (): void => {
    for (const entry of cluster) entry.lanes = laneEnds.length
    out.push(...cluster)
    cluster = []
    laneEnds = []
  }

  for (const block of sorted) {
    if (block.startTs >= clusterEnd) flush()
    let lane = laneEnds.findIndex((end) => end <= block.startTs)
    if (lane === -1) {
      lane = laneEnds.length
      laneEnds.push(block.endTs)
    } else {
      laneEnds[lane] = block.endTs
    }
    cluster.push({ block, lane, lanes: 0 })
    clusterEnd = Math.max(clusterEnd, block.endTs)
  }
  flush()
  return out
}

/** Where a block stands relative to now, for badges and styling. */
export type PlanPhase = 'upcoming' | 'active' | 'missed' | 'done' | 'skipped'

export function planPhase(
  block: { status: string; startTs: number; endTs: number },
  nowTs: number,
): PlanPhase {
  if (block.status === 'done') return 'done'
  if (block.status === 'skipped') return 'skipped'
  if (nowTs < block.startTs) return 'upcoming'
  return nowTs < block.endTs ? 'active' : 'missed'
}
