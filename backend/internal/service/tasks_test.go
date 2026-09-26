package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/PaulRychkov/task-planner/backend/internal/models"
	"github.com/PaulRychkov/task-planner/backend/internal/repository"
	"github.com/PaulRychkov/task-planner/backend/internal/repository/memory"
)

type testClock struct {
	t time.Time
}

func (c *testClock) Now() time.Time { return c.t }

func (c *testClock) Advance(d time.Duration) { c.t = c.t.Add(d) }

type fixture struct {
	store *memory.Store
	clock *testClock
	tasks *TaskService
	occs  *OccurrenceService
	plans *PlanService
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	store := memory.NewStore()
	clock := &testClock{t: time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)}
	return &fixture{
		store: store,
		clock: clock,
		tasks: NewTaskService(store, clock, time.UTC, 60),
		occs:  NewOccurrenceService(store, clock, time.UTC),
		plans: NewPlanService(store, clock, time.UTC),
	}
}

func (f *fixture) mustCreate(t *testing.T, in TaskInput) *models.Task {
	t.Helper()
	task, err := f.tasks.Create(context.Background(), in)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	return task
}

func (f *fixture) occurrences(t *testing.T, taskID uuid.UUID) []models.TaskOccurrence {
	t.Helper()
	occs, err := f.store.Occurrences().List(context.Background(), repository.OccurrenceFilter{TaskID: &taskID})
	if err != nil {
		t.Fatalf("list occurrences: %v", err)
	}
	return occs
}

func (f *fixture) occurrenceByDate(t *testing.T, taskID uuid.UUID, date string) *models.TaskOccurrence {
	t.Helper()
	for _, o := range f.occurrences(t, taskID) {
		if o.Date.String() == date {
			c := o
			return &c
		}
	}
	return nil
}

func (f *fixture) hasEvent(eventType string) bool {
	for _, e := range f.store.EventTypes() {
		if e == eventType {
			return true
		}
	}
	return false
}

func dailyInput(title string) TaskInput {
	return TaskInput{Title: title, RecurrenceKind: models.RecurrenceDaily}
}

func srInput(title string, intervals []int) TaskInput {
	return TaskInput{
		Title:            title,
		RecurrenceKind:   models.RecurrenceSpacedRepetition,
		RecurrenceParams: &models.RecurrenceParams{Intervals: intervals},
	}
}

func TestCreateGeneratesWindow(t *testing.T) {
	f := newFixture(t)
	task := f.mustCreate(t, dailyInput("зарядка"))

	occs := f.occurrences(t, task.ID)
	if len(occs) != 60 {
		t.Fatalf("got %d occurrences, want 60", len(occs))
	}
	if occs[0].Date.String() != "2026-07-06" {
		t.Errorf("first occurrence %s, want 2026-07-06", occs[0].Date)
	}
	if occs[59].Date.String() != "2026-09-03" {
		t.Errorf("last occurrence %s, want 2026-09-03", occs[59].Date)
	}
	for _, o := range occs {
		if o.Status != models.OccurrencePending {
			t.Fatalf("occurrence %s status %s, want pending", o.Date, o.Status)
		}
	}
	if !f.hasEvent("task.created") {
		t.Error("task.created not emitted")
	}
}

func TestGenerateAllIdempotent(t *testing.T) {
	f := newFixture(t)
	task := f.mustCreate(t, dailyInput("чтение"))
	before := f.occurrences(t, task.ID)

	if err := f.tasks.GenerateAll(context.Background()); err != nil {
		t.Fatalf("generate all: %v", err)
	}
	after := f.occurrences(t, task.ID)
	if len(after) != len(before) {
		t.Fatalf("got %d occurrences after regeneration, want %d", len(after), len(before))
	}
	for i := range after {
		if after[i].ID != before[i].ID {
			t.Fatalf("occurrence %s changed id", after[i].Date)
		}
	}
}

