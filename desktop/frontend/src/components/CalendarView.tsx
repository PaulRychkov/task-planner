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
        <MonthGrid anchor={anchor} byDate={byDate} onComplete={complete} onSkip={skip} onOpenDay={(d) => { setAnchor(d); setMode('day') }} />
      )}
      {mode === 'week' && (
        <WeekGrid anchor={anchor} byDate={byDate} onComplete={complete} onSkip={skip} onOpenDay={(d) => { setAnchor(d); setMode('day') }} />
      )}
      {mode === 'day' && <DayTimeline date={anchor} occurrences={byDate.get(anchor) ?? []} onComplete={complete} onSkip={skip} />}
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

function MonthGrid({ anchor, byDate, onComplete, onSkip, onOpenDay }: GridProps) {
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
          const shown = list.slice(0, 3)
          return (
            <div
              key={date}
              onClick={() => onOpenDay(date)}
              className={`min-h-[104px] border-b border-r border-slate-100 p-1.5 cursor-pointer transition-colors hover:bg-primary/5 ${
                inMonth ? '' : 'bg-slate-50/50'
              }`}
            >
              <div
                className={`mb-1 flex h-6 w-6 items-center justify-center rounded-lg text-xs font-semibold ${
                  isToday ? 'bg-primary text-white' : inMonth ? 'text-ink' : 'text-slate-400'
                }`}
              >
                {Number(date.slice(8))}
              </div>
              <div className="space-y-0.5">
                {shown.map((o) => (
                  <OccurrenceChip key={o.id} occurrence={o} compact onComplete={onComplete} onSkip={onSkip} />
                ))}
                {list.length > shown.length && (
                  <div className="text-[11px] text-muted px-1">ещё {list.length - shown.length}</div>
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
