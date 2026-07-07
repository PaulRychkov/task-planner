export function toISO(d: Date): string {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

export function fromISO(s: string): Date {
  const [y, m, d] = s.split('-').map(Number)
  return new Date(y, m - 1, d)
}

export function todayISO(): string {
  return toISO(new Date())
}

export function addDays(s: string, n: number): string {
  const d = fromISO(s)
  d.setDate(d.getDate() + n)
  return toISO(d)
}

export function startOfWeekISO(s: string): string {
  const d = fromISO(s)
  const shift = (d.getDay() + 6) % 7
  d.setDate(d.getDate() - shift)
  return toISO(d)
}

export function monthGrid(anchor: string): string[] {
  const d = fromISO(anchor)
  const first = new Date(d.getFullYear(), d.getMonth(), 1)
  const start = startOfWeekISO(toISO(first))
  const cells: string[] = []
  for (let i = 0; i < 42; i++) cells.push(addDays(start, i))
  return cells
}

export function sameMonth(a: string, b: string): boolean {
  return a.slice(0, 7) === b.slice(0, 7)
}

export function addMonths(s: string, n: number): string {
  const d = fromISO(s)
  const day = d.getDate()
  d.setDate(1)
  d.setMonth(d.getMonth() + n)
  const max = new Date(d.getFullYear(), d.getMonth() + 1, 0).getDate()
  d.setDate(Math.min(day, max))
  return toISO(d)
}

export function minutesToTime(min: number): string {
  const h = Math.floor(min / 60)
  const m = min % 60
  return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}`
}

export function timeToMinutes(t: string): number | null {
  const m = /^(\d{1,2}):(\d{2})$/.exec(t)
  if (!m) return null
  const v = Number(m[1]) * 60 + Number(m[2])
  return v >= 0 && v <= 1439 ? v : null
}

export const WEEKDAYS_SHORT = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс']

export const MONTHS_RU = [
  'Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь',
  'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь',
]

export function monthTitle(s: string): string {
  const d = fromISO(s)
  return `${MONTHS_RU[d.getMonth()]} ${d.getFullYear()}`
}

export const MONTHS_RU_GEN = [
  'января', 'февраля', 'марта', 'апреля', 'мая', 'июня',
  'июля', 'августа', 'сентября', 'октября', 'ноября', 'декабря',
]

export function humanDate(s: string): string {
  const d = fromISO(s)
  return `${d.getDate()} ${MONTHS_RU_GEN[d.getMonth()]} ${d.getFullYear()}`
}

export function weekdayOf(s: string): string {
  return WEEKDAYS_SHORT[(fromISO(s).getDay() + 6) % 7]
}

export function isPast(s: string): boolean {
  return s < todayISO()
}
