package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/PaulRychkov/task-planner/backend/internal/models"
	"github.com/PaulRychkov/task-planner/backend/internal/repository"
)

type TaskInput struct {
	Title                    string                   `json:"title"`
	Description              *string                  `json:"description"`
	TopicID                  *uuid.UUID               `json:"topic_id"`
	Source                   *string                  `json:"source"`
	ExternalID               *string                  `json:"external_id"`
	RecurrenceKind           models.RecurrenceKind    `json:"recurrence_kind"`
	RecurrenceParams         *models.RecurrenceParams `json:"recurrence_params"`
	StartDate                *models.Date             `json:"start_date"`
	Due                      *models.Date             `json:"due"`
	StartTimeMinutes         *int                     `json:"start_time_minutes"`
	EstimatedDurationMinutes *int                     `json:"estimated_duration_minutes"`
	EffortMinutes            *int                     `json:"effort_minutes"`
	AllDay                   *bool                    `json:"all_day"`
	RequiresPomodoro         *bool                    `json:"requires_pomodoro"`
	Reschedulable            *bool                    `json:"reschedulable"`
	Priority                 *int                     `json:"priority"`
	Progress                 *models.TaskProgress     `json:"progress"`
	IsActive                 *bool                    `json:"is_active"`
}

type RescheduleResult struct {
	TaskID      uuid.UUID `json:"task_id"`
	ShiftDays   int       `json:"shift_days"`
	Rescheduled int       `json:"rescheduled"`
}

type TaskService struct {
	store      repository.Store
	clock      Clock
	loc        *time.Location
	windowDays int
}

func NewTaskService(store repository.Store, clock Clock, loc *time.Location, windowDays int) *TaskService {
	return &TaskService{store: store, clock: clock, loc: loc, windowDays: windowDays}
}

func (s *TaskService) today() models.Date {
	return Today(s.clock, s.loc)
}

func (s *TaskService) WindowDays() int {
	return s.windowDays
}

func (s *TaskService) List(ctx context.Context) ([]models.Task, error) {
	return s.store.Tasks().List(ctx)
}

func (s *TaskService) Get(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	return s.store.Tasks().Get(ctx, id)
}

func (s *TaskService) ListOpenWithDue(ctx context.Context, from, to models.Date) ([]models.Task, error) {
	return s.store.Tasks().ListOpenWithDue(ctx, from, to)
}

func (s *TaskService) validateInput(in TaskInput) error {
	if in.Title == "" {
		return invalid("title is required")
	}
	if err := ValidateRecurrence(in.RecurrenceKind, in.RecurrenceParams); err != nil {
		return err
	}
	if in.Due != nil && in.RecurrenceKind != models.RecurrenceOnce {
		return invalid("due is allowed only for once tasks")
	}
	if in.StartTimeMinutes != nil && (*in.StartTimeMinutes < 0 || *in.StartTimeMinutes > 1439) {
		return invalid("start_time_minutes out of range 0..1439")
	}
	if in.EstimatedDurationMinutes != nil && *in.EstimatedDurationMinutes <= 0 {
		return invalid("estimated_duration_minutes must be positive")
	}
	if in.Priority != nil && (*in.Priority < models.MinPriority || *in.Priority > models.MaxPriority) {
		return invalid("priority out of range 1..5")
	}
	if in.StartTimeMinutes != nil && in.EffortMinutes != nil {
		return invalid("start_time_minutes and effort_minutes are mutually exclusive: fixed-time events have no effort budget")
	}
	if in.EstimatedDurationMinutes != nil && in.EffortMinutes != nil {
		return invalid("estimated_duration_minutes and effort_minutes are mutually exclusive: duration belongs to fixed-time events, effort to regular tasks")
	}
	if in.EffortMinutes != nil && *in.EffortMinutes <= 0 {
		return invalid("effort_minutes must be positive")
	}
	if in.Due != nil {
		start := s.today()
		if in.StartDate != nil {
			start = *in.StartDate
		}
		if in.Due.Before(start) {
			return invalid("due must not be earlier than start_date")
		}
	}
	if (in.Source == nil) != (in.ExternalID == nil) {
		return invalid("source and external_id must be set together")
	}
	if in.Progress != nil && !in.Progress.Valid() {
		return invalid(fmt.Sprintf("unknown progress %q", *in.Progress))
	}
	return nil
}

