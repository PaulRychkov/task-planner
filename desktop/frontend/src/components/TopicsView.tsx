import { useMemo, useState } from 'react'
import { Archive, ArchiveRestore, Check, ChevronDown, ChevronRight, Pencil, Plus, Trash2, X } from 'lucide-react'
import { api } from '../api'
import type { Topic } from '../types'

interface Props {
  topics: Topic[]
  onDataChanged: () => Promise<void> | void
}

interface TopicNode {
  topic: Topic
  children: TopicNode[]
}

function buildTree(topics: Topic[]): TopicNode[] {
  const nodes = new Map<string, TopicNode>()
  for (const t of topics) nodes.set(t.id, { topic: t, children: [] })
  const roots: TopicNode[] = []
  for (const node of nodes.values()) {
    const pid = node.topic.parent_id
    if (pid && nodes.has(pid)) nodes.get(pid)!.children.push(node)
    else roots.push(node)
  }
  const sortRec = (list: TopicNode[]) => {
    list.sort((a, b) => a.topic.name.localeCompare(b.topic.name, 'ru'))
    list.forEach((n) => sortRec(n.children))
  }
  sortRec(roots)
  return roots
}

function descendantIds(node: TopicNode): Set<string> {
  const out = new Set<string>([node.topic.id])
  const walk = (n: TopicNode) => {
    for (const c of n.children) {
      out.add(c.topic.id)
      walk(c)
    }
  }
  walk(node)
  return out
}

function flatten(roots: TopicNode[]): { topic: Topic; depth: number }[] {
  const out: { topic: Topic; depth: number }[] = []
  const walk = (list: TopicNode[], depth: number) => {
    for (const n of list) {
      out.push({ topic: n.topic, depth })
      walk(n.children, depth + 1)
    }
  }
  walk(roots, 0)
  return out
}

