package service

import (
	"context"
	"testing"

	"github.com/PaulRychkov/task-planner/backend/internal/models"
)

func newTopicFixture(t *testing.T) (*fixture, *TopicService) {
	t.Helper()
	f := newFixture(t)
	return f, NewTopicService(f.store)
}

func TestCreateTaskRejectsEffortWithFixedTime(t *testing.T) {
	f := newFixture(t)
	start := 600
	effort := 50
	_, err := f.tasks.Create(context.Background(), TaskInput{
		Title:            "x",
		RecurrenceKind:   models.RecurrenceOnce,
		StartTimeMinutes: &start,
		EffortMinutes:    &effort,
	})
	if !IsValidation(err) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestCreateTaskRejectsEffortWithDuration(t *testing.T) {
	f := newFixture(t)
	dur := 60
	effort := 50
	_, err := f.tasks.Create(context.Background(), TaskInput{
		Title:                    "x",
		RecurrenceKind:           models.RecurrenceOnce,
		EstimatedDurationMinutes: &dur,
		EffortMinutes:            &effort,
	})
	if !IsValidation(err) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestCreateTaskRejectsDueBeforeStart(t *testing.T) {
	f := newFixture(t)
	start := mustDate(t, "2026-07-10")
	due := mustDate(t, "2026-07-08")
	_, err := f.tasks.Create(context.Background(), TaskInput{
		Title:          "x",
		RecurrenceKind: models.RecurrenceOnce,
		StartDate:      &start,
		Due:            &due,
	})
	if !IsValidation(err) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestCreateTaskDefaultsRequiresPomodoro(t *testing.T) {
	f := newFixture(t)
	task := f.mustCreate(t, TaskInput{Title: "x", RecurrenceKind: models.RecurrenceOnce})
	if !task.RequiresPomodoro {
		t.Fatalf("requires_pomodoro must default to true")
	}
	off := false
	updated, err := f.tasks.Update(context.Background(), task.ID, TaskInput{
		Title:            task.Title,
		RecurrenceKind:   task.RecurrenceKind,
		StartDate:        &task.StartDate,
		RequiresPomodoro: &off,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.RequiresPomodoro {
		t.Fatalf("requires_pomodoro=false not applied on update")
	}
}

func TestTopicTreeRejectsCycles(t *testing.T) {
	_, topics := newTopicFixture(t)
	ctx := context.Background()
	a, err := topics.Create(ctx, TopicInput{Name: "A"})
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	b, err := topics.Create(ctx, TopicInput{Name: "B", ParentID: &a.ID})
	if err != nil {
		t.Fatalf("create B: %v", err)
	}
	c, err := topics.Create(ctx, TopicInput{Name: "C", ParentID: &b.ID})
	if err != nil {
		t.Fatalf("create C: %v", err)
	}
	if _, err := topics.Update(ctx, a.ID, TopicInput{Name: "A", ParentID: &a.ID}); !IsValidation(err) {
		t.Fatalf("self-parent must be rejected, got %v", err)
	}
	if _, err := topics.Update(ctx, a.ID, TopicInput{Name: "A", ParentID: &c.ID}); !IsValidation(err) {
		t.Fatalf("deep cycle A->C(->B->A) must be rejected, got %v", err)
	}
	if _, err := topics.Update(ctx, c.ID, TopicInput{Name: "C", ParentID: &a.ID}); err != nil {
		t.Fatalf("legal reparent must pass: %v", err)
	}
}

func TestTopicUpdateRejectsMissingParent(t *testing.T) {
	_, topics := newTopicFixture(t)
	ctx := context.Background()
	a, err := topics.Create(ctx, TopicInput{Name: "A"})
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	missing := a.ID
	missing[0] ^= 0xff
	if _, err := topics.Update(ctx, a.ID, TopicInput{Name: "A", ParentID: &missing}); !IsValidation(err) {
		t.Fatalf("missing parent must be validation error, got %v", err)
	}
}

func TestFindOrCreateUnderRejectsCyclicReparent(t *testing.T) {
	_, topics := newTopicFixture(t)
	ctx := context.Background()
	a, err := topics.Create(ctx, TopicInput{Name: "A"})
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	if _, err := topics.Create(ctx, TopicInput{Name: "B", ParentID: &a.ID}); err != nil {
		t.Fatalf("create B: %v", err)
	}
	b, err := topics.FindOrCreateUnder(ctx, "B", nil)
	if err != nil {
		t.Fatalf("find B: %v", err)
	}
	if _, err := topics.FindOrCreateUnder(ctx, "A", &b.ID); !IsValidation(err) {
		t.Fatalf("cyclic reparent via FindOrCreateUnder must be rejected, got %v", err)
	}
}

func TestMarkMissedSkipsOpenWindowAndClosedTasks(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	start := mustDate(t, "2026-07-05")
	due := mustDate(t, "2026-07-20")
	windowed := f.mustCreate(t, TaskInput{
		Title:          "window",
		RecurrenceKind: models.RecurrenceOnce,
		StartDate:      &start,
		Due:            &due,
	})
	inactive := f.mustCreate(t, TaskInput{
		Title:          "inactive",
		RecurrenceKind: models.RecurrenceOnce,
		StartDate:      &start,
	})
	off := false
	if _, err := f.tasks.Update(ctx, inactive.ID, TaskInput{
		Title:          inactive.Title,
		RecurrenceKind: inactive.RecurrenceKind,
		StartDate:      &start,
		IsActive:       &off,
	}); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if _, err := f.tasks.MarkMissed(ctx); err != nil {
		t.Fatalf("mark missed: %v", err)
	}
	if o := f.occurrenceByDate(t, windowed.ID, "2026-07-20"); o == nil || o.Status != models.OccurrencePending {
		t.Fatalf("deadline occurrence must sit on due date and stay pending, got %+v", o)
	}
	if o := f.occurrenceByDate(t, windowed.ID, "2026-07-05"); o != nil {
		t.Fatalf("windowed once task must not have an occurrence on start_date, got %+v", o)
	}
	if o := f.occurrenceByDate(t, inactive.ID, "2026-07-05"); o != nil && o.Status == models.OccurrenceMissed {
		t.Fatalf("occurrence of inactive task must not become missed")
	}
}

func TestOnceWithDueSchedulesOnDeadline(t *testing.T) {
	f := newFixture(t)
	start := mustDate(t, "2026-07-06")
	due := mustDate(t, "2026-09-20")
	task := f.mustCreate(t, TaskInput{
		Title:          "deadline",
		RecurrenceKind: models.RecurrenceOnce,
		StartDate:      &start,
		Due:            &due,
	})
	occs := f.occurrences(t, task.ID)
	if len(occs) != 1 {
		t.Fatalf("expected exactly one occurrence, got %d", len(occs))
	}
	if occs[0].Date.String() != "2026-09-20" {
		t.Fatalf("occurrence must sit on due date beyond generation window, got %s", occs[0].Date)
	}
}
