package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type RecurrenceKind string

const (
	RecurrenceOnce             RecurrenceKind = "once"
	RecurrenceDaily            RecurrenceKind = "daily"
	RecurrenceWeekdays         RecurrenceKind = "weekdays"
	RecurrenceWeekends         RecurrenceKind = "weekends"
	RecurrenceDaysOfWeek       RecurrenceKind = "days_of_week"
	RecurrenceEveryNDays       RecurrenceKind = "every_n_days"
	RecurrenceEveryNWeeks      RecurrenceKind = "every_n_weeks"
	RecurrenceMonthly          RecurrenceKind = "monthly"
	RecurrenceSpacedRepetition RecurrenceKind = "spaced_repetition"
)

func (k RecurrenceKind) Valid() bool {
	switch k {
	case RecurrenceOnce, RecurrenceDaily, RecurrenceWeekdays, RecurrenceWeekends,
		RecurrenceDaysOfWeek, RecurrenceEveryNDays, RecurrenceEveryNWeeks,
		RecurrenceMonthly, RecurrenceSpacedRepetition:
		return true
	}
	return false
}

func (k RecurrenceKind) RequiresParams() bool {
	switch k {
	case RecurrenceOnce, RecurrenceDaily, RecurrenceWeekdays, RecurrenceWeekends:
		return false
	}
	return true
}

type TaskProgress string

const (
	ProgressNeedsAction TaskProgress = "needs_action"
	ProgressInProcess   TaskProgress = "in_process"
	ProgressCompleted   TaskProgress = "completed"
	ProgressCancelled   TaskProgress = "cancelled"
)

func (p TaskProgress) Valid() bool {
	switch p {
	case ProgressNeedsAction, ProgressInProcess, ProgressCompleted, ProgressCancelled:
		return true
	}
	return false
}

func (p TaskProgress) Open() bool {
	return p == ProgressNeedsAction || p == ProgressInProcess
}

type OccurrenceStatus string

const (
	OccurrencePending     OccurrenceStatus = "pending"
	OccurrenceCompleted   OccurrenceStatus = "completed"
	OccurrenceMissed      OccurrenceStatus = "missed"
	OccurrenceRescheduled OccurrenceStatus = "rescheduled"
	OccurrenceSkipped     OccurrenceStatus = "skipped"
)

func (s OccurrenceStatus) Valid() bool {
	switch s {
	case OccurrencePending, OccurrenceCompleted, OccurrenceMissed, OccurrenceRescheduled, OccurrenceSkipped:
		return true
	}
	return false
}

type ActorKind string

const (
	ActorApp   ActorKind = "app"
	ActorBot   ActorKind = "bot"
	ActorAgent ActorKind = "agent"
)

func (a ActorKind) Valid() bool {
	switch a {
	case ActorApp, ActorBot, ActorAgent:
		return true
	}
	return false
}

type RecurrenceParams struct {
	Days       []int `json:"days,omitempty"`
	N          *int  `json:"n,omitempty"`
	DayOfMonth *int  `json:"day_of_month,omitempty"`
	Intervals  []int `json:"intervals,omitempty"`
}

func (p RecurrenceParams) Value() (driver.Value, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal recurrence params: %w", err)
	}
	return string(b), nil
}

func (p *RecurrenceParams) Scan(v any) error {
	switch t := v.(type) {
	case []byte:
		return json.Unmarshal(t, p)
	case string:
		return json.Unmarshal([]byte(t), p)
	default:
		return fmt.Errorf("cannot scan %T into RecurrenceParams", v)
	}
}

func (RecurrenceParams) GormDataType() string {
	return "jsonb"
}

type JSON json.RawMessage

func (j JSON) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return string(j), nil
}

func (j *JSON) Scan(v any) error {
	switch t := v.(type) {
	case []byte:
		*j = JSON(append([]byte(nil), t...))
		return nil
	case string:
		*j = JSON(t)
		return nil
	default:
		return fmt.Errorf("cannot scan %T into JSON", v)
	}
}

func (j JSON) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return j, nil
}

func (j *JSON) UnmarshalJSON(b []byte) error {
	*j = JSON(append([]byte(nil), b...))
	return nil
}

func (JSON) GormDataType() string {
	return "jsonb"
}

