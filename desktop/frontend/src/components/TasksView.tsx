import { useMemo, useState } from 'react'
import { ChevronDown, ChevronRight, Clock, Pencil, Plus, Timer, Trash2 } from 'lucide-react'
import { api } from '../api'
import type { Task, Topic } from '../types'
import { minutesToTime } from '../lib/dates'
import { describeRecurrence } from '../lib/recurrence'
import { priorityWeightPercent } from '../lib/priority'
import TaskForm from './TaskForm'

interface Props {
  tasks: Task[]
  topics: Topic[]
  onDataChanged: () => Promise<void> | void
}

const PROGRESS_LABELS: Record<string, string> = {
  needs_action: 'активна',
  in_process: 'в работе',
  completed: 'завершена',
  cancelled: 'отменена',
}

interface TopicNode {
  topic: Topic
  children: TopicNode[]
  tasks: Task[]
}

function buildTree(topics: Topic[], tasks: Task[]): { roots: TopicNode[]; orphanTasks: Task[] } {
  const nodes = new Map<string, TopicNode>()
  for (const t of topics) {
    nodes.set(t.id, { topic: t, children: [], tasks: [] })
  }
  const roots: TopicNode[] = []
  for (const node of nodes.values()) {
    const pid = node.topic.parent_id
    if (pid && nodes.has(pid)) {
      nodes.get(pid)!.children.push(node)
    } else {
      roots.push(node)
    }
  }
  const orphanTasks: Task[] = []
  for (const task of tasks) {
    const node = task.topic_id ? nodes.get(task.topic_id) : undefined
    if (node) {
      node.tasks.push(task)
    } else {
      orphanTasks.push(task)
    }
  }
  const byName = (a: TopicNode, b: TopicNode) => a.topic.name.localeCompare(b.topic.name, 'ru')
  const sortRec = (list: TopicNode[]) => {
    list.sort(byName)
    list.forEach((n) => sortRec(n.children))
  }
  sortRec(roots)
  return { roots, orphanTasks }
}

function timeMeta(task: Task): React.ReactNode {
  if (task.effort_minutes) {
    return (
      <span className="inline-flex items-center gap-1">
        <Timer size={13} />
        {task.effort_minutes} мин ≈ {Math.max(1, Math.round(task.effort_minutes / 25))} 🍅
      </span>
    )
  }
  if (task.all_day) return 'весь день'
  if (task.start_time_minutes != null) {
    return (
      <span className="inline-flex items-center gap-1">
        <Clock size={13} />
        {minutesToTime(task.start_time_minutes)}
        {task.estimated_duration_minutes ? ` · ${task.estimated_duration_minutes} мин` : ''}
      </span>
    )
  }
  return '—'
}

function countTasks(node: TopicNode): number {
  return node.tasks.length + node.children.reduce((acc, c) => acc + countTasks(c), 0)
}

