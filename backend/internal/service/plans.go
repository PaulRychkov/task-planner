package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/PaulRychkov/task-planner/backend/internal/models"
	"github.com/PaulRychkov/task-planner/backend/internal/repository"
)

type PlanItemInput struct {
	OccurrenceID        uuid.UUID `json:"occurrence_id"`
	PlannedStartMinutes *int      `json:"planned_start_minutes"`
}

type PlanService struct {
	store repository.Store
	clock Clock
	loc   *time.Location
}

func NewPlanService(store repository.Store, clock Clock, loc *time.Location) *PlanService {
	return &PlanService{store: store, clock: clock, loc: loc}
}

func (s *PlanService) Get(ctx context.Context, date models.Date) (*models.DayPlan, error) {
	return s.store.Plans().GetByDate(ctx, date)
}

func (s *PlanService) PutItems(ctx context.Context, date models.Date, items []PlanItemInput) (*models.DayPlan, error) {
	var result *models.DayPlan
	err := s.store.InTx(ctx, func(tx repository.Store) error {
		seen := map[uuid.UUID]bool{}
		for _, item := range items {
			if seen[item.OccurrenceID] {
				return invalid(fmt.Sprintf("duplicate occurrence %s in plan items", item.OccurrenceID))
			}
			seen[item.OccurrenceID] = true
			if item.PlannedStartMinutes != nil &&
				(*item.PlannedStartMinutes < 0 || *item.PlannedStartMinutes > 1439) {
				return invalid("planned_start_minutes out of range 0..1439")
			}
			occ, err := tx.Occurrences().Get(ctx, item.OccurrenceID)
			if err != nil {
				if IsNotFound(err) {
					return invalid(fmt.Sprintf("occurrence %s not found", item.OccurrenceID))
				}
				return err
			}
			if occ.Date != date {
				return invalid(fmt.Sprintf("occurrence %s belongs to %s, not %s", item.OccurrenceID, occ.Date, date))
			}
		}

		plan, err := tx.Plans().GetByDate(ctx, date)
		if IsNotFound(err) {
			plan = &models.DayPlan{Date: date}
			if err := tx.Plans().Create(ctx, plan); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}

		oldOrder := make([]uuid.UUID, 0, len(plan.Items))
		oldSet := map[uuid.UUID]bool{}
		for _, item := range plan.Items {
			oldOrder = append(oldOrder, item.OccurrenceID)
			oldSet[item.OccurrenceID] = true
		}

		newItems := make([]models.DayPlanItem, 0, len(items))
		newSet := map[uuid.UUID]bool{}
		for i, item := range items {
			newItems = append(newItems, models.DayPlanItem{
				OccurrenceID:        item.OccurrenceID,
				PlannedStartMinutes: item.PlannedStartMinutes,
				Position:            i,
			})
			newSet[item.OccurrenceID] = true
		}

		var added, removed []uuid.UUID
		for _, id := range oldOrder {
			if !newSet[id] {
				removed = append(removed, id)
			}
		}
		for _, item := range newItems {
			if !oldSet[item.OccurrenceID] {
				added = append(added, item.OccurrenceID)
			}
		}
		reordered := retainedOrderChanged(oldOrder, newItems, newSet, oldSet)

		if err := tx.Plans().ReplaceItems(ctx, plan.ID, newItems); err != nil {
			return err
		}
		if err := tx.Plans().Update(ctx, plan); err != nil {
			return err
		}
		if plan.Committed() && (len(added) > 0 || len(removed) > 0 || reordered) {
			if err := emitPlanUpdated(ctx, tx, plan.ID, plan.Date, added, removed, reordered); err != nil {
				return err
			}
		}
		loaded, err := tx.Plans().GetByDate(ctx, date)
		if err != nil {
			return err
		}
		result = loaded
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func retainedOrderChanged(oldOrder []uuid.UUID, newItems []models.DayPlanItem, newSet, oldSet map[uuid.UUID]bool) bool {
	var oldRetained, newRetained []uuid.UUID
	for _, id := range oldOrder {
		if newSet[id] {
			oldRetained = append(oldRetained, id)
		}
	}
	for _, item := range newItems {
		if oldSet[item.OccurrenceID] {
			newRetained = append(newRetained, item.OccurrenceID)
		}
	}
	if len(oldRetained) != len(newRetained) {
		return true
	}
	for i := range oldRetained {
		if oldRetained[i] != newRetained[i] {
			return true
		}
	}
	return false
}

func (s *PlanService) Commit(ctx context.Context, date models.Date, by models.ActorKind) (*models.DayPlan, error) {
	if !by.Valid() {
		return nil, invalid(fmt.Sprintf("unknown committed_by %q", by))
	}
	var result *models.DayPlan
	err := s.store.InTx(ctx, func(tx repository.Store) error {
		plan, err := tx.Plans().GetByDate(ctx, date)
		if err != nil {
			return err
		}
		if plan.Committed() {
			return conflict("plan already committed")
		}
		now := s.clock.Now()
		plan.CommittedAt = &now
		plan.CommittedBy = &by
		if err := tx.Plans().Update(ctx, plan); err != nil {
			return err
		}
		if err := emitPlanCommitted(ctx, tx, plan); err != nil {
			return err
		}
		result = plan
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
