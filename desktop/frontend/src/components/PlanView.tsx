import { useCallback, useEffect, useMemo, useState } from 'react'
import { Anchor, ArrowDown, ArrowUp, CheckCheck, Plus, X } from 'lucide-react'
import { api, ApiError } from '../api'
import type { DayPlan, Occurrence } from '../types'
import { humanDate, minutesToTime, timeToMinutes, todayISO, weekdayOf } from '../lib/dates'
import { START_TIMES } from '../lib/recurrence'

interface DraftItem {
  occurrence_id: string
  planned_start_minutes: number | null
}

interface Props {
  onDataChanged: () => Promise<void> | void
}

export default function PlanView({ onDataChanged }: Props) {
  const [date, setDate] = useState(todayISO())
  const [occs, setOccs] = useState<Occurrence[]>([])
  const [plan, setPlan] = useState<DayPlan | null>(null)
  const [draft, setDraft] = useState<DraftItem[]>([])
  const [dirty, setDirty] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const load = useCallback(async () => {
    setError(null)
    try {
      const list = await api.listOccurrences(date, date)
      setOccs(list)
      try {
        const p = await api.getPlan(date)
        setPlan(p)
        setDraft(
          p.items.map((i) => ({ occurrence_id: i.occurrence_id, planned_start_minutes: i.planned_start_minutes })),
        )
      } catch (e) {
        if (e instanceof ApiError && e.code === 'not_found') {
          setPlan(null)
          setDraft([])
        } else {
          throw e
        }
      }
      setDirty(false)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }, [date])

  useEffect(() => {
    void load()
  }, [load])

  const byID = useMemo(() => new Map(occs.map((o) => [o.id, o])), [occs])
  const available = occs.filter(
    (o) => o.status === 'pending' && !draft.some((d) => d.occurrence_id === o.id),
  )

  const addItem = (o: Occurrence) => {
    setDraft((prev) => [
      ...prev,
      { occurrence_id: o.id, planned_start_minutes: o.task?.start_time_minutes ?? null },
    ])
    setDirty(true)
  }

  const removeItem = (id: string) => {
    setDraft((prev) => prev.filter((d) => d.occurrence_id !== id))
    setDirty(true)
  }

  const move = (idx: number, dir: -1 | 1) => {
    setDraft((prev) => {
      const next = [...prev]
      const j = idx + dir
      if (j < 0 || j >= next.length) return prev
      ;[next[idx], next[j]] = [next[j], next[idx]]
      return next
    })
    setDirty(true)
  }

  const setAnchorTime = (id: string, value: string) => {
    setDraft((prev) =>
      prev.map((d) =>
        d.occurrence_id === id ? { ...d, planned_start_minutes: value ? timeToMinutes(value) : null } : d,
      ),
    )
    setDirty(true)
  }

  const save = async () => {
    setBusy(true)
    setError(null)
    try {
      await api.putPlanItems(date, draft)
      await load()
      await onDataChanged()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }

  const commit = async () => {
    setBusy(true)
    setError(null)
    try {
      if (dirty) await api.putPlanItems(date, draft)
      await api.commitPlan(date)
      await load()
      await onDataChanged()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }

  const committed = plan?.committed_at != null

  return (
    <div className="p-6 space-y-4">
      <div className="flex items-center gap-4 flex-wrap">
        <h1 className="text-xl font-semibold">
          План дня · {weekdayOf(date)}, {humanDate(date)}
        </h1>
        <input className="input w-44" type="date" value={date} onChange={(e) => setDate(e.target.value)} />
        {committed && (
          <span className="rounded-xl bg-success/10 px-3 py-1.5 text-sm font-medium text-emerald-700">
            Закоммичен ({plan?.committed_by})
          </span>
        )}
        <div className="ml-auto flex gap-2">
          <button className="btn-outline" onClick={() => void save()} disabled={busy || !dirty}>
            Сохранить черновик
          </button>
          <button className="btn-primary" onClick={() => void commit()} disabled={busy || committed || draft.length === 0}>
            <CheckCheck size={16} />
            Закоммитить план
          </button>
        </div>
      </div>

      {error && (
        <div className="card px-4 py-2.5 text-sm text-danger flex items-center">
          {error}
          <button className="btn-ghost ml-auto py-1" onClick={() => setError(null)}>Закрыть</button>
        </div>
      )}

      <div className="grid grid-cols-2 gap-4">
        <div className="card p-4">
          <div className="text-xs font-semibold text-muted uppercase tracking-wide mb-3">
            Вхождения дня ({available.length})
          </div>
          <div className="space-y-1.5">
            {available.map((o) => (
              <div key={o.id} className="flex items-center gap-2 rounded-xl border border-slate-200 px-3 py-2 text-sm">
                {o.task?.start_time_minutes != null && (
                  <span className="tabular-nums text-xs font-semibold text-primary-dark">
                    {minutesToTime(o.task.start_time_minutes)}
                  </span>
                )}
                <span className="truncate">{o.task?.title}</span>
                {o.task?.topic?.name && <span className="text-xs text-muted truncate">· {o.task.topic.name}</span>}
                <button className="btn-ghost ml-auto p-1.5" title="В план" onClick={() => addItem(o)}>
                  <Plus size={15} />
                </button>
              </div>
            ))}
            {available.length === 0 && (
              <div className="py-8 text-center text-sm text-muted">Все вхождения дня уже в плане или их нет.</div>
            )}
          </div>
        </div>

        <div className="card p-4">
          <div className="text-xs font-semibold text-muted uppercase tracking-wide mb-3">
            В плане ({draft.length}){dirty && <span className="ml-2 text-amber-600 normal-case">не сохранено</span>}
          </div>
          <div className="space-y-1.5">
            {draft.map((d, idx) => {
              const o = byID.get(d.occurrence_id)
              return (
                <div key={d.occurrence_id} className="flex items-center gap-2 rounded-xl border border-primary/25 bg-primary/5 px-3 py-2 text-sm">
                  <span className="w-5 text-center text-xs font-semibold text-muted">{idx + 1}</span>
                  <span className="truncate flex-1">{o?.task?.title ?? '…'}</span>
                  <span title="Якорное время">
                    <Anchor size={13} className="text-muted shrink-0" />
                  </span>
                  <select
                    className="input w-28 py-1 text-xs"
                    value={d.planned_start_minutes != null ? minutesToTime(d.planned_start_minutes) : ''}
                    onChange={(e) => setAnchorTime(d.occurrence_id, e.target.value)}
                  >
                    <option value="">гибкая</option>
                    {START_TIMES.map((m) => (
                      <option key={m} value={minutesToTime(m)}>{minutesToTime(m)}</option>
                    ))}
                  </select>
                  <button className="btn-ghost p-1" onClick={() => move(idx, -1)} disabled={idx === 0}>
                    <ArrowUp size={14} />
                  </button>
                  <button className="btn-ghost p-1" onClick={() => move(idx, 1)} disabled={idx === draft.length - 1}>
                    <ArrowDown size={14} />
                  </button>
                  <button className="btn-ghost p-1 hover:text-danger" onClick={() => removeItem(d.occurrence_id)}>
                    <X size={14} />
                  </button>
                </div>
              )
            })}
            {draft.length === 0 && (
              <div className="py-8 text-center text-sm text-muted">
                Добавь вхождения слева, расставь якоря и закоммить план.
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
