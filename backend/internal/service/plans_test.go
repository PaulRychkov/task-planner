package service

import (
	"context"
	"testing"

	"github.com/PaulRychkov/task-planner/backend/internal/models"
)

func countEvents(f *fixture, eventType string) int {
	n := 0
	for _, e := range f.store.EventTypes() {
		if e == eventType {
			n++
		}
	}
	return n
}

func TestPlanDraftAndCommit(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	today := mustDate(t, "2026-07-06")

	taskA := f.mustCreate(t, dailyInput("A"))
	taskB := f.mustCreate(t, dailyInput("B"))
	occA := f.occurrenceByDate(t, taskA.ID, "2026-07-06")
	occB := f.occurrenceByDate(t, taskB.ID, "2026-07-06")

	start := 540
	plan, err := f.plans.PutItems(ctx, today, []PlanItemInput{
		{OccurrenceID: occA.ID, PlannedStartMinutes: &start},
		{OccurrenceID: occB.ID},
	})
	if err != nil {
		t.Fatalf("put items: %v", err)
	}
	if plan.Committed() {
		t.Fatal("plan must be a draft before commit")
	}
	if len(plan.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(plan.Items))
	}
	if plan.Items[0].OccurrenceID != occA.ID || plan.Items[0].Position != 0 {
		t.Error("first item wrong occurrence or position")
	}
	if plan.Items[0].PlannedStartMinutes == nil || *plan.Items[0].PlannedStartMinutes != 540 {
		t.Error("anchor time lost")
	}
	if countEvents(f, "plan.updated") != 0 {
		t.Error("draft edits must not emit plan.updated")
	}

	committed, err := f.plans.Commit(ctx, today, models.ActorApp)
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if !committed.Committed() || *committed.CommittedBy != models.ActorApp {
		t.Error("commit fields not set")
	}
	if countEvents(f, "plan.committed") != 1 {
		t.Error("plan.committed not emitted")
	}

	if _, err := f.plans.Commit(ctx, today, models.ActorApp); !IsConflict(err) {
		t.Errorf("second commit err = %v, want conflict", err)
	}
}

func TestPlanUpdatedOnCommittedChanges(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	today := mustDate(t, "2026-07-06")

	taskA := f.mustCreate(t, dailyInput("A"))
	taskB := f.mustCreate(t, dailyInput("B"))
	occA := f.occurrenceByDate(t, taskA.ID, "2026-07-06")
	occB := f.occurrenceByDate(t, taskB.ID, "2026-07-06")

	if _, err := f.plans.PutItems(ctx, today, []PlanItemInput{{OccurrenceID: occA.ID}}); err != nil {
		t.Fatalf("put items: %v", err)
	}
	if _, err := f.plans.Commit(ctx, today, models.ActorApp); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if _, err := f.plans.PutItems(ctx, today, []PlanItemInput{{OccurrenceID: occA.ID}, {OccurrenceID: occB.ID}}); err != nil {
		t.Fatalf("add item: %v", err)
	}
	if countEvents(f, "plan.updated") != 1 {
		t.Fatalf("plan.updated = %d after add, want 1", countEvents(f, "plan.updated"))
	}

	if _, err := f.plans.PutItems(ctx, today, []PlanItemInput{{OccurrenceID: occB.ID}, {OccurrenceID: occA.ID}}); err != nil {
		t.Fatalf("reorder: %v", err)
	}
	if countEvents(f, "plan.updated") != 2 {
		t.Fatalf("plan.updated = %d after reorder, want 2", countEvents(f, "plan.updated"))
	}

	if _, err := f.plans.PutItems(ctx, today, []PlanItemInput{{OccurrenceID: occB.ID}, {OccurrenceID: occA.ID}}); err != nil {
		t.Fatalf("noop put: %v", err)
	}
	if countEvents(f, "plan.updated") != 2 {
		t.Fatal("noop change must not emit plan.updated")
	}

	if _, err := f.plans.PutItems(ctx, today, []PlanItemInput{{OccurrenceID: occB.ID}}); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if countEvents(f, "plan.updated") != 3 {
		t.Fatalf("plan.updated = %d after remove, want 3", countEvents(f, "plan.updated"))
	}
}

func TestPlanItemValidation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	today := mustDate(t, "2026-07-06")
	task := f.mustCreate(t, dailyInput("A"))
	occToday := f.occurrenceByDate(t, task.ID, "2026-07-06")
	occTomorrow := f.occurrenceByDate(t, task.ID, "2026-07-07")

	if _, err := f.plans.PutItems(ctx, today, []PlanItemInput{{OccurrenceID: occTomorrow.ID}}); !IsValidation(err) {
		t.Errorf("wrong date: err = %v, want validation", err)
	}
	if _, err := f.plans.PutItems(ctx, today, []PlanItemInput{
		{OccurrenceID: occToday.ID}, {OccurrenceID: occToday.ID},
	}); !IsValidation(err) {
		t.Errorf("duplicate: err = %v, want validation", err)
	}
	bad := 2000
	if _, err := f.plans.PutItems(ctx, today, []PlanItemInput{
		{OccurrenceID: occToday.ID, PlannedStartMinutes: &bad},
	}); !IsValidation(err) {
		t.Errorf("bad minutes: err = %v, want validation", err)
	}
	if _, err := f.plans.Commit(ctx, mustDate(t, "2026-07-07"), models.ActorApp); !IsNotFound(err) {
		t.Errorf("commit missing plan: err = %v, want not found", err)
	}
	if _, err := f.plans.Commit(ctx, today, models.ActorKind("robot")); !IsValidation(err) {
		t.Errorf("bad actor: err = %v, want validation", err)
	}
}