func TestRuleChangeUpdatesPendingInPlace(t *testing.T) {
	f := newFixture(t)
	task := f.mustCreate(t, TaskInput{
		Title:            "спортзал",
		RecurrenceKind:   models.RecurrenceDaysOfWeek,
		RecurrenceParams: &models.RecurrenceParams{Days: []int{1}},
	})
	monday := f.occurrenceByDate(t, task.ID, "2026-07-06")
	if monday == nil {
		t.Fatal("monday occurrence missing")
	}

	in := dailyInput("спортзал")
	in.RecurrenceKind = models.RecurrenceDaysOfWeek
	in.RecurrenceParams = &models.RecurrenceParams{Days: []int{1, 3}}
	if _, err := f.tasks.Update(context.Background(), task.ID, in); err != nil {
		t.Fatalf("update task: %v", err)
	}

	mondayAfter := f.occurrenceByDate(t, task.ID, "2026-07-06")
	if mondayAfter == nil || mondayAfter.ID != monday.ID {
		t.Fatal("monday occurrence was recreated instead of kept in place")
	}
	if f.occurrenceByDate(t, task.ID, "2026-07-08") == nil {
		t.Fatal("wednesday occurrence not added")
	}
}

func TestRuleChangeRemovesFromCommittedPlanWithEvent(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	task := f.mustCreate(t, dailyInput("код-ревью"))
	today := mustDate(t, "2026-07-06")
	occ := f.occurrenceByDate(t, task.ID, "2026-07-06")

	if _, err := f.plans.PutItems(ctx, today, []PlanItemInput{{OccurrenceID: occ.ID}}); err != nil {
		t.Fatalf("put items: %v", err)
	}
	if _, err := f.plans.Commit(ctx, today, models.ActorApp); err != nil {
		t.Fatalf("commit: %v", err)
	}

	in := dailyInput("код-ревью")
	in.RecurrenceKind = models.RecurrenceWeekends
	if _, err := f.tasks.Update(ctx, task.ID, in); err != nil {
		t.Fatalf("update: %v", err)
	}

	if f.occurrenceByDate(t, task.ID, "2026-07-06") != nil {
		t.Error("monday pending occurrence should be deleted")
	}
	if !f.hasEvent("plan.updated") {
		t.Error("plan.updated not emitted for committed plan")
	}
	if !f.hasEvent("task.updated") {
		t.Error("task.updated not emitted")
	}
}

