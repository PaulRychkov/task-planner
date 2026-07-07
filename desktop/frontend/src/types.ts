export type RecurrenceKind =
  | 'once'
  | 'daily'
  | 'weekdays'
  | 'weekends'
  | 'days_of_week'
  | 'every_n_days'
  | 'every_n_weeks'
  | 'monthly'
  | 'spaced_repetition'

export type OccurrenceStatus = 'pending' | 'completed' | 'missed' | 'rescheduled' | 'skipped'

export type TaskProgress = 'needs_action' | 'in_process' | 'completed' | 'cancelled'

export interface RecurrenceParams {
  days?: number[]
  n?: number
  day_of_month?: number | null
  intervals?: number[]
}

export interface Topic {
  id: string
  name: string
  description: string | null
  is_archived: boolean
  created_at: string
  updated_at: string
}

export interface Task {
  id: string
  topic_id: string | null
  title: string
  description: string | null
  source: string | null
  external_id: string | null
  recurrence_kind: RecurrenceKind
  recurrence_params: RecurrenceParams | null
  start_date: string
  due: string | null
  start_time_minutes: number | null
  estimated_duration_minutes: number | null
  all_day: boolean
  priority: number
  progress: TaskProgress
  is_active: boolean
  topic?: Topic | null
}

export interface Occurrence {
  id: string
  task_id: string
  date: string
  status: OccurrenceStatus
  series_step: number | null
  completed_at: string | null
  rescheduled_to: string | null
  task?: Task | null
}

export interface DayPlanItem {
  id: string
  plan_id: string
  occurrence_id: string
  planned_start_minutes: number | null
  position: number
  occurrence?: Occurrence | null
}

export interface DayPlan {
  id: string
  date: string
  committed_at: string | null
  committed_by: string | null
  items: DayPlanItem[]
}

export interface TaskInput {
  title: string
  description?: string | null
  topic_id?: string | null
  recurrence_kind: RecurrenceKind
  recurrence_params?: RecurrenceParams | null
  start_date?: string
  due?: string | null
  start_time_minutes?: number | null
  estimated_duration_minutes?: number | null
  all_day?: boolean
  priority?: number
  is_active?: boolean
}

export interface RescheduleResult {
  task_id: string
  shift_days: number
  rescheduled: number
}
