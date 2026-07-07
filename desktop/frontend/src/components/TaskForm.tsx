import { useMemo, useState } from 'react'
import { X } from 'lucide-react'
import { api } from '../api'
import type { RecurrenceKind, Task, TaskInput, Topic } from '../types'
import { todayISO, timeToMinutes, minutesToTime, WEEKDAYS_SHORT } from '../lib/dates'
import { DURATIONS, KIND_LABELS, KIND_ORDER, paramsRequired, SR_PRESETS, START_TIMES } from '../lib/recurrence'

interface Props {
  task: Task | null
  topics: Topic[]
  onClose: () => void
  onSaved: () => Promise<void> | void
}

function presetIndexFor(intervals: number[] | undefined): number {
  if (!intervals) return 0
  const key = intervals.join(',')
  const idx = SR_PRESETS.findIndex((p) => p.intervals.join(',') === key)
  return idx >= 0 ? idx : -1
}

export default function TaskForm({ task, topics, onClose, onSaved }: Props) {
  const [title, setTitle] = useState(task?.title ?? '')
  const [description, setDescription] = useState(task?.description ?? '')
  const [topicID, setTopicID] = useState(task?.topic_id ?? '')
  const [kind, setKind] = useState<RecurrenceKind>(task?.recurrence_kind ?? 'once')
  const [days, setDays] = useState<number[]>(task?.recurrence_params?.days ?? [1, 3, 5])
  const [n, setN] = useState(task?.recurrence_params?.n ?? 2)
  const [dayOfMonth, setDayOfMonth] = useState<string>(
    task?.recurrence_params?.day_of_month ? String(task.recurrence_params.day_of_month) : '',
  )
  const initialPreset = presetIndexFor(task?.recurrence_params?.intervals ?? undefined)
  const [srPreset, setSrPreset] = useState<number>(task ? initialPreset : 0)
  const [customIntervals, setCustomIntervals] = useState(
    task?.recurrence_params?.intervals?.join(', ') ?? '0, 1, 3, 7, 14, 30, 90',
  )
  const [startDate, setStartDate] = useState(task?.start_date ?? todayISO())
  const [due, setDue] = useState(task?.due ?? '')
  const [startTime, setStartTime] = useState(
    task?.start_time_minutes != null ? minutesToTime(task.start_time_minutes) : '',
  )
  const [duration, setDuration] = useState(
    task?.estimated_duration_minutes != null ? String(task.estimated_duration_minutes) : '',
  )
  const [allDay, setAllDay] = useState(task?.all_day ?? false)
  const [priority, setPriority] = useState(task?.priority ?? 0)
  const [isActive, setIsActive] = useState(task?.is_active ?? true)
  const [error, setError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  const intervals = useMemo(() => {
    if (srPreset >= 0) return SR_PRESETS[srPreset].intervals
    return customIntervals
      .split(/[,\s]+/)
      .filter(Boolean)
      .map(Number)
      .filter((n) => Number.isInteger(n) && n >= 0)
  }, [srPreset, customIntervals])

  const toggleDay = (d: number) =>
    setDays((prev) => (prev.includes(d) ? prev.filter((x) => x !== d) : [...prev, d].sort((a, b) => a - b)))

  const submit = async () => {
    setError(null)
    const input: TaskInput = {
      title: title.trim(),
      description: description.trim() ? description.trim() : null,
      topic_id: topicID || null,
      recurrence_kind: kind,
      recurrence_params: null,
      start_date: startDate,
      due: kind === 'once' && due ? due : null,
      start_time_minutes: startTime ? timeToMinutes(startTime) : null,
      estimated_duration_minutes: duration ? Number(duration) : null,
      all_day: allDay,
      priority,
      is_active: isActive,
    }
    if (paramsRequired(kind)) {
      if (kind === 'days_of_week') input.recurrence_params = { days }
      else if (kind === 'every_n_days' || kind === 'every_n_weeks') input.recurrence_params = { n }
      else if (kind === 'monthly')
        input.recurrence_params = { day_of_month: dayOfMonth ? Number(dayOfMonth) : null }
      else if (kind === 'spaced_repetition') input.recurrence_params = { intervals }
    }
    setSaving(true)
    try {
      if (task) await api.updateTask(task.id, input)
      else await api.createTask(input)
      await onSaved()
      onClose()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-ink/40 p-4" onClick={onClose}>
      <div
        className="card w-full max-w-2xl max-h-[92vh] overflow-y-auto p-6"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center mb-5">
          <h2 className="text-lg font-semibold">{task ? 'Редактировать задачу' : 'Новая задача'}</h2>
          <button className="btn-ghost ml-auto p-2" onClick={onClose}>
            <X size={18} />
          </button>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div className="col-span-2">
            <label className="label">Задача</label>
            <input className="input" value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Что нужно делать" autoFocus />
          </div>

          <div className="col-span-2">
            <label className="label">Заметка</label>
            <textarea className="input resize-none" rows={2} value={description} onChange={(e) => setDescription(e.target.value)} />
          </div>

          <div>
            <label className="label">Тема</label>
            <select className="input" value={topicID} onChange={(e) => setTopicID(e.target.value)}>
              <option value="">Без темы</option>
              {topics.filter((t) => !t.is_archived).map((t) => (
                <option key={t.id} value={t.id}>{t.name}</option>
              ))}
            </select>
          </div>

          <div>
            <label className="label">Формат повторения</label>
            <select className="input" value={kind} onChange={(e) => setKind(e.target.value as RecurrenceKind)}>
              {KIND_ORDER.map((k) => (
                <option key={k} value={k}>{KIND_LABELS[k]}</option>
              ))}
            </select>
          </div>

          {kind === 'days_of_week' && (
            <div className="col-span-2">
              <label className="label">Дни недели</label>
              <div className="flex gap-1.5">
                {WEEKDAYS_SHORT.map((label, i) => (
                  <button
                    key={label}
                    type="button"
                    onClick={() => toggleDay(i + 1)}
                    className={`h-9 w-11 rounded-xl border text-sm font-medium transition-colors ${
                      days.includes(i + 1)
                        ? 'bg-primary text-white border-primary'
                        : 'bg-surface border-slate-200 text-muted hover:border-primary'
                    }`}
                  >
                    {label}
                  </button>
                ))}
              </div>
            </div>
          )}

          {(kind === 'every_n_days' || kind === 'every_n_weeks') && (
            <div>
              <label className="label">{kind === 'every_n_days' ? 'Каждые N дней' : 'Каждые N недель'}</label>
              <input
                className="input"
                type="number"
                min={1}
                value={n}
                onChange={(e) => setN(Math.max(1, Number(e.target.value)))}
              />
            </div>
          )}

          {kind === 'monthly' && (
            <div>
              <label className="label">День месяца (пусто = как дата старта)</label>
              <input
                className="input"
                type="number"
                min={1}
                max={31}
                value={dayOfMonth}
                onChange={(e) => setDayOfMonth(e.target.value)}
                placeholder="как в дате старта"
              />
            </div>
          )}

          {kind === 'spaced_repetition' && (
            <div className="col-span-2 space-y-2">
              <label className="label">Схема закрепления</label>
              <select
                className="input"
                value={srPreset}
                onChange={(e) => setSrPreset(Number(e.target.value))}
              >
                {SR_PRESETS.map((p, i) => (
                  <option key={p.label} value={i}>{p.label}</option>
                ))}
                <option value={-1}>Свой набор интервалов</option>
              </select>
              {srPreset === -1 && (
                <input
                  className="input"
                  value={customIntervals}
                  onChange={(e) => setCustomIntervals(e.target.value)}
                  placeholder="0, 1, 3, 7, 14, 30, 90"
                />
              )}
              <div className="text-xs text-muted">Дни от даты старта: {intervals.join(' → ') || '—'}</div>
            </div>
          )}

          <div>
            <label className="label">С какого дня</label>
            <input className="input" type="date" value={startDate} onChange={(e) => setStartDate(e.target.value)} />
          </div>

          {kind === 'once' && (
            <div>
              <label className="label">Дедлайн</label>
              <input className="input" type="date" value={due} onChange={(e) => setDue(e.target.value)} />
            </div>
          )}

          <div>
            <label className="label">Начало</label>
            <select className="input" value={startTime} onChange={(e) => setStartTime(e.target.value)} disabled={allDay}>
              <option value="">Без времени</option>
              {START_TIMES.map((m) => (
                <option key={m} value={minutesToTime(m)}>{minutesToTime(m)}</option>
              ))}
            </select>
          </div>

          <div>
            <label className="label">Длительность</label>
            <select className="input" value={duration} onChange={(e) => setDuration(e.target.value)} disabled={allDay}>
              <option value="">Не задана</option>
              {DURATIONS.map((d) => (
                <option key={d.minutes} value={d.minutes}>{d.label}</option>
              ))}
            </select>
          </div>

          <div>
            <label className="label">Приоритет (1 — высший, 0 — нет)</label>
            <select className="input" value={priority} onChange={(e) => setPriority(Number(e.target.value))}>
              {Array.from({ length: 10 }, (_, i) => (
                <option key={i} value={i}>{i === 0 ? 'Без приоритета' : i}</option>
              ))}
            </select>
          </div>

          <div className="flex items-end gap-5 pb-2">
            <label className="flex items-center gap-2 text-sm cursor-pointer">
              <input type="checkbox" className="h-4 w-4 accent-primary" checked={allDay} onChange={(e) => setAllDay(e.target.checked)} />
              Весь день
            </label>
            <label className="flex items-center gap-2 text-sm cursor-pointer">
              <input type="checkbox" className="h-4 w-4 accent-primary" checked={isActive} onChange={(e) => setIsActive(e.target.checked)} />
              Вкл
            </label>
          </div>
        </div>

        {error && <div className="mt-4 rounded-xl bg-danger/10 px-4 py-2.5 text-sm text-danger">{error}</div>}

        <div className="mt-6 flex justify-end gap-2">
          <button className="btn-outline" onClick={onClose}>Отмена</button>
          <button className="btn-primary" onClick={() => void submit()} disabled={saving || !title.trim()}>
            {task ? 'Сохранить' : 'Создать'}
          </button>
        </div>
      </div>
    </div>
  )
}