type Topic struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	ParentID    *uuid.UUID `gorm:"type:uuid" json:"parent_id"`
	Name        string     `json:"name"`
	Description *string    `json:"description"`
	IsArchived  bool       `json:"is_archived"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (Topic) TableName() string { return "topics" }

type Task struct {
	ID                       uuid.UUID         `gorm:"type:uuid;primaryKey" json:"id"`
	TopicID                  *uuid.UUID        `gorm:"type:uuid" json:"topic_id"`
	Title                    string            `json:"title"`
	Description              *string           `json:"description"`
	Source                   *string           `json:"source"`
	ExternalID               *string           `json:"external_id"`
	RecurrenceKind           RecurrenceKind    `json:"recurrence_kind"`
	RecurrenceParams         *RecurrenceParams `gorm:"type:jsonb" json:"recurrence_params"`
	StartDate                Date              `gorm:"type:date" json:"start_date"`
	Due                      *Date             `gorm:"type:date" json:"due"`
	StartTimeMinutes         *int              `json:"start_time_minutes"`
	EstimatedDurationMinutes *int              `json:"estimated_duration_minutes"`
	EffortMinutes            *int              `json:"effort_minutes"`
	AllDay                   bool              `json:"all_day"`
	RequiresPomodoro         bool              `gorm:"not null" json:"requires_pomodoro"`
	Reschedulable            bool              `gorm:"not null;default:false" json:"reschedulable"`
	Priority                 int               `json:"priority"`
	Progress                 TaskProgress      `json:"progress"`
	IsActive                 bool              `json:"is_active"`
	CreatedAt                time.Time         `json:"created_at"`
	UpdatedAt                time.Time         `json:"updated_at"`
	Topic                    *Topic            `gorm:"foreignKey:TopicID" json:"topic,omitempty"`
}

func (Task) TableName() string { return "tasks" }

const (
	MinPriority = 1
	MaxPriority = 5
)

func PriorityWeightPercent(priority int) int {
	if priority < MinPriority {
		priority = MinPriority
	}
	if priority > MaxPriority {
		priority = MaxPriority
	}
	return 100 + (priority-MinPriority)*50
}

type SyncTombstone struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	Table     string    `gorm:"column:table_name;not null" json:"table"`
	RowID     uuid.UUID `gorm:"type:uuid;not null" json:"row_id"`
	DeletedAt time.Time `gorm:"not null" json:"deleted_at"`
}

func (SyncTombstone) TableName() string { return "sync_tombstones" }

type SyncState struct {
	Key   string `gorm:"primaryKey"`
	Value string `gorm:"not null"`
}

func (SyncState) TableName() string { return "sync_state" }

func (t Task) TopicName() string {
	if t.Topic == nil {
		return ""
	}
	return t.Topic.Name
}

type TaskOccurrence struct {
	ID            uuid.UUID        `gorm:"type:uuid;primaryKey" json:"id"`
	TaskID        uuid.UUID        `gorm:"type:uuid" json:"task_id"`
	Date            Date             `gorm:"type:date" json:"date"`
	Status          OccurrenceStatus `json:"status"`
	ProgressMinutes int              `gorm:"not null;default:0" json:"progress_minutes"`
	SeriesStep      *int             `json:"series_step"`
	CompletedAt   *time.Time       `json:"completed_at"`
	RescheduledTo *Date            `gorm:"type:date" json:"rescheduled_to"`
	CreatedAt     time.Time        `json:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at"`
	Task          *Task            `gorm:"foreignKey:TaskID" json:"task,omitempty"`
}

func (TaskOccurrence) TableName() string { return "task_occurrences" }

type DayPlan struct {
	ID          uuid.UUID     `gorm:"type:uuid;primaryKey" json:"id"`
	Date        Date          `gorm:"type:date" json:"date"`
	CommittedAt *time.Time    `json:"committed_at"`
	CommittedBy *ActorKind    `json:"committed_by"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	Items       []DayPlanItem `gorm:"foreignKey:PlanID" json:"items"`
}

func (DayPlan) TableName() string { return "day_plans" }

func (p DayPlan) Committed() bool { return p.CommittedAt != nil }

type DayPlanItem struct {
	ID                  uuid.UUID       `gorm:"type:uuid;primaryKey" json:"id"`
	PlanID              uuid.UUID       `gorm:"type:uuid" json:"plan_id"`
	OccurrenceID        uuid.UUID       `gorm:"type:uuid" json:"occurrence_id"`
	PlannedStartMinutes *int            `json:"planned_start_minutes"`
	Position            int             `json:"position"`
	CreatedAt           time.Time       `json:"created_at"`
	Occurrence          *TaskOccurrence `gorm:"foreignKey:OccurrenceID" json:"occurrence,omitempty"`
}

func (DayPlanItem) TableName() string { return "day_plan_items" }

type OutboxEvent struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	EventType     string     `json:"event_type"`
	AggregateType string     `json:"aggregate_type"`
	AggregateID   uuid.UUID  `gorm:"type:uuid" json:"aggregate_id"`
	Payload       JSON       `gorm:"type:jsonb" json:"payload"`
	CreatedAt     time.Time  `json:"created_at"`
	PublishedAt   *time.Time `json:"published_at"`
	Attempts      int        `json:"attempts"`
	LastError     *string    `json:"last_error"`
}

func (OutboxEvent) TableName() string { return "events_outbox" }