func (s *TaskService) Create(ctx context.Context, in TaskInput) (*models.Task, error) {
	if err := s.validateInput(in); err != nil {
		return nil, err
	}
	task := &models.Task{
		Title:                    in.Title,
		Description:              in.Description,
		TopicID:                  in.TopicID,
		Source:                   in.Source,
		ExternalID:               in.ExternalID,
		RecurrenceKind:           in.RecurrenceKind,
		RecurrenceParams:         in.RecurrenceParams,
		StartDate:                s.today(),
		Due:                      in.Due,
		StartTimeMinutes:         in.StartTimeMinutes,
		EstimatedDurationMinutes: in.EstimatedDurationMinutes,
		EffortMinutes:            in.EffortMinutes,
		Priority:                 models.MinPriority,
		Progress:                 models.ProgressNeedsAction,
		IsActive:                 true,
		RequiresPomodoro:         true,
		Reschedulable:            in.RecurrenceKind == models.RecurrenceOnce || in.RecurrenceKind == models.RecurrenceSpacedRepetition,
	}
	if in.StartDate != nil {
		task.StartDate = *in.StartDate
	}
	if in.AllDay != nil {
		task.AllDay = *in.AllDay
	}
	if in.RequiresPomodoro != nil {
		task.RequiresPomodoro = *in.RequiresPomodoro
	}
	if in.Reschedulable != nil {
		task.Reschedulable = *in.Reschedulable
	}
	if in.Priority != nil {
		task.Priority = *in.Priority
	}
	if in.IsActive != nil {
		task.IsActive = *in.IsActive
	}
	if in.Progress != nil {
		task.Progress = *in.Progress
	}

	err := s.store.InTx(ctx, func(tx repository.Store) error {
		if task.TopicID != nil {
			if _, err := tx.Topics().Get(ctx, *task.TopicID); err != nil {
				if IsNotFound(err) {
					return invalid("topic not found")
				}
				return err
			}
		}
		if err := tx.Tasks().Create(ctx, task); err != nil {
			return fmt.Errorf("create task: %w", err)
		}
		loaded, err := tx.Tasks().Get(ctx, task.ID)
		if err != nil {
			return err
		}
		*task = *loaded
		if err := s.regenerate(ctx, tx, task); err != nil {
			return err
		}
		return emitTaskEvent(ctx, tx, "task.created", task)
	})
	if err != nil {
		return nil, err
	}
	return task, nil
}

