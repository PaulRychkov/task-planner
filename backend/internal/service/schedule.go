package service

import (
	"fmt"
	"time"

	"github.com/PaulRychkov/task-planner/backend/internal/models"
)

type ScheduledDate struct {
	Date       models.Date
	SeriesStep *int
}

func ValidateRecurrence(kind models.RecurrenceKind, params *models.RecurrenceParams) error {
	if !kind.Valid() {
		return invalid(fmt.Sprintf("unknown recurrence_kind %q", kind))
	}
	if kind.RequiresParams() != (params != nil) {
		if params == nil {
			return invalid(fmt.Sprintf("recurrence_params required for kind %q", kind))
		}
		return invalid(fmt.Sprintf("recurrence_params must be null for kind %q", kind))
	}
	if params == nil {
		return nil
	}
	switch kind {
	case models.RecurrenceDaysOfWeek:
		if len(params.Days) == 0 {
			return invalid("days_of_week requires non-empty days")
		}
		seen := map[int]bool{}
		for _, d := range params.Days {
			if d < 1 || d > 7 {
				return invalid(fmt.Sprintf("day of week %d out of range 1..7", d))
			}
			if seen[d] {
				return invalid(fmt.Sprintf("duplicate day of week %d", d))
			}
			seen[d] = true
		}
	case models.RecurrenceEveryNDays, models.RecurrenceEveryNWeeks:
		if params.N == nil || *params.N < 1 {
			return invalid(fmt.Sprintf("%s requires n >= 1", kind))
		}
	case models.RecurrenceMonthly:
		if params.DayOfMonth != nil && (*params.DayOfMonth < 1 || *params.DayOfMonth > 31) {
			return invalid("day_of_month out of range 1..31")
		}
	case models.RecurrenceSpacedRepetition:
		if len(params.Intervals) == 0 {
			return invalid("spaced_repetition requires non-empty intervals")
		}
		prev := -1
		for _, iv := range params.Intervals {
			if iv < 0 {
				return invalid("intervals must be non-negative")
			}
			if iv <= prev {
				return invalid("intervals must be strictly increasing")
			}
			prev = iv
		}
	}
	return nil
}

func ScheduleDates(task models.Task, from, to models.Date) []ScheduledDate {
	if to.Before(from) {
		return nil
	}
	start := task.StartDate
	params := task.RecurrenceParams

	switch task.RecurrenceKind {
	case models.RecurrenceOnce:
		date := start
		if task.Due != nil {
			date = *task.Due
		}
		if !date.Before(from) {
			return []ScheduledDate{{Date: date}}
		}
		return nil
	case models.RecurrenceDaily:
		return scanDays(start, from, to, func(models.Date) bool { return true })
	case models.RecurrenceWeekdays:
		return scanDays(start, from, to, func(d models.Date) bool { return d.ISOWeekday() <= 5 })
	case models.RecurrenceWeekends:
		return scanDays(start, from, to, func(d models.Date) bool { return d.ISOWeekday() >= 6 })
	case models.RecurrenceDaysOfWeek:
		days := map[int]bool{}
		for _, d := range params.Days {
			days[d] = true
		}
		return scanDays(start, from, to, func(d models.Date) bool { return days[d.ISOWeekday()] })
	case models.RecurrenceEveryNDays:
		return stepDates(start, from, to, *params.N)
	case models.RecurrenceEveryNWeeks:
		return stepDates(start, from, to, *params.N*7)
	case models.RecurrenceMonthly:
		return monthlyDates(start, from, to, params.DayOfMonth)
	case models.RecurrenceSpacedRepetition:
		var out []ScheduledDate
		for i, interval := range params.Intervals {
			d := start.AddDays(interval)
			if d.Before(from) || d.After(to) {
				continue
			}
			step := i
			out = append(out, ScheduledDate{Date: d, SeriesStep: &step})
		}
		return out
	}
	return nil
}

func scanDays(start, from, to models.Date, match func(models.Date) bool) []ScheduledDate {
	d := from
	if d.Before(start) {
		d = start
	}
	var out []ScheduledDate
	for ; !d.After(to); d = d.AddDays(1) {
		if match(d) {
			out = append(out, ScheduledDate{Date: d})
		}
	}
	return out
}

func stepDates(start, from, to models.Date, step int) []ScheduledDate {
	if step < 1 {
		return nil
	}
	diff := from.DaysSince(start)
	k := 0
	if diff > 0 {
		k = (diff + step - 1) / step
	}
	var out []ScheduledDate
	for d := start.AddDays(k * step); !d.After(to); d = d.AddDays(step) {
		out = append(out, ScheduledDate{Date: d})
	}
	return out
}

func monthlyDates(start, from, to models.Date, dayOfMonth *int) []ScheduledDate {
	dom := start.Day
	if dayOfMonth != nil {
		dom = *dayOfMonth
	}
	var out []ScheduledDate
	year, month := start.Year, start.Month
	for {
		monthStart := models.NewDate(year, month, 1)
		if monthStart.After(to) {
			break
		}
		day := dom
		if max := models.DaysInMonth(year, month); day > max {
			day = max
		}
		d := models.NewDate(year, month, day)
		if !d.Before(start) && !d.Before(from) && !d.After(to) {
			out = append(out, ScheduledDate{Date: d})
		}
		month++
		if month > time.December {
			month = time.January
			year++
		}
	}
	return out
}
