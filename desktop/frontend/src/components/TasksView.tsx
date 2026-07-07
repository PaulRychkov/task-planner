import { useState } from 'react'
import { Clock, Pencil, Plus, Trash2 } from 'lucide-react'
import { api } from '../api'
import type { Task, Topic } from '../types'
import { minutesToTime } from '../lib/dates'
import { describeRecurrence } from '../lib/recurrence'
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

export default function TasksView({ tasks, topics, onDataChanged }: Props) {
  const [editing, setEditing] = useState<Task | null>(null)
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState<string | null>(null)

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
        all_day: task.all_day,
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

  return (
    <div className="p-6 space-y-4">
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
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-slate-200 bg-slate-50/60 text-left text-xs text-muted">
              <th className="px-4 py-2.5 font-semibold w-12">Вкл</th>
              <th className="px-4 py-2.5 font-semibold">Задача</th>
              <th className="px-4 py-2.5 font-semibold">Тема</th>
              <th className="px-4 py-2.5 font-semibold">Повторение</th>
              <th className="px-4 py-2.5 font-semibold">Время</th>
              <th className="px-4 py-2.5 font-semibold">С какого дня</th>
              <th className="px-4 py-2.5 font-semibold w-24"></th>
            </tr>
          </thead>
          <tbody>
            {tasks.map((task) => (
              <tr key={task.id} className={`border-b border-slate-100 last:border-b-0 hover:bg-slate-50/60 ${!task.is_active || !['needs_action', 'in_process'].includes(task.progress) ? 'opacity-50' : ''}`}>
                <td className="px-4 py-2.5">
                  <input
                    type="checkbox"
                    className="h-4 w-4 accent-primary cursor-pointer"
                    checked={task.is_active}
                    onChange={() => void toggleActive(task)}
                  />
                </td>
                <td className="px-4 py-2.5">
                  <div className="font-medium flex items-center gap-2">
                    {task.priority > 0 && (
                      <span className="rounded-md bg-primary/10 px-1.5 py-0.5 text-[11px] font-semibold text-primary-dark">
                        P{task.priority}
                      </span>
                    )}
                    {task.title}
                  </div>
                  <div className="text-xs text-muted flex gap-2">
                    <span>{PROGRESS_LABELS[task.progress]}</span>
                    {task.due && <span className="text-danger">дедлайн {task.due}</span>}
                  </div>
                </td>
                <td className="px-4 py-2.5 text-muted">{task.topic?.name ?? '—'}</td>
                <td className="px-4 py-2.5">
                  <span className="rounded-lg bg-slate-100 px-2 py-1 text-xs">{describeRecurrence(task)}</span>
                </td>
                <td className="px-4 py-2.5 text-muted tabular-nums">
                  {task.all_day ? (
                    'весь день'
                  ) : task.start_time_minutes != null ? (
                    <span className="inline-flex items-center gap-1">
                      <Clock size={13} />
                      {minutesToTime(task.start_time_minutes)}
                      {task.estimated_duration_minutes ? ` · ${task.estimated_duration_minutes} мин` : ''}
                    </span>
                  ) : task.estimated_duration_minutes ? (
                    `${task.estimated_duration_minutes} мин`
                  ) : (
                    '—'
                  )}
                </td>
                <td className="px-4 py-2.5 text-muted tabular-nums">{task.start_date}</td>
                <td className="px-4 py-2.5">
                  <div className="flex gap-1 justify-end">
                    <button className="btn-ghost p-1.5" title="Редактировать" onClick={() => setEditing(task)}>
                      <Pencil size={15} />
                    </button>
                    <button className="btn-ghost p-1.5 hover:text-danger" title="Удалить" onClick={() => void remove(task)}>
                      <Trash2 size={15} />
                    </button>
                  </div>
                </td>
              </tr>
            ))}
            {tasks.length === 0 && (
              <tr>
                <td colSpan={7} className="px-4 py-10 text-center text-muted">
                  Задач пока нет — создай первую.
                </td>
              </tr>
            )}
          </tbody>
        </table>
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