func (s *TaskService) Update(ctx context.Context, id uuid.UUID, in TaskInput) (*models.Task, error) {
	var updated *models.Task
	err := s.store.InTx(ctx, func(tx repository.Store) error {
		task, err := tx.Tasks().Get(ctx, id)
		if err != nil {
			return err
		}
		if err := s.validateInput(in); err != nil {
			return err
		}
		prevProgress := task.Progress

		task.Title = in.Title
		task.Description = in.Description
		task.TopicID = in.TopicID
		task.RecurrenceKind = in.RecurrenceKind
		task.RecurrenceParams = in.RecurrenceParams
		if in.StartDate != nil {
			task.StartDate = *in.StartDate
		}
		task.Due = in.Due
		task.StartTimeMinutes = in.StartTimeMinutes
		task.EstimatedDurationMinutes = in.EstimatedDurationMinutes
		task.EffortMinutes = in.EffortMinutes
		if in.AllDay != nil {
			task.AllDay = *in.AllDay
		}
		if in.RequiresPomodoro != nil {
			task.RequiresPomodoro = *in.RequiresPomodoro
		}
		if in.Reschedulable != nil {
			task.Reschedulable = *in.Reschedulable
		}
		if in.Priority != nil {
			task.Priority = *in.Priority
		}
		if in.IsActive != nil {
			task.IsActive = *in.IsActive
		}
		if in.Progress != nil {
			task.Progress = *in.Progress
		}

		if task.TopicID != nil {
			if _, err := tx.Topics().Get(ctx, *task.TopicID); err != nil {
				if IsNotFound(err) {
					return invalid("topic not found")
				}
				return err
			}
		}
		if err := tx.Tasks().Update(ctx, task); err != nil {
			return fmt.Errorf("update task: %w", err)
		}
		loaded, err := tx.Tasks().Get(ctx, task.ID)
		if err != nil {
			return err
		}
		task = loaded
		if err := s.regenerate(ctx, tx, task); err != nil {
			return err
		}
		if err := emitTaskEvent(ctx, tx, "task.updated", task); err != nil {
			return err
		}
		if task.Progress != prevProgress {
			switch task.Progress {
			case models.ProgressCompleted:
				if err := emitTaskEvent(ctx, tx, "task.completed", task); err != nil {
					return err
				}
			case models.ProgressCancelled:
				if err := emitTaskEvent(ctx, tx, "task.cancelled", task); err != nil {
					return err
				}
			}
		}
		updated = task
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *TaskService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.store.InTx(ctx, func(tx repository.Store) error {
		task, err := tx.Tasks().Get(ctx, id)
		if err != nil {
			return err
		}
		occs, err := tx.Occurrences().List(ctx, repository.OccurrenceFilter{TaskID: &id})
		if err != nil {
			return err
		}
		occIDs := make([]uuid.UUID, 0, len(occs))
		for _, o := range occs {
			occIDs = append(occIDs, o.ID)
		}
		if err := s.emitPlanRemovals(ctx, tx, occIDs); err != nil {
			return err
		}
		if err := tx.Tasks().Delete(ctx, id); err != nil {
			return err
		}
		return emitTaskEvent(ctx, tx, "task.cancelled", task)
	})
}

type planRemoval struct {
	date    models.Date
	removed []uuid.UUID
}

func (s *TaskService) emitPlanRemovals(ctx context.Context, tx repository.Store, occIDs []uuid.UUID) error {
	refs, err := tx.Plans().CommittedRefsForOccurrences(ctx, occIDs)
	if err != nil {
		return err
	}
	byPlan := map[uuid.UUID]*planRemoval{}
	order := []uuid.UUID{}
	for _, ref := range refs {
		entry, ok := byPlan[ref.PlanID]
		if !ok {
			entry = &planRemoval{date: ref.PlanDate}
			byPlan[ref.PlanID] = entry
			order = append(order, ref.PlanID)
		}
		entry.removed = append(entry.removed, ref.OccurrenceID)
	}
	for _, planID := range order {
		entry := byPlan[planID]
		if err := emitPlanUpdated(ctx, tx, planID, entry.date, nil, entry.removed, false); err != nil {
			return err
		}
	}
	return nil
}

func (s *TaskService) regenerate(ctx context.Context, tx repository.Store, task *models.Task) error {
	today := s.today()
	windowEnd := today.AddDays(s.windowDays - 1)

	from := today
	if task.StartDate.Before(today) {
		switch task.RecurrenceKind {
		case models.RecurrenceSpacedRepetition:
			from = task.StartDate
			if floor := today.AddDays(-s.windowDays); from.Before(floor) {
				from = floor
			}
		case models.RecurrenceOnce:
			from = task.StartDate
		}
	}

	var sched []ScheduledDate
	if task.IsActive && task.Progress.Open() {
		sched = ScheduleDates(*task, from, windowEnd)
	}

	if task.RecurrenceKind == models.RecurrenceSpacedRepetition && len(sched) > 0 {
		completedSteps, err := tx.Occurrences().CompletedSteps(ctx, task.ID)
		if err != nil {
			return err
		}
		done := map[int]bool{}
		for _, step := range completedSteps {
			done[step] = true
		}
		filtered := sched[:0]
		for _, sd := range sched {
			if sd.SeriesStep != nil && done[*sd.SeriesStep] {
				continue
			}
			filtered = append(filtered, sd)
		}
		sched = filtered
	}

	schedMap := map[string]ScheduledDate{}
	for _, sd := range sched {
		schedMap[sd.Date.String()] = sd
	}

	pendingStatus := models.OccurrencePending
	pendings, err := tx.Occurrences().List(ctx, repository.OccurrenceFilter{
		TaskID: &task.ID,
		From:   &from,
		Status: &pendingStatus,
	})
	if err != nil {
		return err
	}

	var removedIDs []uuid.UUID
	for i := range pendings {
		p := pendings[i]
		p.Task = nil
		sd, ok := schedMap[p.Date.String()]
		if ok {
			delete(schedMap, p.Date.String())
			if !intPtrEqual(p.SeriesStep, sd.SeriesStep) {
				p.SeriesStep = sd.SeriesStep
				if err := tx.Occurrences().Update(ctx, &p); err != nil {
					return err
				}
			}
			continue
		}
		removedIDs = append(removedIDs, p.ID)
	}

	if len(removedIDs) > 0 {
		if err := s.emitPlanRemovals(ctx, tx, removedIDs); err != nil {
			return err
		}
		for _, id := range removedIDs {
			if err := tx.Occurrences().Delete(ctx, id); err != nil {
				return err
			}
		}
	}

	for _, sd := range sched {
		if _, remains := schedMap[sd.Date.String()]; !remains {
			continue
		}
		occ := &models.TaskOccurrence{
			TaskID:     task.ID,
			Date:       sd.Date,
			Status:     models.OccurrencePending,
			SeriesStep: sd.SeriesStep,
		}
		if _, err := tx.Occurrences().InsertIgnoreConflict(ctx, occ); err != nil {
			return err
		}
	}
	return s.supersedeStaleMissed(ctx, tx, task)
}

func (s *TaskService) supersedeStaleMissed(ctx context.Context, tx repository.Store, task *models.Task) error {
	if task.RecurrenceKind != models.RecurrenceSpacedRepetition {
		return nil
	}
	occs, err := tx.Occurrences().List(ctx, repository.OccurrenceFilter{TaskID: &task.ID})
	if err != nil {
		return err
	}
	live := map[int]models.TaskOccurrence{}
	for _, o := range occs {
		if o.SeriesStep == nil || (o.Status != models.OccurrenceCompleted && o.Status != models.OccurrencePending) {
			continue
		}
		cur, ok := live[*o.SeriesStep]
		if !ok || (o.Status == models.OccurrenceCompleted && cur.Status != models.OccurrenceCompleted) {
			live[*o.SeriesStep] = o
		}
	}
	for i := range occs {
		o := occs[i]
		if o.Status != models.OccurrenceMissed || o.SeriesStep == nil {
			continue
		}
		actual, ok := live[*o.SeriesStep]
		if !ok || actual.Date == o.Date {
			continue
		}
		oldDate, newDate := o.Date, actual.Date
		o.Task = nil
		o.Status = models.OccurrenceRescheduled
		o.RescheduledTo = &newDate
		if err := tx.Occurrences().Update(ctx, &o); err != nil {
			return err
		}
		if err := emitOccurrenceRescheduled(ctx, tx, &o, oldDate, newDate, newDate.DaysSince(oldDate)); err != nil {
			return err
		}
	}
	return nil
}

func (s *TaskService) RescheduleMissed(ctx context.Context, id uuid.UUID) (*RescheduleResult, error) {
	result := &RescheduleResult{TaskID: id}
	err := s.store.InTx(ctx, func(tx repository.Store) error {
		task, err := tx.Tasks().Get(ctx, id)
		if err != nil {
			return err
		}
		if !task.Reschedulable {
			return nil
		}
		today := s.today()

		occs, err := tx.Occurrences().List(ctx, repository.OccurrenceFilter{TaskID: &id})
		if err != nil {
			return err
		}
		var candidates []models.TaskOccurrence
		for _, o := range occs {
			if o.Status == models.OccurrenceMissed ||
				(o.Status == models.OccurrencePending && o.Date.Before(today)) {
				candidates = append(candidates, o)
			}
		}
		if len(candidates) == 0 {
			return nil
		}
		first := candidates[0].Date
		for _, o := range candidates {
			if o.Date.Before(first) {
				first = o.Date
			}
		}
		shift := today.DaysSince(first)
		if shift <= 0 {
			return nil
		}

		for i := range candidates {
			o := candidates[i]
			o.Task = nil
			oldDate := o.Date
			newDate := oldDate.AddDays(shift)
			o.Status = models.OccurrenceRescheduled
			o.RescheduledTo = &newDate
			if err := tx.Occurrences().Update(ctx, &o); err != nil {
				return err
			}
			if err := emitOccurrenceRescheduled(ctx, tx, &o, oldDate, newDate, shift); err != nil {
				return err
			}
		}

		task.StartDate = task.StartDate.AddDays(shift)
		if err := tx.Tasks().Update(ctx, task); err != nil {
			return err
		}
		if err := s.regenerate(ctx, tx, task); err != nil {
			return err
		}
		if err := emitTaskEvent(ctx, tx, "task.updated", task); err != nil {
			return err
		}
		result.ShiftDays = shift
		result.Rescheduled = len(candidates)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *TaskService) GenerateAll(ctx context.Context) error {
	tasks, err := s.store.Tasks().ListActive(ctx)
	if err != nil {
		return err
	}
	for i := range tasks {
		task := tasks[i]
		err := s.store.InTx(ctx, func(tx repository.Store) error {
			return s.regenerate(ctx, tx, &task)
		})
		if err != nil {
			return fmt.Errorf("regenerate task %s: %w", task.ID, err)
		}
	}
	return nil
}

func (s *TaskService) MarkMissed(ctx context.Context) (int, error) {
	today := s.today()
	yesterday := today.AddDays(-1)
	marked := 0
	err := s.store.InTx(ctx, func(tx repository.Store) error {
		pendingStatus := models.OccurrencePending
		occs, err := tx.Occurrences().List(ctx, repository.OccurrenceFilter{
			To:     &yesterday,
			Status: &pendingStatus,
		})
		if err != nil {
			return err
		}
		for i := range occs {
			o := occs[i]
			task := o.Task
			if task != nil {
				if !task.IsActive || !task.Progress.Open() {
					continue
				}
				if task.Due != nil && !task.Due.Before(today) {
					continue
				}
			}
			o.Task = nil
			o.Status = models.OccurrenceMissed
			if err := tx.Occurrences().Update(ctx, &o); err != nil {
				return err
			}
			if task == nil {
				task = &models.Task{ID: o.TaskID}
			}
			if err := emitOccurrenceMissed(ctx, tx, &o, task); err != nil {
				return err
			}
			marked++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return marked, nil
}

func intPtrEqual(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
