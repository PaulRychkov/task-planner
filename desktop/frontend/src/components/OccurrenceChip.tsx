import { Check, X } from 'lucide-react'
import type { Occurrence } from '../types'
import { isPast, minutesToTime } from '../lib/dates'

export function statusStyles(o: Occurrence): string {
  const missed = o.status === 'missed' || (o.status === 'pending' && isPast(o.date))
  if (o.status === 'completed') return 'bg-success/10 text-emerald-700 border-success/30'
  if (missed) return 'bg-danger/10 text-red-700 border-danger/30'
  if (o.status === 'skipped') return 'bg-slate-100 text-muted border-slate-200 line-through'
  if (o.status === 'rescheduled') return 'bg-amber-50 text-amber-700 border-amber-200'
  return 'bg-primary/5 text-primary-dark border-primary/25'
}

interface Props {
  occurrence: Occurrence
  compact?: boolean
  onComplete: (o: Occurrence) => void
  onSkip: (o: Occurrence) => void
}

export default function OccurrenceChip({ occurrence: o, compact, onComplete, onSkip }: Props) {
  const actionable = o.status === 'pending' || o.status === 'missed'
  const title = o.task?.title ?? '—'
  const time = o.task?.start_time_minutes != null && !o.task.all_day ? minutesToTime(o.task.start_time_minutes) : null

  return (
    <div
      className={`group flex items-center gap-1 rounded-lg border px-1.5 py-0.5 text-xs ${statusStyles(o)}`}
      title={`${title}${o.task?.topic?.name ? ` · ${o.task.topic.name}` : ''} · ${o.status}`}
    >
      {time && !compact && <span className="font-semibold tabular-nums">{time}</span>}
      <span className="truncate flex-1">{title}</span>
      {actionable && (
        <span className="flex items-center gap-0.5 shrink-0 [@media(hover:hover)]:hidden [@media(hover:hover)]:group-hover:flex">
          <button
            className="rounded p-0.5 hover:bg-success/20 text-emerald-700"
            title="Выполнено"
            onClick={(e) => {
              e.stopPropagation()
              onComplete(o)
            }}
          >
            <Check size={12} />
          </button>
          {o.status === 'pending' && (
            <button
              className="rounded p-0.5 hover:bg-danger/20 text-red-600"
              title="Пропустить осознанно"
              onClick={(e) => {
                e.stopPropagation()
                onSkip(o)
              }}
            >
              <X size={12} />
            </button>
          )}
        </span>
      )}
    </div>
  )
}