export default function TopicsView({ topics, onDataChanged }: Props) {
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [parentID, setParentID] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set())
  const [editing, setEditing] = useState<{ id: string; name: string; description: string; parent_id: string } | null>(null)

  const roots = useMemo(() => buildTree(topics), [topics])
  const flat = useMemo(() => flatten(roots), [roots])

  const toggle = (id: string) =>
    setCollapsed((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })

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
      await api.createTopic({
        name: name.trim(),
        parent_id: parentID || null,
        description: description.trim() || null,
      })
      setName('')
      setDescription('')
      setParentID('')
    })

  const saveEdit = () => {
    if (!editing) return
    const e = editing
    setEditing(null)
    void run(() =>
      api.updateTopic(e.id, {
        name: e.name.trim(),
        parent_id: e.parent_id || null,
        description: e.description.trim() || null,
      }),
    )
  }

  const renderNode = (node: TopicNode, depth: number) => {
    const t = node.topic
    const isCollapsed = collapsed.has(t.id)
    const hasKids = node.children.length > 0
    const isEditing = editing?.id === t.id
    const forbidden = descendantIds(node)

    return (
      <div key={t.id}>
        <div
          className={`flex items-center gap-1.5 border-b border-slate-100 py-2 pr-3 hover:bg-slate-50/60 ${
            t.is_archived ? 'opacity-50' : ''
          }`}
          style={{ paddingLeft: `${depth * 22 + 10}px` }}
        >
          <button
            className="flex h-5 w-5 shrink-0 items-center justify-center text-muted hover:text-ink disabled:opacity-0"
            onClick={() => toggle(t.id)}
            disabled={!hasKids}
          >
            {hasKids ? (isCollapsed ? <ChevronRight size={15} /> : <ChevronDown size={15} />) : null}
          </button>

          {isEditing ? (
            <div className="flex min-w-0 flex-1 items-center gap-2">
              <input
                autoFocus
                className="input py-1 text-sm"
                value={editing.name}
                onChange={(e) => setEditing({ ...editing, name: e.target.value })}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') saveEdit()
                  if (e.key === 'Escape') setEditing(null)
                }}
              />
              <input
                className="input py-1 text-sm"
                placeholder="Описание"
                value={editing.description}
                onChange={(e) => setEditing({ ...editing, description: e.target.value })}
              />
              <select
                className="input py-1 text-sm"
                value={editing.parent_id}
                onChange={(e) => setEditing({ ...editing, parent_id: e.target.value })}
              >
                <option value="">— корневая —</option>
                {flat
                  .filter(({ topic }) => !forbidden.has(topic.id))
                  .map(({ topic, depth: d }) => (
                    <option key={topic.id} value={topic.id}>
                      {' '.repeat(d * 3)}
                      {topic.name}
                    </option>
                  ))}
              </select>
              <button className="btn-ghost p-1.5 text-success" title="Сохранить" onClick={saveEdit}>
                <Check size={15} />
              </button>
              <button className="btn-ghost p-1.5" title="Отмена" onClick={() => setEditing(null)}>
                <X size={15} />
              </button>
            </div>
          ) : (
            <>
              <div className="min-w-0 flex-1">
                <div className={`font-medium ${t.is_archived ? 'line-through' : ''}`}>{t.name}</div>
                {t.description && <div className="truncate text-xs text-muted">{t.description}</div>}
              </div>
              <div className="flex shrink-0 gap-1">
                <button
                  className="btn-ghost p-1.5"
                  title="Редактировать"
                  onClick={() =>
                    setEditing({
                      id: t.id,
                      name: t.name,
                      description: t.description ?? '',
                      parent_id: t.parent_id ?? '',
                    })
                  }
                >
                  <Pencil size={15} />
                </button>
                <button
                  className="btn-ghost p-1.5"
                  title={t.is_archived ? 'Вернуть из архива' : 'В архив'}
                  onClick={() =>
                    void run(() =>
                      api.updateTopic(t.id, {
                        name: t.name,
                        parent_id: t.parent_id,
                        description: t.description,
                        is_archived: !t.is_archived,
                      }),
                    )
                  }
                >
                  {t.is_archived ? <ArchiveRestore size={15} /> : <Archive size={15} />}
                </button>
                <button
                  className="btn-ghost p-1.5 hover:text-danger"
                  title="Удалить"
                  onClick={() => {
                    if (confirm(`Удалить тему «${t.name}»? Задачи и подтемы останутся без неё.`)) {
                      void run(() => api.deleteTopic(t.id))
                    }
                  }}
                >
                  <Trash2 size={15} />
                </button>
              </div>
            </>
          )}
        </div>
        {!isCollapsed && node.children.map((c) => renderNode(c, depth + 1))}
      </div>
    )
  }

  return (
    <div className="p-6 space-y-4 max-w-4xl">
      <h1 className="text-xl font-semibold">Темы</h1>

      <div className="card p-4 flex gap-3 items-end flex-wrap">
        <div className="flex-1 min-w-[180px]">
          <label className="label">Тема</label>
          <input className="input" value={name} onChange={(e) => setName(e.target.value)} placeholder="Поиск работы" />
        </div>
        <div className="flex-1 min-w-[180px]">
          <label className="label">Родительская тема</label>
          <select className="input" value={parentID} onChange={(e) => setParentID(e.target.value)}>
            <option value="">— корневая —</option>
            {flat.map(({ topic, depth }) => (
              <option key={topic.id} value={topic.id}>
                {' '.repeat(depth * 3)}
                {topic.name}
              </option>
            ))}
          </select>
        </div>
        <div className="flex-[2] min-w-[240px]">
          <label className="label">Описание</label>
          <input
            className="input"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Подготовка к собеседованиям"
          />
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

      <div className="card overflow-hidden">
        {roots.map((n) => renderNode(n, 0))}
        {topics.length === 0 && <div className="px-4 py-10 text-center text-muted text-sm">Тем пока нет.</div>}
      </div>
    </div>
  )
}
