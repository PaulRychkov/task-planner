import { useCallback, useEffect, useState } from 'react'
import { CalendarDays, ListTodo, Tags, BarChart3, CircleAlert } from 'lucide-react'
import { api } from './api'
import type { Task, Topic } from './types'
import CalendarView from './components/CalendarView'
import TasksView from './components/TasksView'
import TopicsView from './components/TopicsView'
import StatsView from './components/StatsView'

type View = 'calendar' | 'tasks' | 'topics' | 'stats'

const NAV: { view: View; label: string; icon: typeof CalendarDays }[] = [
  { view: 'calendar', label: 'Календарь', icon: CalendarDays },
  { view: 'tasks', label: 'Задачи', icon: ListTodo },
  { view: 'topics', label: 'Темы', icon: Tags },
  { view: 'stats', label: 'Статистика', icon: BarChart3 },
]

export default function App() {
  const [view, setView] = useState<View>('calendar')
  const [topics, setTopics] = useState<Topic[]>([])
  const [tasks, setTasks] = useState<Task[]>([])
  const [online, setOnline] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const reload = useCallback(async () => {
    try {
      const ok = await api.health()
      setOnline(ok)
      if (!ok) return
      const [t, ts] = await Promise.all([api.listTopics(true), api.listTasks()])
      setTopics(t)
      setTasks(ts)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }, [])

  useEffect(() => {
    void reload()
    const timer = setInterval(() => {
      void api.health().then((ok) => {
        setOnline((prev) => {
          if (ok && !prev) {
            void reload()
          }
          return ok
        })
      })
    }, 15000)
    return () => clearInterval(timer)
  }, [reload])

  return (
    <div className="flex h-screen overflow-hidden">
      <aside className="w-56 shrink-0 border-r border-slate-200 bg-surface flex flex-col">
        <div className="px-5 py-5 flex items-center gap-2.5">
          <div className="h-9 w-9 rounded-xl bg-primary flex items-center justify-center text-white font-bold text-lg">
            З
          </div>
          <div>
            <div className="font-semibold leading-tight">Задачи</div>
            <div className="text-xs text-muted">activization</div>
          </div>
        </div>
        <nav className="px-3 space-y-1">
          {NAV.map(({ view: v, label, icon: Icon }) => (
            <button
              key={v}
              onClick={() => setView(v)}
              className={`w-full flex items-center gap-2.5 rounded-xl px-3 py-2.5 text-sm font-medium transition-colors ${
                view === v ? 'bg-primary/10 text-primary-dark' : 'text-muted hover:bg-slate-100 hover:text-ink'
              }`}
            >
              <Icon size={18} />
              {label}
            </button>
          ))}
        </nav>
        <div className="mt-auto px-5 py-4 text-xs text-muted flex items-center gap-2">
          <span className={`h-2 w-2 rounded-full ${online ? 'bg-success' : 'bg-danger'}`} />
          {online ? 'backend доступен' : 'backend недоступен'}
        </div>
      </aside>

      <main className="flex-1 overflow-y-auto">
        {!online && (
          <div className="m-6 card p-4 flex items-center gap-3 text-sm text-danger border-danger/30">
            <CircleAlert size={18} />
            Бэкенд не отвечает на http://localhost:8081 — запусти tasks backend и обнови.
            <button className="btn-outline ml-auto" onClick={() => void reload()}>
              Обновить
            </button>
          </div>
        )}
        {error && <div className="m-6 card p-4 text-sm text-danger border-danger/30">{error}</div>}
        {view === 'calendar' && <CalendarView onDataChanged={reload} />}
        {view === 'tasks' && <TasksView tasks={tasks} topics={topics} onDataChanged={reload} />}
        {view === 'topics' && <TopicsView topics={topics} onDataChanged={reload} />}
        {view === 'stats' && <StatsView tasks={tasks} topics={topics} />}
      </main>
    </div>
  )
}