export default function TasksView({ tasks, topics, onDataChanged }: Props) {
  const [editing, setEditing] = useState<Task | null>(null)
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set())

  const { roots, orphanTasks } = useMemo(() => buildTree(topics, tasks), [topics, tasks])

  const toggleCollapsed = (id: string) =>
    setCollapsed((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })

  const toggleActive = async (task: Task) => {
    try {
      await api.updateTask(task.id, {
        title: task.title,
        description: task.description,
        topic_id: task.topic_id,
        recurrence_kind: task.recurrence_kind,
        recurrence_params: task.recurrence_params,
        start_date: task.start_date,
        due: task.due,
        start_time_minutes: task.start_time_minutes,
        estimated_duration_minutes: task.estimated_duration_minutes,
        effort_minutes: task.effort_minutes,
        all_day: task.all_day,
        requires_pomodoro: task.requires_pomodoro,
        priority: task.priority,
        is_active: !task.is_active,
      })
      await onDataChanged()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  const remove = async (task: Task) => {
    if (!confirm(`Удалить задачу «${task.title}» вместе с историей?`)) return
    try {
      await api.deleteTask(task.id)
      await onDataChanged()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  const renderTask = (task: Task, depth: number) => (
    <div
      key={task.id}
      className={`flex items-center gap-3 border-b border-slate-100 py-2 pr-3 last:border-b-0 hover:bg-slate-50/60 ${
        !task.is_active || !['needs_action', 'in_process'].includes(task.progress) ? 'opacity-50' : ''
      }`}
      style={{ paddingLeft: `${depth * 22 + 34}px` }}
    >
      <input
        type="checkbox"
        className="h-4 w-4 shrink-0 accent-primary cursor-pointer"
        checked={task.is_active}
        onChange={() => void toggleActive(task)}
      />
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2 font-medium">
          {task.priority > 0 && (
            <span
              className="shrink-0 rounded-md bg-primary/10 px-1.5 py-0.5 text-[11px] font-semibold text-primary-dark"
              title={`Приоритет ${task.priority} — вес ${priorityWeightPercent(task.priority)}%`}
            >
              P{task.priority}
            </span>
          )}
          <span className="min-w-0 flex-1 truncate">{task.title}</span>
        </div>
        <div className="flex flex-wrap gap-x-2 gap-y-0.5 text-xs text-muted">
          <span>{PROGRESS_LABELS[task.progress]}</span>
          <span>{describeRecurrence(task)}</span>
          {task.due && <span className="text-danger">дедлайн {task.due}</span>}
          <span className="min-[560px]:hidden">{timeMeta(task)}</span>
        </div>
      </div>
      <span className="hidden shrink-0 text-xs text-muted tabular-nums min-[560px]:inline">
        {timeMeta(task)}
      </span>
      <div className="flex shrink-0 gap-1">
        <button className="btn-ghost p-1.5" title="Редактировать" onClick={() => setEditing(task)}>
          <Pencil size={15} />
        </button>
        <button className="btn-ghost p-1.5 hover:text-danger" title="Удалить" onClick={() => void remove(task)}>
          <Trash2 size={15} />
        </button>
      </div>
    </div>
  )

  const renderNode = (node: TopicNode, depth: number) => {
    const isCollapsed = collapsed.has(node.topic.id)
    const total = countTasks(node)
    return (
      <div key={node.topic.id}>
        <button
          className="flex w-full items-center gap-1.5 border-b border-slate-100 py-2 pr-3 text-left hover:bg-slate-50/60"
          style={{ paddingLeft: `${depth * 22 + 10}px` }}
          onClick={() => toggleCollapsed(node.topic.id)}
        >
          {isCollapsed ? <ChevronRight size={15} className="shrink-0 text-muted" /> : <ChevronDown size={15} className="shrink-0 text-muted" />}
          <span className={`font-semibold ${node.topic.is_archived ? 'text-muted line-through' : ''}`}>{node.topic.name}</span>
          <span className="text-xs text-muted">{total}</span>
        </button>
        {!isCollapsed && (
          <div>
            {node.tasks.map((t) => renderTask(t, depth + 1))}
            {node.children.map((c) => renderNode(c, depth + 1))}
          </div>
        )}
      </div>
    )
  }

  return (
    <div className="p-4 md:p-6 space-y-4">
      <div className="flex items-center">
        <h1 className="text-xl font-semibold">Мои задачи</h1>
        <button className="btn-primary ml-auto" onClick={() => setCreating(true)}>
          <Plus size={16} />
          Новая задача
        </button>
      </div>

      {error && (
        <div className="card px-4 py-2.5 text-sm text-danger flex items-center">
          {error}
          <button className="btn-ghost ml-auto py-1" onClick={() => setError(null)}>Закрыть</button>
        </div>
      )}

      <div className="card overflow-hidden">
        {roots.map((n) => renderNode(n, 0))}
        {orphanTasks.length > 0 && (
          <div>
            <div className="border-b border-slate-100 py-2 pl-2.5 pr-3 text-sm font-semibold text-muted">Без темы</div>
            {orphanTasks.map((t) => renderTask(t, 0))}
          </div>
        )}
        {roots.length === 0 && orphanTasks.length === 0 && (
          <div className="px-4 py-10 text-center text-muted">Задач пока нет — создай первую.</div>
        )}
      </div>

      {(creating || editing) && (
        <TaskForm
          task={editing}
          topics={topics}
          onClose={() => {
            setCreating(false)
            setEditing(null)
          }}
          onSaved={onDataChanged}
        />
      )}
    </div>
  )
}
