package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/PaulRychkov/task-planner/backend/internal/models"
)

var (
	ErrNotFound = errors.New("record not found")
	ErrConflict = errors.New("unique constraint violation")
)

type OccurrenceFilter struct {
	TaskID *uuid.UUID
	From   *models.Date
	To     *models.Date
	Status *models.OccurrenceStatus
}

type CommittedPlanRef struct {
	PlanID       uuid.UUID
	PlanDate     models.Date
	OccurrenceID uuid.UUID
}

type Store interface {
	InTx(ctx context.Context, fn func(Store) error) error
	Ping(ctx context.Context) error
	Topics() TopicRepo
	Tasks() TaskRepo
	Occurrences() OccurrenceRepo
	Plans() PlanRepo
	Outbox() OutboxRepo
}

type TopicRepo interface {
	List(ctx context.Context, includeArchived bool) ([]models.Topic, error)
	Get(ctx context.Context, id uuid.UUID) (*models.Topic, error)
	GetByName(ctx context.Context, name string) (*models.Topic, error)
	Create(ctx context.Context, t *models.Topic) error
	Update(ctx context.Context, t *models.Topic) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type TaskRepo interface {
	List(ctx context.Context) ([]models.Task, error)
	ListActive(ctx context.Context) ([]models.Task, error)
	ListOpenWithDue(ctx context.Context, from, to models.Date) ([]models.Task, error)
	Get(ctx context.Context, id uuid.UUID) (*models.Task, error)
	Create(ctx context.Context, t *models.Task) error
	Update(ctx context.Context, t *models.Task) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type OccurrenceRepo interface {
	Get(ctx context.Context, id uuid.UUID) (*models.TaskOccurrence, error)
	List(ctx context.Context, f OccurrenceFilter) ([]models.TaskOccurrence, error)
	CompletedSteps(ctx context.Context, taskID uuid.UUID) ([]int, error)
	InsertIgnoreConflict(ctx context.Context, o *models.TaskOccurrence) (bool, error)
	Update(ctx context.Context, o *models.TaskOccurrence) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type PlanRepo interface {
	GetByDate(ctx context.Context, date models.Date) (*models.DayPlan, error)
	Create(ctx context.Context, p *models.DayPlan) error
	Update(ctx context.Context, p *models.DayPlan) error
	ReplaceItems(ctx context.Context, planID uuid.UUID, items []models.DayPlanItem) error
	CommittedRefsForOccurrences(ctx context.Context, occurrenceIDs []uuid.UUID) ([]CommittedPlanRef, error)
}

type OutboxRepo interface {
	Add(ctx context.Context, e *models.OutboxEvent) error
	ListUnpublished(ctx context.Context, limit int) ([]models.OutboxEvent, error)
	MarkPublished(ctx context.Context, id uuid.UUID, at time.Time) error
	MarkFailed(ctx context.Context, id uuid.UUID, msg string) error
}

func EnsureID(id *uuid.UUID) {
	if *id == uuid.Nil {
		*id = uuid.New()
	}
}
