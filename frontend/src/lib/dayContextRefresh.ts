// delayUntilDayContextRefresh schedules a refresh from a boundary supplied by
// the backend. The frontend must not derive a logical day itself: the backend
// owns the 04:00 boundary and any zone/DST behaviour.
export function delayUntilDayContextRefresh(dayEndTs: number, nowMs = Date.now()): number {
  return Math.max(0, dayEndTs * 1000 - nowMs)
}
