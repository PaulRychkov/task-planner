import { useCallback, useEffect, useMemo, useState } from 'react'
import { ChevronLeft, ChevronRight, FastForward } from 'lucide-react'
import { api } from '../api'
import type { Occurrence } from '../types'
import {
  addDays, addMonths, humanDate, isPast, minutesToTime, monthGrid, monthTitle,
  sameMonth, startOfWeekISO, todayISO, WEEKDAYS_SHORT, weekdayOf,
} from '../lib/dates'
import OccurrenceChip, { statusStyles } from './OccurrenceChip'

type Mode = 'month' | 'week' | 'day'

const MODE_LABELS: Record<Mode, string> = { month: 'Месяц', week: 'Неделя', day: 'День' }

const RESCHEDULE_LOOKBACK_DAYS = 60

interface Props {
  onDataChanged: () => Promise<void> | void
}

export default function CalendarView({ onDataChanged }: Props) {
  const [mode, setMode] = useState<Mode>('month')
  const [anchor, setAnchor] = useState(todayISO())
  const [occs, setOccs] = useState<Occurrence[]>([])
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState<string | null>(null)
  const [popupDate, setPopupDate] = useState<string | null>(null)

  const range = useMemo((): [string, string] => {
    if (mode === 'month') {
      const cells = monthGrid(anchor)
      return [cells[0], cells[41]]
    }
    if (mode === 'week') {
      const start = startOfWeekISO(anchor)
      return [start, addDays(start, 6)]
    }
    return [anchor, anchor]
  }, [mode, anchor])

  const load = useCallback(async () => {
    try {
      setOccs(await api.listOccurrences(range[0], range[1]))
    } catch {
      setOccs([])
    }
  }, [range])

  useEffect(() => {
    void load()
  }, [load])

  const byDate = useMemo(() => {
    const map = new Map<string, Occurrence[]>()
    for (const o of occs) {
      const list = map.get(o.date) ?? []
      list.push(o)
      map.set(o.date, list)
    }
    for (const list of map.values()) {
      list.sort((a, b) => {
        const ta = a.task?.start_time_minutes ?? 24 * 60
        const tb = b.task?.start_time_minutes ?? 24 * 60
        return ta - tb || (a.task?.title ?? '').localeCompare(b.task?.title ?? '')
      })
    }
    return map
  }, [occs])

  const mutate = async (fn: () => Promise<unknown>) => {
    setBusy(true)
    try {
      await fn()
      await load()
      await onDataChanged()
    } catch (e) {
      setNotice(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }

  const complete = (o: Occurrence) => void mutate(() => api.completeOccurrence(o.id))
  const skip = (o: Occurrence) => void mutate(() => api.skipOccurrence(o.id))
  const progress = (o: Occurrence, minutes: number) => void mutate(() => api.addProgress(o.id, minutes))

  const rescheduleAll = () =>
    void mutate(async () => {
      const today = todayISO()
      const past = await api.listOccurrences(addDays(today, -RESCHEDULE_LOOKBACK_DAYS), addDays(today, -1))
      const taskIds = new Set<string>()
      for (const o of past) {
        if (o.status === 'missed' || o.status === 'pending') taskIds.add(o.task_id)
      }
      if (taskIds.size === 0) {
        setNotice('Нет пропущенных задач — всё выполнено!')
        return
      }
      let shifted = 0
      for (const id of taskIds) {
        const res = await api.rescheduleMissed(id)
        if (res.rescheduled > 0) shifted++
      }
      setNotice(`Перенесено задач: ${shifted}`)
    })

  const navigate = (dir: -1 | 1) => {
    if (mode === 'month') setAnchor(addMonths(anchor, dir))
    else if (mode === 'week') setAnchor(addDays(anchor, dir * 7))
    else setAnchor(addDays(anchor, dir))
  }

  const title =
    mode === 'month'
      ? monthTitle(anchor)
      : mode === 'week'
        ? `${humanDate(startOfWeekISO(anchor))} — ${humanDate(addDays(startOfWeekISO(anchor), 6))}`
        : `${weekdayOf(anchor)}, ${humanDate(anchor)}`

  return (
    <div className="p-6 space-y-4">
      <div className="flex items-center gap-3 flex-wrap">
        <h1 className="text-xl font-semibold">{title}</h1>
        <div className="flex items-center gap-1 ml-auto">
          <button className="btn-ghost" onClick={() => navigate(-1)}>
            <ChevronLeft size={18} />
          </button>
          <button className="btn-outline" onClick={() => setAnchor(todayISO())}>
            Сегодня
          </button>
          <button className="btn-ghost" onClick={() => navigate(1)}>
            <ChevronRight size={18} />
          </button>
        </div>
        <div className="flex rounded-xl border border-slate-200 bg-surface p-0.5">
          {(Object.keys(MODE_LABELS) as Mode[]).map((m) => (
            <button
              key={m}
              onClick={() => setMode(m)}
              className={`rounded-[10px] px-3.5 py-1.5 text-sm font-medium transition-colors ${
                mode === m ? 'bg-primary text-white' : 'text-muted hover:text-ink'
              }`}
            >
              {MODE_LABELS[m]}
            </button>
          ))}
        </div>
        <button className="btn-primary" onClick={rescheduleAll} disabled={busy}>
          <FastForward size={16} />
          Перенести невыполненные
        </button>
      </div>

      {notice && (
        <div className="card px-4 py-2.5 text-sm flex items-center">
          {notice}
          <button className="btn-ghost ml-auto py-1" onClick={() => setNotice(null)}>
            Закрыть
          </button>
        </div>
      )}

      {mode === 'month' && (
        <MonthGrid anchor={anchor} byDate={byDate} onPickDay={setPopupDate} onOpenDay={(d) => { setAnchor(d); setMode('day') }} />
      )}
      {mode === 'week' && (
        <WeekGrid anchor={anchor} byDate={byDate} onComplete={complete} onSkip={skip} onOpenDay={(d) => { setAnchor(d); setMode('day') }} />
      )}
      {mode === 'day' && <DayTimeline date={anchor} occurrences={byDate.get(anchor) ?? []} onComplete={complete} onSkip={skip} />}

      {popupDate && (
        <DayPopup
          date={popupDate}
          occurrences={byDate.get(popupDate) ?? []}
          busy={busy}
          onComplete={complete}
          onSkip={skip}
          onProgress={progress}
          onOpenDay={(d) => { setPopupDate(null); setAnchor(d); setMode('day') }}
          onClose={() => setPopupDate(null)}
        />
      )}
    </div>
  )
}

const DOT_COLORS: Record<string, string> = {
  pending: '#00ADD8',
  completed: '#2F9E44',
  skipped: '#868E96',
  missed: '#E03131',
  rescheduled: '#868E96',
}

function isTimed(o: Occurrence): boolean {
  return o.task?.start_time_minutes != null && !o.task?.all_day
}

interface DayPopupProps {
  date: string
  occurrences: Occurrence[]
  busy: boolean
  onComplete: (o: Occurrence) => void
  onSkip: (o: Occurrence) => void
  onProgress: (o: Occurrence, minutes: number) => void
  onOpenDay: (date: string) => void
  onClose: () => void
}

function DayPopup({ date, occurrences, busy, onComplete, onSkip, onProgress, onOpenDay, onClose }: DayPopupProps) {
  return (
    <div className="fixed inset-0 z-40 flex items-center justify-center bg-slate-900/30" onClick={onClose}>
      <div
        className="max-h-[80vh] w-[560px] overflow-y-auto rounded-2xl bg-surface p-5 shadow-xl"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="mb-3 flex items-center">
          <h2 className="text-base font-semibold">{weekdayOf(date)}, {humanDate(date)}</h2>
          <button className="btn-outline ml-auto py-1 text-xs" onClick={() => onOpenDay(date)}>
            Открыть день
          </button>
          <button className="btn-ghost ml-1 py-1" onClick={onClose}>✕</button>
        </div>
        {occurrences.length === 0 && <div className="py-6 text-center text-sm text-muted">На этот день задач нет.</div>}
        <div className="space-y-2">
          {occurrences.map((o) => {
            const effort = o.task?.effort_minutes ?? null
            const done = o.status === 'completed'
            const open = o.status === 'pending' || o.status === 'missed'
            const pct = effort ? Math.min(100, Math.round((o.progress_minutes / effort) * 100)) : null
            return (
              <div key={o.id} className={`rounded-xl border border-slate-200 p-3 ${done ? 'opacity-70' : ''}`}>
                <div className="flex items-center gap-2">
                  <span className="h-2.5 w-2.5 shrink-0 rounded-full" style={{ background: DOT_COLORS[o.status] ?? '#00ADD8' }} />
                  {isTimed(o) && (
                    <span className="text-xs font-semibold tabular-nums">{minutesToTime(o.task!.start_time_minutes!)}</span>
                  )}
                  <span className={`truncate text-sm font-medium ${done ? 'line-through' : ''}`}>{o.task?.title}</span>
                  {o.task?.topic?.name && <span className="truncate text-xs text-muted">· {o.task.topic.name}</span>}
                  {open && (
                    <span className="ml-auto flex shrink-0 gap-1">
                      <button className="btn-outline px-2 py-0.5 text-xs" disabled={busy} onClick={() => onComplete(o)}>✓</button>
                      <button className="btn-ghost px-2 py-0.5 text-xs" disabled={busy} onClick={() => onSkip(o)}>⏭</button>
                    </span>
                  )}
                </div>
                {effort != null && (
                  <div className="mt-2 flex items-center gap-2">
                    <div className="h-2 min-w-0 flex-1 overflow-hidden rounded-full bg-slate-100">
                      <div
                        className="h-full rounded-full"
                        style={{ width: `${pct}%`, background: pct !== null && pct >= 100 ? '#2F9E44' : '#00ADD8' }}
                      />
                    </div>
                    <span className="shrink-0 text-xs tabular-nums text-muted">
                      {o.progress_minutes}/{effort} мин
                    </span>
                    {open && (
                      <span className="flex shrink-0 gap-1">
                        <button className="btn-ghost px-1.5 py-0.5 text-xs" disabled={busy} onClick={() => onProgress(o, 25)}>+25′</button>
                        <button className="btn-ghost px-1.5 py-0.5 text-xs" disabled={busy || o.progress_minutes === 0} onClick={() => onProgress(o, -25)}>−25′</button>
                      </span>
                    )}
                  </div>
                )}
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}

interface GridProps {
  anchor: string
  byDate: Map<string, Occurrence[]>
  onComplete: (o: Occurrence) => void
  onSkip: (o: Occurrence) => void
  onOpenDay: (date: string) => void
}

interface MonthGridProps {
  anchor: string
  byDate: Map<string, Occurrence[]>
  onPickDay: (date: string) => void
  onOpenDay: (date: string) => void
}

function MonthGrid({ anchor, byDate, onPickDay, onOpenDay }: MonthGridProps) {
  const cells = monthGrid(anchor)
  const today = todayISO()
  return (
    <div className="card overflow-hidden">
      <div className="grid grid-cols-7 border-b border-slate-200 bg-slate-50/60">
        {WEEKDAYS_SHORT.map((d, i) => (
          <div key={d} className={`px-2 py-2 text-xs font-semibold text-center ${i >= 5 ? 'text-primary-dark' : 'text-muted'}`}>
            {d}
          </div>
        ))}
      </div>
      <div className="grid grid-cols-7">
        {cells.map((date) => {
          const inMonth = sameMonth(date, anchor)
          const isToday = date === today
          const list = byDate.get(date) ?? []
          const timed = list.filter(isTimed).slice(0, 2)
          const dots = list.filter((o) => !isTimed(o))
          return (
            <div
              key={date}
              onClick={() => onPickDay(date)}
              className={`min-h-[88px] border-b border-r border-slate-100 p-1.5 cursor-pointer transition-colors hover:bg-primary/5 ${
                inMonth ? '' : 'bg-slate-50/50'
              }`}
            >
              <div
                onClick={(e) => { e.stopPropagation(); onOpenDay(date) }}
                className={`mb-1 flex h-6 w-6 items-center justify-center rounded-lg text-xs font-semibold hover:ring-1 hover:ring-primary/40 ${
                  isToday ? 'bg-primary text-white' : inMonth ? 'text-ink' : 'text-slate-400'
                }`}
              >
                {Number(date.slice(8))}
              </div>
              <div className="space-y-1">
                {timed.map((o) => (
                  <div
                    key={o.id}
                    className="truncate rounded-md bg-primary/10 px-1.5 py-0.5 text-[11px] text-primary-dark"
                    title={`${minutesToTime(o.task!.start_time_minutes!)} ${o.task?.title ?? ''}`}
                  >
                    {minutesToTime(o.task!.start_time_minutes!)} {o.task?.title}
                  </div>
                ))}
                {dots.length > 0 && (
                  <div className="flex flex-wrap gap-1 px-0.5 pt-0.5">
                    {dots.slice(0, 12).map((o) => (
                      <span
                        key={o.id}
                        className="h-2 w-2 rounded-full"
                        title={`${o.task?.title ?? ''}${o.task?.effort_minutes ? ` · ${o.progress_minutes}/${o.task.effort_minutes} мин` : ''}`}
                        style={{ background: DOT_COLORS[o.status] ?? '#00ADD8' }}
                      />
                    ))}
                    {dots.length > 12 && <span className="text-[10px] text-muted">+{dots.length - 12}</span>}
                  </div>
                )}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}

function WeekGrid({ anchor, byDate, onComplete, onSkip, onOpenDay }: GridProps) {
  const start = startOfWeekISO(anchor)
  const days = Array.from({ length: 7 }, (_, i) => addDays(start, i))
  const today = todayISO()
  return (
    <div className="card overflow-hidden">
      <div className="grid grid-cols-7">
        {days.map((date, i) => (
          <div key={date} className="border-r border-slate-100 last:border-r-0">
            <button
              onClick={() => onOpenDay(date)}
              className={`w-full px-2 py-2 text-center border-b border-slate-200 ${
                date === today ? 'bg-primary/10' : 'bg-slate-50/60'
              }`}
            >
              <div className={`text-xs font-semibold ${i >= 5 ? 'text-primary-dark' : 'text-muted'}`}>
                {WEEKDAYS_SHORT[i]}
              </div>
              <div className={`text-sm font-semibold ${date === today ? 'text-primary-dark' : ''}`}>
                {Number(date.slice(8))}
              </div>
            </button>
            <div className="p-1.5 space-y-1 min-h-[320px]">
              {(byDate.get(date) ?? []).map((o) => (
                <OccurrenceChip key={o.id} occurrence={o} onComplete={onComplete} onSkip={onSkip} />
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

const DAY_START = 6 * 60
const DAY_END = 24 * 60

interface DayProps {
  date: string
  occurrences: Occurrence[]
  onComplete: (o: Occurrence) => void
  onSkip: (o: Occurrence) => void
}

function DayTimeline({ date, occurrences, onComplete, onSkip }: DayProps) {
  const anchored = occurrences.filter((o) => o.task?.start_time_minutes != null && !o.task?.all_day)
  const flexible = occurrences.filter((o) => o.task?.start_time_minutes == null || o.task?.all_day)
  const hours = Array.from({ length: (DAY_END - DAY_START) / 60 }, (_, i) => DAY_START + i * 60)

  return (
    <div className="space-y-4">
      {flexible.length > 0 && (
        <div className="card p-4">
          <div className="text-xs font-semibold text-muted uppercase tracking-wide mb-2">
            Гибкие задачи · {humanDate(date)}
          </div>
          <div className="flex flex-wrap gap-1.5">
            {flexible.map((o) => (
              <div key={o.id} className="min-w-[200px] max-w-xs flex-1">
                <OccurrenceChip occurrence={o} onComplete={onComplete} onSkip={onSkip} />
              </div>
            ))}
          </div>
        </div>
      )}

      <div className="card p-4">
        <div className="text-xs font-semibold text-muted uppercase tracking-wide mb-3">Расписание дня</div>
        <div className="relative" style={{ height: DAY_END - DAY_START }}>
          {hours.map((m) => (
            <div
              key={m}
              className="absolute left-0 right-0 border-t border-slate-100 flex"
              style={{ top: m - DAY_START }}
            >
              <span className="-mt-2.5 w-12 shrink-0 bg-surface pr-2 text-right text-[11px] text-muted tabular-nums">
                {minutesToTime(m)}
              </span>
            </div>
          ))}
          {date === todayISO() && <NowLine />}
          {anchored.map((o) => {
            const start = Math.max(o.task!.start_time_minutes!, DAY_START)
            const dur = Math.max(o.task?.estimated_duration_minutes ?? 30, 24)
            const top = start - DAY_START
            const height = Math.min(dur, DAY_END - start)
            const missed = o.status === 'missed' || (o.status === 'pending' && isPast(o.date))
            return (
              <div
                key={o.id}
                className={`absolute left-14 right-2 rounded-xl border px-3 py-1.5 text-sm overflow-hidden ${statusStyles(o)} ${
                  missed ? 'ring-1 ring-danger/40' : ''
                }`}
                style={{ top, height }}
              >
                <div className="flex items-center gap-2">
                  <span className="font-semibold tabular-nums text-xs">
                    {minutesToTime(o.task!.start_time_minutes!)}
                  </span>
                  <span className="font-medium truncate">{o.task?.title}</span>
                  {o.task?.topic?.name && <span className="text-xs opacity-70 truncate">· {o.task.topic.name}</span>}
                  {(o.status === 'pending' || o.status === 'missed') && (
                    <span className="ml-auto flex gap-1 shrink-0">
                      <button
                        className="rounded-lg border border-current/20 px-2 py-0.5 text-xs hover:bg-success/20"
                        onClick={() => onComplete(o)}
                      >
                        Выполнено
                      </button>
                      {o.status === 'pending' && (
                        <button
                          className="rounded-lg border border-current/20 px-2 py-0.5 text-xs hover:bg-danger/20"
                          onClick={() => onSkip(o)}
                        >
                          Пропустить
                        </button>
                      )}
                    </span>
                  )}
                </div>
              </div>
            )
          })}
          {anchored.length === 0 && (
            <div className="absolute inset-x-14 top-24 text-sm text-muted">Нет якорных задач с временем начала</div>
          )}
        </div>
      </div>
    </div>
  )
}

function NowLine() {
  const now = new Date()
  const minutes = now.getHours() * 60 + now.getMinutes()
  if (minutes < DAY_START || minutes > DAY_END) return null
  return (
    <div className="absolute left-12 right-0 z-10 border-t-2 border-primary" style={{ top: minutes - DAY_START }}>
      <span className="absolute -left-1 -top-1 h-2 w-2 rounded-full bg-primary" />
    </div>
  )
}
