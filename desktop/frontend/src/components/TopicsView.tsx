import { useState } from 'react'
import { Archive, ArchiveRestore, Plus, Trash2 } from 'lucide-react'
import { api } from '../api'
import type { Topic } from '../types'

interface Props {
  topics: Topic[]
  onDataChanged: () => Promise<void> | void
}

export default function TopicsView({ topics, onDataChanged }: Props) {
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [error, setError] = useState<string | null>(null)

  const run = async (fn: () => Promise<unknown>) => {
    setError(null)
    try {
      await fn()
      await onDataChanged()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  const add = () =>
    void run(async () => {
      await api.createTopic({ name: name.trim(), description: description.trim() || null })
      setName('')
      setDescription('')
    })

  return (
    <div className="p-6 space-y-4 max-w-3xl">
      <h1 className="text-xl font-semibold">Темы</h1>

      <div className="card p-4 flex gap-3 items-end flex-wrap">
        <div className="flex-1 min-w-[180px]">
          <label className="label">Тема</label>
          <input className="input" value={name} onChange={(e) => setName(e.target.value)} placeholder="Go / Golang" />
        </div>
        <div className="flex-[2] min-w-[240px]">
          <label className="label">Описание</label>
          <input className="input" value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Горутины, каналы, интерфейсы" />
        </div>
        <button className="btn-primary" onClick={add} disabled={!name.trim()}>
          <Plus size={16} />
          Добавить
        </button>
      </div>

      {error && (
        <div className="card px-4 py-2.5 text-sm text-danger flex items-center">
          {error}
          <button className="btn-ghost ml-auto py-1" onClick={() => setError(null)}>Закрыть</button>
        </div>
      )}

      <div className="card divide-y divide-slate-100">
        {topics.map((t) => (
          <div key={t.id} className={`flex items-center gap-3 px-4 py-3 ${t.is_archived ? 'opacity-50' : ''}`}>
            <div className="h-2.5 w-2.5 rounded-full bg-primary shrink-0" />
            <div className="min-w-0">
              <div className="font-medium">{t.name}</div>
              {t.description && <div className="text-xs text-muted truncate">{t.description}</div>}
            </div>
            <div className="ml-auto flex gap-1">
              <button
                className="btn-ghost p-1.5"
                title={t.is_archived ? 'Вернуть из архива' : 'В архив'}
                onClick={() =>
                  void run(() =>
                    api.updateTopic(t.id, { name: t.name, description: t.description, is_archived: !t.is_archived }),
                  )
                }
              >
                {t.is_archived ? <ArchiveRestore size={15} /> : <Archive size={15} />}
              </button>
              <button
                className="btn-ghost p-1.5 hover:text-danger"
                title="Удалить"
                onClick={() => {
                  if (confirm(`Удалить тему «${t.name}»? Задачи останутся без темы.`)) {
                    void run(() => api.deleteTopic(t.id))
                  }
                }}
              >
                <Trash2 size={15} />
              </button>
            </div>
          </div>
        ))}
        {topics.length === 0 && <div className="px-4 py-10 text-center text-muted text-sm">Тем пока нет.</div>}
      </div>
    </div>
  )
}
