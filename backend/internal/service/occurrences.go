package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/PaulRychkov/task-planner/backend/internal/models"
	"github.com/PaulRychkov/task-planner/backend/internal/repository"
)

type OccurrenceService struct {
	store repository.Store
	clock Clock
	loc   *time.Location
}

func NewOccurrenceService(store repository.Store, clock Clock, loc *time.Location) *OccurrenceService {
	return &OccurrenceService{store: store, clock: clock, loc: loc}
}

func (s *OccurrenceService) List(ctx context.Context, from, to *models.Date, status *models.OccurrenceStatus) ([]models.TaskOccurrence, error) {
	return s.ListForTask(ctx, nil, from, to, status)
}

// ListForTask — то же, что List, с необязательным фильтром по задаче (GET /occurrences?task_id=…):
// клиентам вроде study-mcp не нужно выкачивать вхождения всех задач ради одной.
func (s *OccurrenceService) ListForTask(ctx context.Context, taskID *uuid.UUID, from, to *models.Date, status *models.OccurrenceStatus) ([]models.TaskOccurrence, error) {
	return s.store.Occurrences().List(ctx, repository.OccurrenceFilter{TaskID: taskID, From: from, To: to, Status: status})
}

func (s *OccurrenceService) Complete(ctx context.Context, id uuid.UUID) (*models.TaskOccurrence, error) {
	var result *models.TaskOccurrence
	err := s.store.InTx(ctx, func(tx repository.Store) error {
		occ, err := tx.Occurrences().Get(ctx, id)
		if err != nil {
			return err
		}
		if occ.Status != models.OccurrencePending && occ.Status != models.OccurrenceMissed {
			return conflict(fmt.Sprintf("cannot complete occurrence in status %q", occ.Status))
		}
		task := occ.Task
		if task == nil {
			return fmt.Errorf("occurrence %s has no task loaded", id)
		}
		now := s.clock.Now()
		occ.Task = nil
		occ.Status = models.OccurrenceCompleted
		occ.CompletedAt = &now
		if err := tx.Occurrences().Update(ctx, occ); err != nil {
			return err
		}
		if err := emitOccurrenceCompleted(ctx, tx, occ, task); err != nil {
			return err
		}
		if s.closesTask(task, occ) && task.Progress.Open() {
			task.Progress = models.ProgressCompleted
			if err := tx.Tasks().Update(ctx, task); err != nil {
				return err
			}
			if err := emitTaskEvent(ctx, tx, "task.completed", task); err != nil {
				return err
			}
		}
		occ.Task = task
		result = occ
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *OccurrenceService) AddProgress(ctx context.Context, id uuid.UUID, deltaMinutes int) (*models.TaskOccurrence, error) {
	if deltaMinutes == 0 {
		return nil, invalid("minutes must be non-zero")
	}
	var result *models.TaskOccurrence
	err := s.store.InTx(ctx, func(tx repository.Store) error {
		occ, err := tx.Occurrences().Get(ctx, id)
		if err != nil {
			return err
		}
		if occ.Status == models.OccurrenceSkipped || occ.Status == models.OccurrenceRescheduled {
			return conflict(fmt.Sprintf("cannot log progress on occurrence in status %q", occ.Status))
		}
		task := occ.Task
		occ.Task = nil
		occ.ProgressMinutes += deltaMinutes
		if occ.ProgressMinutes < 0 {
			occ.ProgressMinutes = 0
		}
		if err := tx.Occurrences().Update(ctx, occ); err != nil {
			return err
		}
		occ.Task = task
		result = occ
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *OccurrenceService) closesTask(task *models.Task, occ *models.TaskOccurrence) bool {
	if task.RecurrenceKind == models.RecurrenceOnce {
		return true
	}
	if task.RecurrenceKind == models.RecurrenceSpacedRepetition &&
		task.RecurrenceParams != nil && occ.SeriesStep != nil {
		return *occ.SeriesStep == len(task.RecurrenceParams.Intervals)-1
	}
	return false
}

func (s *OccurrenceService) Skip(ctx context.Context, id uuid.UUID) (*models.TaskOccurrence, error) {
	var result *models.TaskOccurrence
	err := s.store.InTx(ctx, func(tx repository.Store) error {
		occ, err := tx.Occurrences().Get(ctx, id)
		if err != nil {
			return err
		}
		if occ.Status != models.OccurrencePending {
			return conflict(fmt.Sprintf("cannot skip occurrence in status %q", occ.Status))
		}
		task := occ.Task
		if task == nil {
			return fmt.Errorf("occurrence %s has no task loaded", id)
		}
		occ.Task = nil
		occ.Status = models.OccurrenceSkipped
		if err := tx.Occurrences().Update(ctx, occ); err != nil {
			return err
		}
		if err := emitOccurrenceSkipped(ctx, tx, occ, task); err != nil {
			return err
		}
		occ.Task = task
		result = occ
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
