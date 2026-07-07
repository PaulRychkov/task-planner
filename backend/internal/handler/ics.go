package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/PaulRychkov/task-planner/backend/internal/models"
	"github.com/PaulRychkov/task-planner/backend/internal/service"
)

const (
	icsPastDays      = 30
	icsMaxLineOctets = 75
	icsUTCLayout     = "20060102T150405Z"
)

func (h *Handler) calendarICS(c *gin.Context) {
	today := service.Today(h.clock, h.loc)
	from := today.AddDays(-icsPastDays)
	to := today.AddDays(h.tasks.WindowDays())

	occs, err := h.occs.List(c.Request.Context(), &from, &to, nil)
	if err != nil {
		h.fail(c, err)
		return
	}

	body := buildICS(occs, h.clock, h.loc)
	c.Header("Content-Disposition", `attachment; filename="tasks.ics"`)
	c.Data(http.StatusOK, "text/calendar; charset=utf-8", []byte(body))
}

func buildICS(occs []models.TaskOccurrence, clock service.Clock, loc *time.Location) string {
	var b strings.Builder
	writeLine := func(s string) {
		writeFolded(&b, s)
	}

	writeLine("BEGIN:VCALENDAR")
	writeLine("VERSION:2.0")
	writeLine("PRODID:-//activization//tasks//RU")
	writeLine("CALSCALE:GREGORIAN")
	writeLine("X-WR-CALNAME:Tasks")

	stamp := clock.Now().UTC().Format(icsUTCLayout)

	for i := range occs {
		o := occs[i]
		if o.Task == nil {
			continue
		}
		writeLine("BEGIN:VEVENT")
		writeLine("UID:" + o.ID.String() + "@tasks")
		writeLine("DTSTAMP:" + stamp)

		if o.Task.StartTimeMinutes != nil && !o.Task.AllDay {
			startMin := *o.Task.StartTimeMinutes
			start := time.Date(o.Date.Year, o.Date.Month, o.Date.Day, startMin/60, startMin%60, 0, 0, loc)
			writeLine("DTSTART:" + start.UTC().Format(icsUTCLayout))
			duration := 30
			if o.Task.EstimatedDurationMinutes != nil {
				duration = *o.Task.EstimatedDurationMinutes
			}
			end := start.Add(time.Duration(duration) * time.Minute)
			writeLine("DTEND:" + end.UTC().Format(icsUTCLayout))
		} else {
			writeLine("DTSTART;VALUE=DATE:" + icsDate(o.Date))
			writeLine("DTEND;VALUE=DATE:" + icsDate(o.Date.AddDays(1)))
		}

		writeLine("SUMMARY:" + icsEscape(o.Task.Title))
		if topic := o.Task.TopicName(); topic != "" {
			writeLine("CATEGORIES:" + icsEscape(topic))
		}
		if o.Task.Description != nil && *o.Task.Description != "" {
			writeLine("DESCRIPTION:" + icsEscape(*o.Task.Description))
		}
		writeLine("STATUS:" + icsStatus(o.Status))
		writeLine("END:VEVENT")
	}

	writeLine("END:VCALENDAR")
	return b.String()
}

func writeFolded(b *strings.Builder, line string) {
	limit := icsMaxLineOctets
	for len(line) > limit {
		cut := limit
		for cut > 0 && !utf8.RuneStart(line[cut]) {
			cut--
		}
		b.WriteString(line[:cut])
		b.WriteString("\r\n ")
		line = line[cut:]
		limit = icsMaxLineOctets - 1
	}
	b.WriteString(line)
	b.WriteString("\r\n")
}

func icsDate(d models.Date) string {
	return fmt.Sprintf("%04d%02d%02d", d.Year, int(d.Month), d.Day)
}

func icsStatus(s models.OccurrenceStatus) string {
	switch s {
	case models.OccurrenceSkipped, models.OccurrenceRescheduled:
		return "CANCELLED"
	case models.OccurrenceMissed:
		return "TENTATIVE"
	default:
		return "CONFIRMED"
	}
}

func icsEscape(s string) string {
	r := strings.NewReplacer(
		"\\", "\\\\",
		";", "\\;",
		",", "\\,",
		"\r\n", "\\n",
		"\n", "\\n",
	)
	return r.Replace(s)
}
