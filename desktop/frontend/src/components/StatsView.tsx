import { useEffect, useMemo, useState } from 'react'
import { api } from '../api'
import type { Occurrence, Task, Topic } from '../types'
import { todayISO } from '../lib/dates'

interface Props {
  tasks: Task[]
  topics: Topic[]
}

const STATUS_META = [
  { key: 'completed', label: 'Выполнено', color: '#2F9E44' },
  { key: 'skipped', label: 'Пропущено осознанно', color: '#868E96' },
  { key: 'missed', label: 'Просрочено', color: '#E03131' },
] as const

type StatusKey = (typeof STATUS_META)[number]['key']

interface DayStat {
  date: string
  completed: number
  skipped: number
  missed: number
}

interface TopicStat {
  id: string
  name: string
  depth: number
  minutes: number
  count: number
}

function addDaysISO(iso: string, days: number): string {
  const d = new Date(iso + 'T00:00:00')
  d.setDate(d.getDate() + days)
  return d.toISOString().slice(0, 10)
}

function effortOf(task: Task | null | undefined): number {
  if (!task) return 0
  if (task.effort_minutes) return task.effort_minutes
  return 0
}

export default function StatsView({ tasks, topics }: Props) {
  const [occs, setOccs] = useState<Occurrence[]>([])
  const [error, setError] = useState<string | null>(null)
  const [tip, setTip] = useState<{ x: number; y: number; lines: string[] } | null>(null)

  const today = todayISO()
  const from = addDaysISO(today, -29)

  useEffect(() => {
    api
      .listOccurrences(from, today)
      .then(setOccs)
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }, [from, today])

  const taskById = useMemo(() => new Map(tasks.map((t) => [t.id, t])), [tasks])

  const days: DayStat[] = useMemo(() => {
    const map = new Map<string, DayStat>()
    for (let i = 13; i >= 0; i--) {
      const d = addDaysISO(today, -i)
      map.set(d, { date: d, completed: 0, skipped: 0, missed: 0 })
    }
    for (const o of occs) {
      const row = map.get(o.date)
      if (!row) continue
      if (o.status === 'completed' || o.status === 'skipped' || o.status === 'missed') {
        row[o.status as StatusKey]++
      }
    }
    return [...map.values()]
  }, [occs, today])

  const totals = useMemo(() => {
    let completed = 0
    let missed = 0
    let minutes = 0
    for (const o of occs) {
      if (o.status === 'completed') {
        completed++
        minutes += effortOf(o.task ?? taskById.get(o.task_id))
      }
      if (o.status === 'missed') missed++
    }
    return { completed, missed, minutes }
  }, [occs, taskById])

  const topicStats: TopicStat[] = useMemo(() => {
    const own = new Map<string, { minutes: number; count: number }>()
    const bump = (key: string, minutes: number) => {
      const cur = own.get(key) ?? { minutes: 0, count: 0 }
      cur.minutes += minutes
      cur.count += 1
      own.set(key, cur)
    }
    for (const o of occs) {
      if (o.status !== 'completed') continue
      const task = o.task ?? taskById.get(o.task_id)
      bump(task?.topic_id ?? 'none', effortOf(task))
    }
    const children = new Map<string | null, Topic[]>()
    const ids = new Set(topics.map((t) => t.id))
    for (const t of topics) {
      const key = t.parent_id && ids.has(t.parent_id) ? t.parent_id : null
      const list = children.get(key) ?? []
      list.push(t)
      children.set(key, list)
    }
    const subtree = (id: string): { minutes: number; count: number } => {
      const acc = { ...(own.get(id) ?? { minutes: 0, count: 0 }) }
      for (const c of children.get(id) ?? []) {
        const s = subtree(c.id)
        acc.minutes += s.minutes
        acc.count += s.count
      }
      return acc
    }
    const rows: TopicStat[] = []
    const walk = (parent: string | null, depth: number) => {
      const list = (children.get(parent) ?? []).sort((a, b) => a.name.localeCompare(b.name, 'ru'))
      for (const t of list) {
        const agg = subtree(t.id)
        if (agg.count === 0) continue
        rows.push({ id: t.id, name: t.name, depth, minutes: agg.minutes, count: agg.count })
        walk(t.id, depth + 1)
      }
    }
    walk(null, 0)
    const none = own.get('none')
    if (none && none.count > 0) {
      rows.push({ id: 'none', name: 'Без темы', depth: 0, minutes: none.minutes, count: none.count })
    }
    return rows
  }, [occs, taskById, topics])

  const dayMax = Math.max(1, ...days.map((d) => d.completed + d.skipped + d.missed))
  const topicMax = Math.max(1, ...topicStats.filter((t) => t.depth === 0).map((t) => t.minutes))

  const showTip = (e: React.MouseEvent, lines: string[]) =>
    setTip({ x: e.clientX + 12, y: e.clientY + 12, lines })

  const chartH = 160
  const barW = 22
  const gap = 14

  return (
    <div className="p-4 md:p-6 space-y-5 max-w-4xl" onMouseLeave={() => setTip(null)}>
      <h1 className="text-xl font-semibold">Статистика</h1>
      {error && <div className="card px-4 py-2.5 text-sm text-danger">{error}</div>}

      <div className="grid grid-cols-1 gap-3 min-[480px]:grid-cols-3 min-[480px]:gap-4">
        <div className="card p-4">
          <div className="text-2xl font-semibold tabular-nums">{totals.completed}</div>
          <div className="text-xs text-muted">выполнено вхождений за 30 дней</div>
        </div>
        <div className="card p-4">
          <div className="text-2xl font-semibold tabular-nums">
            {Math.round(totals.minutes / 60 * 10) / 10} ч
          </div>
          <div className="text-xs text-muted">трудозатрат закрыто ≈ {Math.round(totals.minutes / 25)} 🍅</div>
        </div>
        <div className="card p-4">
          <div className="text-2xl font-semibold tabular-nums">{totals.missed}</div>
          <div className="text-xs text-muted">просрочено за 30 дней</div>
        </div>
      </div>

      <div className="card p-5">
        <div className="mb-1 flex flex-wrap items-center gap-x-4 gap-y-1">
          <h2 className="text-sm font-semibold">Последние 14 дней</h2>
          <div className="flex flex-wrap gap-x-4 gap-y-1 md:ml-auto">
            {STATUS_META.map((s) => (
              <span key={s.key} className="flex items-center gap-1.5 text-xs text-muted">
                <span className="h-2.5 w-2.5 rounded-sm" style={{ background: s.color }} />
                {s.label}
              </span>
            ))}
          </div>
        </div>
        <svg
          viewBox={`0 0 ${(barW + gap) * days.length + 8} ${chartH + 24}`}
          width="100%"
          height={chartH + 24}
          preserveAspectRatio="xMinYMid meet"
          className="max-w-full"
        >
          {days.map((d, i) => {
            const total = d.completed + d.skipped + d.missed
            let y = chartH
            const x = 4 + i * (barW + gap)
            const segs = STATUS_META.map((s) => {
              const v = d[s.key]
              const h = total ? (v / dayMax) * (chartH - 8) : 0
              y -= h
              return { ...s, v, y, h }
            })
            const label = d.date.slice(8) + '.' + d.date.slice(5, 7)
            return (
              <g
                key={d.date}
                onMouseMove={(e) =>
                  showTip(e, [
                    label,
                    ...STATUS_META.filter((s) => d[s.key] > 0).map((s) => `${s.label}: ${d[s.key]}`),
                    ...(total === 0 ? ['нет событий'] : []),
                  ])
                }
                onMouseLeave={() => setTip(null)}
              >
                <rect x={x - 4} y={0} width={barW + 8} height={chartH} fill="transparent" />
                {segs.map(
                  (s, si) =>
                    s.h > 0 && (
                      <rect
                        key={s.key}
                        x={x}
                        y={s.y + (si > 0 ? 1 : 0)}
                        width={barW}
                        height={Math.max(1, s.h - (si > 0 ? 2 : 0))}
                        fill={s.color}
                      />
                    ),
                )}
                {total > 0 && (
                  <text x={x + barW / 2} y={Math.min(...segs.filter((s) => s.h > 0).map((s) => s.y)) - 4} textAnchor="middle" className="fill-slate-500" fontSize="10">
                    {total}
                  </text>
                )}
                <text x={x + barW / 2} y={chartH + 14} textAnchor="middle" className="fill-slate-400" fontSize="9">
                  {label.slice(0, 2)}
                </text>
              </g>
            )
          })}
          <line x1="0" y1={chartH} x2={(barW + gap) * days.length} y2={chartH} stroke="#E2E8F0" />
        </svg>
      </div>

      <div className="card p-5">
        <h2 className="mb-3 text-sm font-semibold">Темы за 30 дней — закрытые трудозатраты</h2>
        {topicStats.length === 0 && <div className="text-sm text-muted">Пока нет выполненных вхождений.</div>}
        <div className="flex flex-col gap-1.5">
          {topicStats.map((t) => (
            <div
              key={t.id}
              className="flex items-center gap-3"
              style={{ paddingLeft: `${t.depth * 20}px` }}
              onMouseMove={(e) => showTip(e, [t.name, `${t.minutes} мин · ${t.count} вхожд.`])}
              onMouseLeave={() => setTip(null)}
            >
              <span className={`w-44 shrink-0 truncate text-sm ${t.depth === 0 ? 'font-semibold' : 'text-muted'}`}>
                {t.name}
              </span>
              <svg className="h-4 min-w-0 flex-1">
                <rect
                  x="0"
                  y="2"
                  rx="4"
                  width={`${Math.max(1.5, (t.minutes / topicMax) * 100)}%`}
                  height="12"
                  fill="#00ADD8"
                  opacity={t.depth === 0 ? 1 : 0.55}
                />
              </svg>
              <span className="w-28 shrink-0 text-right text-xs tabular-nums text-muted">
                {t.minutes} мин · {t.count}
              </span>
            </div>
          ))}
        </div>
      </div>

      {tip && (
        <div
          className="pointer-events-none fixed z-50 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs shadow-lg"
          style={{ left: tip.x, top: tip.y }}
        >
          {tip.lines.map((l, i) => (
            <div key={i} className={i === 0 ? 'font-semibold' : 'text-muted'}>
              {l}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
