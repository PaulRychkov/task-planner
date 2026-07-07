package service

import (
	"testing"
	"time"

	"github.com/PaulRychkov/task-planner/backend/internal/models"
)

func intp(n int) *int { return &n }

func mustDate(t *testing.T, s string) models.Date {
	t.Helper()
	d, err := models.ParseDate(s)
	if err != nil {
		t.Fatalf("parse date %s: %v", s, err)
	}
	return d
}

func taskWith(kind models.RecurrenceKind, params *models.RecurrenceParams, start models.Date) models.Task {
	return models.Task{RecurrenceKind: kind, RecurrenceParams: params, StartDate: start}
}

func datesOf(sched []ScheduledDate) []string {
	out := make([]string, 0, len(sched))
	for _, sd := range sched {
		out = append(out, sd.Date.String())
	}
	return out
}

func TestScheduleDates(t *testing.T) {
	tests := []struct {
		name   string
		kind   models.RecurrenceKind
		params *models.RecurrenceParams
		start  string
		from   string
		to     string
		want   []string
	}{
		{
			name: "once inside window",
			kind: models.RecurrenceOnce, start: "2026-07-08",
			from: "2026-07-06", to: "2026-07-12",
			want: []string{"2026-07-08"},
		},
		{
			name: "once outside window",
			kind: models.RecurrenceOnce, start: "2026-07-20",
			from: "2026-07-06", to: "2026-07-12",
			want: nil,
		},
		{
			name: "daily from start",
			kind: models.RecurrenceDaily, start: "2026-07-08",
			from: "2026-07-06", to: "2026-07-10",
			want: []string{"2026-07-08", "2026-07-09", "2026-07-10"},
		},
		{
			name: "weekdays",
			kind: models.RecurrenceWeekdays, start: "2026-07-06",
			from: "2026-07-06", to: "2026-07-12",
			want: []string{"2026-07-06", "2026-07-07", "2026-07-08", "2026-07-09", "2026-07-10"},
		},
		{
			name: "weekends",
			kind: models.RecurrenceWeekends, start: "2026-07-06",
			from: "2026-07-06", to: "2026-07-12",
			want: []string{"2026-07-11", "2026-07-12"},
		},
		{
			name: "days of week mon wed fri",
			kind: models.RecurrenceDaysOfWeek, params: &models.RecurrenceParams{Days: []int{1, 3, 5}},
			start: "2026-07-06", from: "2026-07-06", to: "2026-07-12",
			want: []string{"2026-07-06", "2026-07-08", "2026-07-10"},
		},
		{
			name: "every 2 days aligned to start",
			kind: models.RecurrenceEveryNDays, params: &models.RecurrenceParams{N: intp(2)},
			start: "2026-07-06", from: "2026-07-09", to: "2026-07-14",
			want: []string{"2026-07-10", "2026-07-12", "2026-07-14"},
		},
		{
			name: "every 30 days",
			kind: models.RecurrenceEveryNDays, params: &models.RecurrenceParams{N: intp(30)},
			start: "2026-07-06", from: "2026-07-06", to: "2026-09-30",
			want: []string{"2026-07-06", "2026-08-05", "2026-09-04"},
		},
		{
			name: "every 2 weeks",
			kind: models.RecurrenceEveryNWeeks, params: &models.RecurrenceParams{N: intp(2)},
			start: "2026-07-06", from: "2026-07-06", to: "2026-08-10",
			want: []string{"2026-07-06", "2026-07-20", "2026-08-03"},
		},
		{
			name: "monthly same day",
			kind: models.RecurrenceMonthly, params: &models.RecurrenceParams{},
			start: "2026-07-15", from: "2026-07-01", to: "2026-09-30",
			want: []string{"2026-07-15", "2026-08-15", "2026-09-15"},
		},
		{
			name: "monthly clamped to short month",
			kind: models.RecurrenceMonthly, params: &models.RecurrenceParams{},
			start: "2026-01-31", from: "2026-01-01", to: "2026-04-30",
			want: []string{"2026-01-31", "2026-02-28", "2026-03-31", "2026-04-30"},
		},
		{
			name: "monthly explicit day of month clamped",
			kind: models.RecurrenceMonthly, params: &models.RecurrenceParams{DayOfMonth: intp(31)},
			start: "2026-01-10", from: "2026-01-01", to: "2026-03-31",
			want: []string{"2026-01-31", "2026-02-28", "2026-03-31"},
		},
		{
			name: "monthly february in leap year",
			kind: models.RecurrenceMonthly, params: &models.RecurrenceParams{DayOfMonth: intp(30)},
			start: "2028-01-01", from: "2028-02-01", to: "2028-02-29",
			want: []string{"2028-02-29"},
		},
		{
			name: "spaced repetition window",
			kind: models.RecurrenceSpacedRepetition, params: &models.RecurrenceParams{Intervals: []int{0, 1, 3, 7, 14, 30, 90}},
			start: "2026-07-06", from: "2026-07-06", to: "2026-08-31",
			want: []string{"2026-07-06", "2026-07-07", "2026-07-09", "2026-07-13", "2026-07-20", "2026-08-05"},
		},
		{
			name: "spaced repetition partial window",
			kind: models.RecurrenceSpacedRepetition, params: &models.RecurrenceParams{Intervals: []int{0, 1, 3, 7}},
			start: "2026-07-01", from: "2026-07-06", to: "2026-07-31",
			want: []string{"2026-07-08"},
		},
		{
			name: "empty window",
			kind: models.RecurrenceDaily, start: "2026-07-06",
			from: "2026-07-10", to: "2026-07-09",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := taskWith(tt.kind, tt.params, mustDate(t, tt.start))
			got := datesOf(ScheduleDates(task, mustDate(t, tt.from), mustDate(t, tt.to)))
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestScheduleDatesSeriesSteps(t *testing.T) {
	task := taskWith(
		models.RecurrenceSpacedRepetition,
		&models.RecurrenceParams{Intervals: []int{0, 1, 3, 7}},
		mustDate(t, "2026-07-06"),
	)
	sched := ScheduleDates(task, mustDate(t, "2026-07-07"), mustDate(t, "2026-07-31"))
	wantSteps := map[string]int{"2026-07-07": 1, "2026-07-09": 2, "2026-07-13": 3}
	if len(sched) != len(wantSteps) {
		t.Fatalf("got %d dates, want %d", len(sched), len(wantSteps))
	}
	for _, sd := range sched {
		if sd.SeriesStep == nil {
			t.Fatalf("date %s has nil series step", sd.Date)
		}
		if want := wantSteps[sd.Date.String()]; *sd.SeriesStep != want {
			t.Errorf("date %s: step %d, want %d", sd.Date, *sd.SeriesStep, want)
		}
	}
}

func TestScheduleDatesNonSRHaveNoStep(t *testing.T) {
	task := taskWith(models.RecurrenceDaily, nil, mustDate(t, "2026-07-06"))
	for _, sd := range ScheduleDates(task, mustDate(t, "2026-07-06"), mustDate(t, "2026-07-10")) {
		if sd.SeriesStep != nil {
			t.Fatalf("daily date %s has series step %d", sd.Date, *sd.SeriesStep)
		}
	}
}

func TestValidateRecurrence(t *testing.T) {
	tests := []struct {
		name    string
		kind    models.RecurrenceKind
		params  *models.RecurrenceParams
		wantErr bool
	}{
		{"once without params", models.RecurrenceOnce, nil, false},
		{"daily with params", models.RecurrenceDaily, &models.RecurrenceParams{}, true},
		{"days_of_week without params", models.RecurrenceDaysOfWeek, nil, true},
		{"days_of_week valid", models.RecurrenceDaysOfWeek, &models.RecurrenceParams{Days: []int{1, 7}}, false},
		{"days_of_week out of range", models.RecurrenceDaysOfWeek, &models.RecurrenceParams{Days: []int{0}}, true},
		{"days_of_week duplicate", models.RecurrenceDaysOfWeek, &models.RecurrenceParams{Days: []int{2, 2}}, true},
		{"every_n_days valid", models.RecurrenceEveryNDays, &models.RecurrenceParams{N: intp(2)}, false},
		{"every_n_days zero", models.RecurrenceEveryNDays, &models.RecurrenceParams{N: intp(0)}, true},
		{"every_n_days missing n", models.RecurrenceEveryNDays, &models.RecurrenceParams{}, true},
		{"every_n_weeks valid", models.RecurrenceEveryNWeeks, &models.RecurrenceParams{N: intp(1)}, false},
		{"monthly default", models.RecurrenceMonthly, &models.RecurrenceParams{}, false},
		{"monthly day 31", models.RecurrenceMonthly, &models.RecurrenceParams{DayOfMonth: intp(31)}, false},
		{"monthly day 32", models.RecurrenceMonthly, &models.RecurrenceParams{DayOfMonth: intp(32)}, true},
		{"sr valid", models.RecurrenceSpacedRepetition, &models.RecurrenceParams{Intervals: []int{0, 1, 3}}, false},
		{"sr empty", models.RecurrenceSpacedRepetition, &models.RecurrenceParams{}, true},
		{"sr negative", models.RecurrenceSpacedRepetition, &models.RecurrenceParams{Intervals: []int{-1, 0}}, true},
		{"sr not increasing", models.RecurrenceSpacedRepetition, &models.RecurrenceParams{Intervals: []int{0, 3, 3}}, true},
		{"unknown kind", models.RecurrenceKind("weekly"), nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRecurrence(tt.kind, tt.params)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestDateHelpers(t *testing.T) {
	d := mustDate(t, "2026-07-06")
	if wd := d.ISOWeekday(); wd != 1 {
		t.Errorf("2026-07-06 ISO weekday = %d, want 1", wd)
	}
	if s := mustDate(t, "2026-07-05").ISOWeekday(); s != 7 {
		t.Errorf("2026-07-05 ISO weekday = %d, want 7", s)
	}
	if got := d.AddDays(26).String(); got != "2026-08-01" {
		t.Errorf("AddDays(26) = %s, want 2026-08-01", got)
	}
	if got := mustDate(t, "2026-08-01").DaysSince(d); got != 26 {
		t.Errorf("DaysSince = %d, want 26", got)
	}
	if models.DaysInMonth(2028, time.February) != 29 {
		t.Errorf("February 2028 should have 29 days")
	}
	if models.DaysInMonth(2026, time.February) != 28 {
		t.Errorf("February 2026 should have 28 days")
	}
}
