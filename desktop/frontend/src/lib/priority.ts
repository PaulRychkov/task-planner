export const MIN_PRIORITY = 1
export const MAX_PRIORITY = 5

export const PRIORITY_LEVELS = [1, 2, 3, 4, 5]

export function priorityWeightPercent(priority: number): number {
  const p = Math.min(MAX_PRIORITY, Math.max(MIN_PRIORITY, priority))
  return 100 + (p - MIN_PRIORITY) * 50
}
