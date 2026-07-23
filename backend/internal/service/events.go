package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/PaulRychkov/task-planner/backend/internal/models"
	"github.com/PaulRychkov/task-planner/backend/internal/repository"
)

const (
	AggregateTask       = "task"
	AggregateOccurrence = "occurrence"
	AggregatePlan       = "plan"
)

func emitEvent(ctx context.Context, st repository.Store, eventType, aggregateType string, aggregateID uuid.UUID, payload map[string]any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload for %s: %w", eventType, err)
	}
	e := &models.OutboxEvent{
		EventType:     eventType,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		Payload:       models.JSON(b),
	}
	if err := st.Outbox().Add(ctx, e); err != nil {
		return fmt.Errorf("outbox add %s: %w", eventType, err)
	}
	return nil
}

func taskSnapshot(t *models.Task) map[string]any {
	return map[string]any{
		"task_id":                    t.ID,
		"title":                      t.Title,
		"description":                t.Description,
		"topic_id":                   t.TopicID,
		"topic":                      nullableTopicName(t),
		"source":                     t.Source,
		"external_id":                t.ExternalID,
		"recurrence_kind":            t.RecurrenceKind,
		"recurrence_params":          t.RecurrenceParams,
		"start_date":                 t.StartDate,
		"due":                        t.Due,
		"start_time_minutes":         t.StartTimeMinutes,
		"estimated_duration_minutes": t.EstimatedDurationMinutes,
		"effort_minutes":             t.EffortMinutes,
		"all_day":                    t.AllDay,
		"requires_pomodoro":          t.RequiresPomodoro,
		"priority":                   t.Priority,
		"progress":                   t.Progress,
		"is_active":                  t.IsActive,
	}
}

func nullableTopicName(t *models.Task) any {
	if t.Topic == nil {
		return nil
	}
	return t.Topic.Name
}

func emitTaskEvent(ctx context.Context, st repository.Store, eventType string, t *models.Task) error {
	return emitEvent(ctx, st, eventType, AggregateTask, t.ID, taskSnapshot(t))
}

func emitOccurrenceCompleted(ctx context.Context, st repository.Store, o *models.TaskOccurrence, t *models.Task) error {
	return emitEvent(ctx, st, "occurrence.completed", AggregateOccurrence, o.ID, map[string]any{
		"occurrence_id": o.ID,
		"task_id":       o.TaskID,
		"title":         t.Title,
		"topic":         nullableTopicName(t),
		"date":          o.Date,
		"series_step":   o.SeriesStep,
		"completed_at":  o.CompletedAt,
	})
}

func emitOccurrenceSkipped(ctx context.Context, st repository.Store, o *models.TaskOccurrence, t *models.Task) error {
	return emitEvent(ctx, st, "occurrence.skipped", AggregateOccurrence, o.ID, map[string]any{
		"occurrence_id": o.ID,
		"task_id":       o.TaskID,
		"title":         t.Title,
		"topic":         nullableTopicName(t),
		"date":          o.Date,
	})
}

func emitOccurrenceMissed(ctx context.Context, st repository.Store, o *models.TaskOccurrence, t *models.Task) error {
	return emitEvent(ctx, st, "occurrence.missed", AggregateOccurrence, o.ID, map[string]any{
		"occurrence_id": o.ID,
		"task_id":       o.TaskID,
		"title":         t.Title,
		"topic":         nullableTopicName(t),
		"date":          o.Date,
	})
}

func emitOccurrenceRescheduled(ctx context.Context, st repository.Store, o *models.TaskOccurrence, oldDate, newDate models.Date, shiftDays int) error {
	return emitEvent(ctx, st, "occurrence.rescheduled", AggregateOccurrence, o.ID, map[string]any{
		"occurrence_id": o.ID,
		"task_id":       o.TaskID,
		"old_date":      oldDate,
		"new_date":      newDate,
		"shift_days":    shiftDays,
	})
}

func emitPlanCommitted(ctx context.Context, st repository.Store, p *models.DayPlan) error {
	items := make([]map[string]any, 0, len(p.Items))
	for _, item := range p.Items {
		entry := map[string]any{
			"occurrence_id":         item.OccurrenceID,
			"planned_start_minutes": item.PlannedStartMinutes,
			"position":              item.Position,
		}
		if item.Occurrence != nil {
			entry["task_id"] = item.Occurrence.TaskID
			entry["date"] = item.Occurrence.Date
			if item.Occurrence.Task != nil {
				entry["title"] = item.Occurrence.Task.Title
			}
		}
		items = append(items, entry)
	}
	return emitEvent(ctx, st, "plan.committed", AggregatePlan, p.ID, map[string]any{
		"plan_id":      p.ID,
		"date":         p.Date,
		"committed_by": p.CommittedBy,
		"items":        items,
	})
}

func emitPlanUpdated(ctx context.Context, st repository.Store, planID uuid.UUID, date models.Date, added, removed []uuid.UUID, reordered bool) error {
	if added == nil {
		added = []uuid.UUID{}
	}
	if removed == nil {
		removed = []uuid.UUID{}
	}
	return emitEvent(ctx, st, "plan.updated", AggregatePlan, planID, map[string]any{
		"plan_id":   planID,
		"date":      date,
		"added":     added,
		"removed":   removed,
		"reordered": reordered,
	})
}
