import type { Occurrence, RescheduleResult, Task, TaskInput, Topic } from './types'

declare global {
  interface Window {
    go?: {
      main?: {
        App?: {
          GetBackendURL(): Promise<string>
        }
      }
    }
  }
}

let baseURL: string | null = null

async function base(): Promise<string> {
  if (baseURL !== null) return baseURL
  if (window.go?.main?.App?.GetBackendURL) {
    baseURL = await window.go.main.App.GetBackendURL()
    return baseURL
  }
  if (window.location.protocol.startsWith('http')) {
    try {
      const probe = await fetch('/healthz')
      if (probe.ok) {
        baseURL = ''
        return baseURL
      }
    } catch {
      /* dev-сервер без бэкенда на том же origin */
    }
  }
  baseURL = 'http://localhost:8081'
  return baseURL
}

export class ApiError extends Error {
  code: string
  constructor(code: string, message: string) {
    super(message)
    this.code = code
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const url = (await base()) + path
  const res = await fetch(url, {
    method,
    headers: body !== undefined ? { 'Content-Type': 'application/json' } : undefined,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  if (res.status === 204) return undefined as T
  const text = await res.text()
  const data = text ? JSON.parse(text) : undefined
  if (!res.ok) {
    const err = data?.error
    throw new ApiError(err?.code ?? String(res.status), err?.message ?? res.statusText)
  }
  return data as T
}

export const api = {
  listTopics: (includeArchived = false) =>
    request<Topic[]>('GET', `/api/v1/topics?include_archived=${includeArchived}`),
  createTopic: (input: { name: string; parent_id?: string | null; description?: string | null }) =>
    request<Topic>('POST', '/api/v1/topics', input),
  updateTopic: (id: string, input: { name: string; parent_id?: string | null; description?: string | null; is_archived?: boolean }) =>
    request<Topic>('PUT', `/api/v1/topics/${id}`, input),
  deleteTopic: (id: string) => request<void>('DELETE', `/api/v1/topics/${id}`),

  listTasks: () => request<Task[]>('GET', '/api/v1/tasks'),
  createTask: (input: TaskInput) => request<Task>('POST', '/api/v1/tasks', input),
  updateTask: (id: string, input: TaskInput) => request<Task>('PUT', `/api/v1/tasks/${id}`, input),
  deleteTask: (id: string) => request<void>('DELETE', `/api/v1/tasks/${id}`),
  rescheduleMissed: (taskId: string) =>
    request<RescheduleResult>('POST', `/api/v1/tasks/${taskId}/reschedule-missed`),

  listOccurrences: (from: string, to: string, status?: string) =>
    request<Occurrence[]>(
      'GET',
      `/api/v1/occurrences?from=${from}&to=${to}${status ? `&status=${status}` : ''}`,
    ),
  completeOccurrence: (id: string) => request<Occurrence>('POST', `/api/v1/occurrences/${id}/complete`),
  skipOccurrence: (id: string) => request<Occurrence>('POST', `/api/v1/occurrences/${id}/skip`),
  addProgress: (id: string, minutes: number) =>
    request<Occurrence>('POST', `/api/v1/occurrences/${id}/progress`, { minutes }),

  health: async (): Promise<boolean> => {
    try {
      await request<{ status: string }>('GET', '/healthz')
      return true
    } catch {
      return false
    }
  },
}