func TestRescheduleMissedPreservesSeriesStep(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	task := f.mustCreate(t, srInput("повторение Go", []int{0, 1, 3, 7}))

	step0 := f.occurrenceByDate(t, task.ID, "2026-07-06")
	if step0 == nil || step0.SeriesStep == nil || *step0.SeriesStep != 0 {
		t.Fatal("step 0 occurrence missing")
	}
	if _, err := f.occs.Complete(ctx, step0.ID); err != nil {
		t.Fatalf("complete step 0: %v", err)
	}

	f.clock.Advance(72 * time.Hour)

	if _, err := f.tasks.MarkMissed(ctx); err != nil {
		t.Fatalf("mark missed: %v", err)
	}
	missed := f.occurrenceByDate(t, task.ID, "2026-07-07")
	if missed.Status != models.OccurrenceMissed {
		t.Fatalf("step 1 status %s, want missed", missed.Status)
	}

	keptID := f.occurrenceByDate(t, task.ID, "2026-07-09").ID

	res, err := f.tasks.RescheduleMissed(ctx, task.ID)
	if err != nil {
		t.Fatalf("reschedule missed: %v", err)
	}
	if res.ShiftDays != 2 {
		t.Errorf("shift days = %d, want 2", res.ShiftDays)
	}

	updated, err := f.tasks.Get(ctx, task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if updated.StartDate.String() != "2026-07-08" {
		t.Errorf("start date %s, want 2026-07-08", updated.StartDate)
	}

	completed := f.occurrenceByDate(t, task.ID, "2026-07-06")
	if completed.Status != models.OccurrenceCompleted || *completed.SeriesStep != 0 {
		t.Error("completed history row was modified")
	}

	rescheduled := f.occurrenceByDate(t, task.ID, "2026-07-07")
	if rescheduled.Status != models.OccurrenceRescheduled {
		t.Errorf("old missed row status %s, want rescheduled", rescheduled.Status)
	}
	if rescheduled.RescheduledTo == nil || rescheduled.RescheduledTo.String() != "2026-07-09" {
		t.Error("rescheduled_to not set to shifted date")
	}

	wantPending := map[string]int{"2026-07-09": 1, "2026-07-11": 2, "2026-07-15": 3}
	pendingCount := 0
	for _, o := range f.occurrences(t, task.ID) {
		if o.Status != models.OccurrencePending {
			continue
		}
		pendingCount++
		step, ok := wantPending[o.Date.String()]
		if !ok {
			t.Errorf("unexpected pending date %s", o.Date)
			continue
		}
		if o.SeriesStep == nil || *o.SeriesStep != step {
			t.Errorf("date %s series_step = %v, want %d", o.Date, o.SeriesStep, step)
		}
	}
	if pendingCount != len(wantPending) {
		t.Errorf("pending count = %d, want %d", pendingCount, len(wantPending))
	}

	reused := f.occurrenceByDate(t, task.ID, "2026-07-09")
	if reused.ID != keptID {
		t.Error("existing pending date should be updated in place, not recreated")
	}
	if !f.hasEvent("occurrence.rescheduled") {
		t.Error("occurrence.rescheduled not emitted")
	}
}

func TestRescheduleMissedNoCandidates(t *testing.T) {
	f := newFixture(t)
	task := f.mustCreate(t, dailyInput("вода"))
	res, err := f.tasks.RescheduleMissed(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("reschedule: %v", err)
	}
	if res.Rescheduled != 0 || res.ShiftDays != 0 {
		t.Errorf("got %+v, want zero result", res)
	}
}

func TestCompleteLastSRStepClosesTask(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	task := f.mustCreate(t, srInput("выучить стих", []int{0}))

	occ := f.occurrenceByDate(t, task.ID, "2026-07-06")
	if _, err := f.occs.Complete(ctx, occ.ID); err != nil {
		t.Fatalf("complete: %v", err)
	}

	updated, err := f.tasks.Get(ctx, task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if updated.Progress != models.ProgressCompleted {
		t.Errorf("task progress %s, want completed", updated.Progress)
	}
	if !f.hasEvent("occurrence.completed") {
		t.Error("occurrence.completed not emitted")
	}
	if !f.hasEvent("task.completed") {
		t.Error("task.completed not emitted")
	}
}

func TestCompletedSRStepNotRegenerated(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	task := f.mustCreate(t, srInput("формулы", []int{0, 1}))

	occ := f.occurrenceByDate(t, task.ID, "2026-07-06")
	if _, err := f.occs.Complete(ctx, occ.ID); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if err := f.tasks.GenerateAll(ctx); err != nil {
		t.Fatalf("generate: %v", err)
	}

	for _, o := range f.occurrences(t, task.ID) {
		if o.Status == models.OccurrencePending && o.SeriesStep != nil && *o.SeriesStep == 0 {
			t.Fatal("completed step 0 was regenerated as pending")
		}
	}
}

func TestMarkMissedEmitsEvent(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.mustCreate(t, dailyInput("дневник"))

	f.clock.Advance(24 * time.Hour)
	n, err := f.tasks.MarkMissed(ctx)
	if err != nil {
		t.Fatalf("mark missed: %v", err)
	}
	if n != 1 {
		t.Fatalf("marked %d, want 1", n)
	}
	if !f.hasEvent("occurrence.missed") {
		t.Error("occurrence.missed not emitted")
	}
}

func TestSkipDoesNotTouchTask(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	task := f.mustCreate(t, dailyInput("уборка"))

	occ := f.occurrenceByDate(t, task.ID, "2026-07-06")
	skipped, err := f.occs.Skip(ctx, occ.ID)
	if err != nil {
		t.Fatalf("skip: %v", err)
	}
	if skipped.Status != models.OccurrenceSkipped {
		t.Errorf("status %s, want skipped", skipped.Status)
	}
	if !f.hasEvent("occurrence.skipped") {
		t.Error("occurrence.skipped not emitted")
	}
	if _, err := f.occs.Skip(ctx, occ.ID); !IsConflict(err) {
		t.Errorf("second skip: err = %v, want conflict", err)
	}
}

func TestCompleteConflictOnDone(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	task := f.mustCreate(t, dailyInput("отчёт"))
	occ := f.occurrenceByDate(t, task.ID, "2026-07-06")

	if _, err := f.occs.Complete(ctx, occ.ID); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if _, err := f.occs.Complete(ctx, occ.ID); !IsConflict(err) {
		t.Errorf("second complete: err = %v, want conflict", err)
	}
}

func TestCreateValidation(t *testing.T) {
	f := newFixture(t)
	due := mustDate(t, "2026-08-01")
	src := "learning"
	tests := []struct {
		name string
		in   TaskInput
	}{
		{"empty title", TaskInput{RecurrenceKind: models.RecurrenceDaily}},
		{"due on recurring", TaskInput{Title: "x", RecurrenceKind: models.RecurrenceDaily, Due: &due}},
		{"params for daily", TaskInput{Title: "x", RecurrenceKind: models.RecurrenceDaily, RecurrenceParams: &models.RecurrenceParams{}}},
		{"missing params", TaskInput{Title: "x", RecurrenceKind: models.RecurrenceEveryNDays}},
		{"bad start time", TaskInput{Title: "x", RecurrenceKind: models.RecurrenceDaily, StartTimeMinutes: intp(1500)}},
		{"bad priority", TaskInput{Title: "x", RecurrenceKind: models.RecurrenceDaily, Priority: intp(15)}},
		{"source without external id", TaskInput{Title: "x", RecurrenceKind: models.RecurrenceDaily, Source: &src}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := f.tasks.Create(context.Background(), tt.in); !IsValidation(err) {
				t.Fatalf("err = %v, want validation error", err)
			}
		})
	}
}

func TestUpdatePreservesSourceAndExternalID(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	src := "learning"
	ext := "R3"
	task := f.mustCreate(t, TaskInput{
		Title:          "повторение Go",
		RecurrenceKind: models.RecurrenceDaily,
		Source:         &src,
		ExternalID:     &ext,
	})

	if _, err := f.tasks.Update(ctx, task.ID, dailyInput("повторение Go (правка)")); err != nil {
		t.Fatalf("update: %v", err)
	}
	updated, err := f.tasks.Get(ctx, task.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if updated.Source == nil || *updated.Source != src {
		t.Errorf("source = %v, want %q", updated.Source, src)
	}
	if updated.ExternalID == nil || *updated.ExternalID != ext {
		t.Errorf("external_id = %v, want %q", updated.ExternalID, ext)
	}
}

func TestDeactivatedTaskLosesPending(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	task := f.mustCreate(t, dailyInput("подкаст"))

	in := dailyInput("подкаст")
	inactive := false
	in.IsActive = &inactive
	if _, err := f.tasks.Update(ctx, task.ID, in); err != nil {
		t.Fatalf("update: %v", err)
	}
	for _, o := range f.occurrences(t, task.ID) {
		if o.Status == models.OccurrencePending {
			t.Fatalf("pending occurrence %s remains after deactivation", o.Date)
		}
	}
}

func TestRescheduleSkipsNonReschedulable(t *testing.T) {
	f := newFixture(t)
	task := f.mustCreate(t, TaskInput{Title: "ежедневная", RecurrenceKind: models.RecurrenceDaily})
	if task.Reschedulable {
		t.Fatalf("ежедневная задача не должна быть переносимой")
	}
	res, err := f.tasks.RescheduleMissed(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if res.Rescheduled != 0 {
		t.Fatalf("непереносимая задача не должна переноситься, перенесено %d", res.Rescheduled)
	}
}

func TestSpacedRepetitionIsReschedulable(t *testing.T) {
	f := newFixture(t)
	task := f.mustCreate(t, TaskInput{
		Title:            "тема go",
		RecurrenceKind:   models.RecurrenceSpacedRepetition,
		RecurrenceParams: &models.RecurrenceParams{Intervals: []int{0, 1, 3}},
	})
	if !task.Reschedulable {
		t.Fatalf("задача с интервальным повторением должна быть переносимой")
	}
}

func TestSRRebuildSupersedesStaleMissed(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	start := mustDate(t, "2026-07-06")
	in := srInput("повторение Go", []int{0, 1, 3, 7})
	in.StartDate = &start
	task := f.mustCreate(t, in)
	if _, err := f.occs.Complete(ctx, f.occurrenceByDate(t, task.ID, "2026-07-06").ID); err != nil {
		t.Fatalf("complete step 0: %v", err)
	}

	f.clock.Advance(5 * 24 * time.Hour)
	if _, err := f.tasks.MarkMissed(ctx); err != nil {
		t.Fatalf("mark missed: %v", err)
	}
	if err := f.tasks.GenerateAll(ctx); err != nil {
		t.Fatalf("generate: %v", err)
	}
	for _, d := range []string{"2026-07-07", "2026-07-09"} {
		if o := f.occurrenceByDate(t, task.ID, d); o == nil || o.Status != models.OccurrenceMissed {
			t.Fatalf("%s: настоящий пропуск должен остаться missed после GenerateAll, got %+v", d, o)
		}
	}

	if _, err := f.occs.Complete(ctx, f.occurrenceByDate(t, task.ID, "2026-07-07").ID); err != nil {
		t.Fatalf("complete missed R1: %v", err)
	}
	cur, err := f.tasks.Get(ctx, task.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if _, err := f.tasks.Update(ctx, task.ID, TaskInput{
		Title:            cur.Title,
		RecurrenceKind:   cur.RecurrenceKind,
		RecurrenceParams: &models.RecurrenceParams{Intervals: []int{0, 1, 7, 11}},
		StartDate:        &cur.StartDate,
	}); err != nil {
		t.Fatalf("update intervals: %v", err)
	}

	stale := f.occurrenceByDate(t, task.ID, "2026-07-09")
	if stale.Status != models.OccurrenceRescheduled {
		t.Fatalf("старый missed R2: status %s, want rescheduled", stale.Status)
	}
	if stale.RescheduledTo == nil || stale.RescheduledTo.String() != "2026-07-13" {
		t.Errorf("rescheduled_to = %v, want 2026-07-13", stale.RescheduledTo)
	}
	if r1 := f.occurrenceByDate(t, task.ID, "2026-07-07"); r1.Status != models.OccurrenceCompleted {
		t.Errorf("выполненный R1 изменён: %s", r1.Status)
	}
	if r2 := f.occurrenceByDate(t, task.ID, "2026-07-13"); r2 == nil || r2.Status != models.OccurrencePending || *r2.SeriesStep != 2 {
		t.Errorf("новый pending R2 на 2026-07-13 не создан: %+v", r2)
	}
	for _, o := range f.occurrences(t, task.ID) {
		if o.Status == models.OccurrenceMissed {
			t.Errorf("остался missed: %s step %v", o.Date, *o.SeriesStep)
		}
	}
	if !f.hasEvent("occurrence.rescheduled") {
		t.Error("occurrence.rescheduled not emitted")
	}
	if updated, _ := f.tasks.Get(ctx, task.ID); updated.StartDate != cur.StartDate {
		t.Errorf("start_date сдвинут: %s → %s (серия не должна сдвигаться)", cur.StartDate, updated.StartDate)
	}
}

func TestSRStaleMissedOfCompletedStepSuperseded(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	start := mustDate(t, "2026-07-06")
	in := srInput("повторное изучение", []int{0, 1, 3})
	in.StartDate = &start
	task := f.mustCreate(t, in)
	if _, err := f.occs.Complete(ctx, f.occurrenceByDate(t, task.ID, "2026-07-06").ID); err != nil {
		t.Fatalf("complete step 0: %v", err)
	}
	f.clock.Advance(3 * 24 * time.Hour)
	if _, err := f.tasks.MarkMissed(ctx); err != nil {
		t.Fatalf("mark missed: %v", err)
	}
	cur, _ := f.tasks.Get(ctx, task.ID)
	if _, err := f.tasks.Update(ctx, task.ID, TaskInput{
		Title:            cur.Title,
		RecurrenceKind:   cur.RecurrenceKind,
		RecurrenceParams: &models.RecurrenceParams{Intervals: []int{0, 3, 4, 6}},
		StartDate:        &cur.StartDate,
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	today := f.occurrenceByDate(t, task.ID, "2026-07-09")
	if today == nil || today.Status != models.OccurrencePending || *today.SeriesStep != 1 {
		t.Fatalf("pending step 1 на сегодня не создан: %+v", today)
	}
	if _, err := f.occs.Complete(ctx, today.ID); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if old := f.occurrenceByDate(t, task.ID, "2026-07-07"); old.Status != models.OccurrenceRescheduled || old.RescheduledTo.String() != "2026-07-09" {
		t.Errorf("старый missed step 1: %s → %v, want rescheduled → 2026-07-09", old.Status, old.RescheduledTo)
	}
}
