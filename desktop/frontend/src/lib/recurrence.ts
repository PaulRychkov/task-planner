import type { RecurrenceKind, RecurrenceParams, Task } from '../types'
import { WEEKDAYS_SHORT } from './dates'

export const KIND_LABELS: Record<RecurrenceKind, string> = {
  once: 'Один раз',
  daily: 'Ежедневно',
  weekdays: 'Пн–Пт (будни)',
  weekends: 'Сб–Вс (выходные)',
  days_of_week: 'Дни недели',
  every_n_days: 'Каждые N дней',
  every_n_weeks: 'Каждые N недель',
  monthly: 'Раз в месяц',
  spaced_repetition: 'Закрепление (SR)',
}

export const KIND_ORDER: RecurrenceKind[] = [
  'once', 'daily', 'weekdays', 'weekends', 'days_of_week',
  'every_n_days', 'every_n_weeks', 'monthly', 'spaced_repetition',
]

export interface SRPreset {
  label: string
  intervals: number[]
}

export const SR_PRESETS: SRPreset[] = [
  { label: 'Закрепление: 0→1→3→7→14→30→90', intervals: [0, 1, 3, 7, 14, 30, 90] },
  { label: 'Ebbinghaus: 0→1→3→7→14→30→60→90→180→365', intervals: [0, 1, 3, 7, 14, 30, 60, 90, 180, 365] },
  { label: 'Интенсив: 0→1→2→4→7→14→21→30→60→90', intervals: [0, 1, 2, 4, 7, 14, 21, 30, 60, 90] },
  { label: 'Anki: 0→1→4→10→25→60→150', intervals: [0, 1, 4, 10, 25, 60, 150] },
]

export const DURATIONS: { label: string; minutes: number }[] = [
  { label: '5 мин', minutes: 5 },
  { label: '10 мин', minutes: 10 },
  { label: '15 мин', minutes: 15 },
  { label: '20 мин', minutes: 20 },
  { label: '25 мин', minutes: 25 },
  { label: '30 мин', minutes: 30 },
  { label: '45 мин', minutes: 45 },
  { label: '1 ч', minutes: 60 },
  { label: '1.5 ч', minutes: 90 },
  { label: '2 ч', minutes: 120 },
  { label: '2.5 ч', minutes: 150 },
  { label: '3 ч', minutes: 180 },
  { label: '4 ч', minutes: 240 },
  { label: '5 ч', minutes: 300 },
]

export const START_TIMES: number[] = []
for (let m = 6 * 60; m <= 23 * 60 + 30; m += 30) START_TIMES.push(m)

export function describeRecurrence(task: Task): string {
  const p: RecurrenceParams = task.recurrence_params ?? {}
  switch (task.recurrence_kind) {
    case 'once':
      return 'Один раз'
    case 'daily':
      return 'Ежедневно'
    case 'weekdays':
      return 'Будни'
    case 'weekends':
      return 'Выходные'
    case 'days_of_week':
      return (p.days ?? []).map((d) => WEEKDAYS_SHORT[d - 1]).join(' / ')
    case 'every_n_days':
      return p.n === 2 ? 'Через день' : `Каждые ${p.n} дн.`
    case 'every_n_weeks':
      return p.n === 1 ? 'Раз в неделю' : `Раз в ${p.n} нед.`
    case 'monthly':
      return p.day_of_month ? `Ежемесячно, ${p.day_of_month} числа` : 'Раз в месяц'
    case 'spaced_repetition':
      return `SR: ${(p.intervals ?? []).join('→')}`
  }
}

export function paramsRequired(kind: RecurrenceKind): boolean {
  return !['once', 'daily', 'weekdays', 'weekends'].includes(kind)
}
