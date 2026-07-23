package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/PaulRychkov/task-planner/backend/internal/models"
	"github.com/PaulRychkov/task-planner/backend/internal/service"
)

func textResult(v any) (*mcp.CallToolResult, any, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("marshal tool result: %w", err)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
	}, nil, nil
}

type listTasksInput struct {
	IncludeInactive bool `json:"include_inactive,omitempty" jsonschema:"include disabled and closed tasks"`
}

type createTaskInput struct {
	Title                    string `json:"title" jsonschema:"task title"`
	Description              string `json:"description,omitempty"`
	Topic                    string `json:"topic,omitempty" jsonschema:"topic name, created if missing"`
	ParentTopic              string `json:"parent_topic,omitempty" jsonschema:"parent topic name for the topic, creates the hierarchy if missing"`
	RecurrenceKind           string `json:"recurrence_kind" jsonschema:"once|daily|weekdays|weekends|days_of_week|every_n_days|every_n_weeks|monthly|spaced_repetition"`
	Days                     []int  `json:"days,omitempty" jsonschema:"ISO weekdays 1..7 for days_of_week"`
	N                        int    `json:"n,omitempty" jsonschema:"step for every_n_days / every_n_weeks"`
	DayOfMonth               int    `json:"day_of_month,omitempty" jsonschema:"day of month for monthly, 0 = same as start_date"`
	Intervals                []int  `json:"intervals,omitempty" jsonschema:"spaced repetition intervals in days from start_date"`
	StartDate                string `json:"start_date,omitempty" jsonschema:"YYYY-MM-DD, default today"`
	Due                      string `json:"due,omitempty" jsonschema:"deadline YYYY-MM-DD: the execution window is start_date..due, may span many days; only for once"`
	StartTimeMinutes         int    `json:"start_time_minutes,omitempty" jsonschema:"fixed clock time (minutes from midnight) ONLY for continuous events like meetings or gym; mutually exclusive with effort_minutes; regular tasks must NOT have a fixed time"`
	EstimatedDurationMinutes int    `json:"estimated_duration_minutes,omitempty" jsonschema:"duration of a fixed-time continuous event in minutes"`
	EffortMinutes            int    `json:"effort_minutes,omitempty" jsonschema:"estimated effort in minutes for regular tasks; the pomodoro app converts it to pomodoros (~25 min each); mutually exclusive with start_time_minutes"`
	Priority                 int    `json:"priority,omitempty" jsonschema:"1..9, 1 = highest; omit for no priority (0 cannot be set via MCP)"`
	AllDay                   bool   `json:"all_day,omitempty"`
	RequiresPomodoro         *bool  `json:"requires_pomodoro,omitempty" jsonschema:"whether the task needs pomodoro focus sessions; false for events like gym or meetings, default true"`
}

type createTopicInput struct {
	Name        string `json:"name" jsonschema:"topic name"`
	Parent      string `json:"parent,omitempty" jsonschema:"parent topic name, created if missing; builds a multi-level topic tree"`
	Description string `json:"description,omitempty"`
}

type listTopicsInput struct {
	IncludeArchived bool `json:"include_archived,omitempty"`
}

type occurrenceIDInput struct {
	OccurrenceID string `json:"occurrence_id" jsonschema:"occurrence UUID"`
}

type logProgressInput struct {
	OccurrenceID string `json:"occurrence_id" jsonschema:"occurrence UUID"`
	Minutes      int    `json:"minutes" jsonschema:"worked minutes to add; negative to subtract"`
}

type listDueInput struct {
	From string `json:"from,omitempty" jsonschema:"YYYY-MM-DD, default today"`
	To   string `json:"to,omitempty" jsonschema:"YYYY-MM-DD, default from+7 days"`
}

type dayPlanInput struct {
	Date string `json:"date,omitempty" jsonschema:"YYYY-MM-DD, default today"`
}

type commitPlanInput struct {
	Date        string `json:"date,omitempty" jsonschema:"YYYY-MM-DD, default today"`
	CommittedBy string `json:"committed_by,omitempty" jsonschema:"app|bot|agent, default agent"`
}

type taskIDInput struct {
	TaskID string `json:"task_id" jsonschema:"task UUID"`
}

func (h *Handler) MCPHandler() http.Handler {
	server := mcp.NewServer(&mcp.Implementation{Name: "tasks", Version: "1.0.0"}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_tasks",
		Description: "List task rules with topic, recurrence, schedule fields",
	}, h.mcpListTasks)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_task",
		Description: "Create a task rule; occurrences are generated immediately",
	}, h.mcpCreateTask)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "complete_occurrence",
		Description: "Mark a task occurrence as completed",
	}, h.mcpCompleteOccurrence)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "skip_occurrence",
		Description: "Consciously skip a pending occurrence (not a failure)",
	}, h.mcpSkipOccurrence)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_due",
		Description: "List pending/missed occurrences and open tasks with deadlines in a date range",
	}, h.mcpListDue)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_day_plan",
		Description: "Get the day plan with its items for a date",
	}, h.mcpGetDayPlan)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "commit_day_plan",
		Description: "Commit the draft day plan for a date",
	}, h.mcpCommitDayPlan)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "reschedule_missed",
		Description: "Shift missed occurrences of a task forward to today, moving the whole schedule",
	}, h.mcpRescheduleMissed)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "log_progress",
		Description: "Add worked minutes to a task occurrence (progress toward its effort_minutes); negative minutes subtract",
	}, h.mcpLogProgress)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_topic",
		Description: "Create a topic (optionally under a parent topic) for the multi-level task tree",
	}, h.mcpCreateTopic)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_topics",
		Description: "List topics with their parent_id to see the topic tree",
	}, h.mcpListTopics)

	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
}

func (h *Handler) mcpListTasks(ctx context.Context, _ *mcp.CallToolRequest, in listTasksInput) (*mcp.CallToolResult, any, error) {
	tasks, err := h.tasks.List(ctx)
	if err != nil {
		return nil, nil, err
	}
	if !in.IncludeInactive {
		filtered := tasks[:0]
		for _, t := range tasks {
			if t.IsActive && t.Progress.Open() {
				filtered = append(filtered, t)
			}
		}
		tasks = filtered
	}
	return textResult(tasks)
}

func (h *Handler) mcpCreateTask(ctx context.Context, _ *mcp.CallToolRequest, in createTaskInput) (*mcp.CallToolResult, any, error) {
	kind := models.RecurrenceKind(in.RecurrenceKind)
	input := service.TaskInput{Title: in.Title, RecurrenceKind: kind}

	if in.Description != "" {
		input.Description = &in.Description
	}
	if in.Topic != "" {
		var parentID *uuid.UUID
		if in.ParentTopic != "" {
			parent, err := h.topics.FindOrCreate(ctx, in.ParentTopic)
			if err != nil {
				return nil, nil, err
			}
			parentID = &parent.ID
		}
		topic, err := h.topics.FindOrCreateUnder(ctx, in.Topic, parentID)
		if err != nil {
			return nil, nil, err
		}
		input.TopicID = &topic.ID
	}
	if kind.RequiresParams() {
		params := &models.RecurrenceParams{}
		switch kind {
		case models.RecurrenceDaysOfWeek:
			params.Days = in.Days
		case models.RecurrenceEveryNDays, models.RecurrenceEveryNWeeks:
			n := in.N
			params.N = &n
		case models.RecurrenceMonthly:
			if in.DayOfMonth > 0 {
				dom := in.DayOfMonth
				params.DayOfMonth = &dom
			}
		case models.RecurrenceSpacedRepetition:
			params.Intervals = in.Intervals
		}
		input.RecurrenceParams = params
	}
	if in.StartDate != "" {
		d, err := models.ParseDate(in.StartDate)
		if err != nil {
			return nil, nil, err
		}
		input.StartDate = &d
	}
	if in.Due != "" {
		d, err := models.ParseDate(in.Due)
		if err != nil {
			return nil, nil, err
		}
		input.Due = &d
	}
	if in.StartTimeMinutes > 0 {
		v := in.StartTimeMinutes
		input.StartTimeMinutes = &v
	}
	if in.EstimatedDurationMinutes > 0 {
		v := in.EstimatedDurationMinutes
		input.EstimatedDurationMinutes = &v
	}
	if in.EffortMinutes > 0 {
		v := in.EffortMinutes
		input.EffortMinutes = &v
	}
	input.RequiresPomodoro = in.RequiresPomodoro
	if in.Priority > 0 {
		v := in.Priority
		input.Priority = &v
	}
	if in.AllDay {
		v := true
		input.AllDay = &v
	}

	task, err := h.tasks.Create(ctx, input)
	if err != nil {
		return nil, nil, err
	}
	return textResult(task)
}

func (h *Handler) mcpCompleteOccurrence(ctx context.Context, _ *mcp.CallToolRequest, in occurrenceIDInput) (*mcp.CallToolResult, any, error) {
	id, err := uuid.Parse(in.OccurrenceID)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid occurrence_id: %w", err)
	}
	occ, err := h.occs.Complete(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return textResult(occ)
}

func (h *Handler) mcpSkipOccurrence(ctx context.Context, _ *mcp.CallToolRequest, in occurrenceIDInput) (*mcp.CallToolResult, any, error) {
	id, err := uuid.Parse(in.OccurrenceID)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid occurrence_id: %w", err)
	}
	occ, err := h.occs.Skip(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return textResult(occ)
}

func (h *Handler) mcpListDue(ctx context.Context, _ *mcp.CallToolRequest, in listDueInput) (*mcp.CallToolResult, any, error) {
	from := service.Today(h.clock, h.loc)
	if in.From != "" {
		d, err := models.ParseDate(in.From)
		if err != nil {
			return nil, nil, err
		}
		from = d
	}
	to := from.AddDays(7)
	if in.To != "" {
		d, err := models.ParseDate(in.To)
		if err != nil {
			return nil, nil, err
		}
		to = d
	}

	occs, err := h.occs.List(ctx, &from, &to, nil)
	if err != nil {
		return nil, nil, err
	}
	open := occs[:0]
	for _, o := range occs {
		if o.Status == models.OccurrencePending || o.Status == models.OccurrenceMissed {
			open = append(open, o)
		}
	}
	dueTasks, err := h.tasks.ListOpenWithDue(ctx, from, to)
	if err != nil {
		return nil, nil, err
	}
	return textResult(map[string]any{
		"from":           from,
		"to":             to,
		"occurrences":    open,
		"tasks_with_due": dueTasks,
	})
}

func (h *Handler) resolveDate(s string) (models.Date, error) {
	if s == "" {
		return service.Today(h.clock, h.loc), nil
	}
	return models.ParseDate(s)
}

func (h *Handler) mcpGetDayPlan(ctx context.Context, _ *mcp.CallToolRequest, in dayPlanInput) (*mcp.CallToolResult, any, error) {
	date, err := h.resolveDate(in.Date)
	if err != nil {
		return nil, nil, err
	}
	plan, err := h.plans.Get(ctx, date)
	if service.IsNotFound(err) {
		return textResult(map[string]any{"date": date, "plan": nil})
	}
	if err != nil {
		return nil, nil, err
	}
	return textResult(plan)
}

func (h *Handler) mcpCommitDayPlan(ctx context.Context, _ *mcp.CallToolRequest, in commitPlanInput) (*mcp.CallToolResult, any, error) {
	date, err := h.resolveDate(in.Date)
	if err != nil {
		return nil, nil, err
	}
	by := models.ActorAgent
	if in.CommittedBy != "" {
		by = models.ActorKind(in.CommittedBy)
	}
	plan, err := h.plans.Commit(ctx, date, by)
	if err != nil {
		return nil, nil, err
	}
	return textResult(plan)
}

func (h *Handler) mcpLogProgress(ctx context.Context, _ *mcp.CallToolRequest, in logProgressInput) (*mcp.CallToolResult, any, error) {
	id, err := uuid.Parse(in.OccurrenceID)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid occurrence_id: %w", err)
	}
	occ, err := h.occs.AddProgress(ctx, id, in.Minutes)
	if err != nil {
		return nil, nil, err
	}
	return textResult(occ)
}

func (h *Handler) mcpCreateTopic(ctx context.Context, _ *mcp.CallToolRequest, in createTopicInput) (*mcp.CallToolResult, any, error) {
	var parentID *uuid.UUID
	if in.Parent != "" {
		parent, err := h.topics.FindOrCreate(ctx, in.Parent)
		if err != nil {
			return nil, nil, err
		}
		parentID = &parent.ID
	}
	topic, err := h.topics.FindOrCreateUnder(ctx, in.Name, parentID)
	if err != nil {
		return nil, nil, err
	}
	if in.Description != "" && topic.Description == nil {
		desc := in.Description
		updated, uerr := h.topics.Update(ctx, topic.ID, service.TopicInput{Name: topic.Name, ParentID: topic.ParentID, Description: &desc})
		if uerr != nil {
			return nil, nil, uerr
		}
		topic = updated
	}
	return textResult(topic)
}

func (h *Handler) mcpListTopics(ctx context.Context, _ *mcp.CallToolRequest, in listTopicsInput) (*mcp.CallToolResult, any, error) {
	topics, err := h.topics.List(ctx, in.IncludeArchived)
	if err != nil {
		return nil, nil, err
	}
	return textResult(topics)
}

func (h *Handler) mcpRescheduleMissed(ctx context.Context, _ *mcp.CallToolRequest, in taskIDInput) (*mcp.CallToolResult, any, error) {
	id, err := uuid.Parse(in.TaskID)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid task_id: %w", err)
	}
	result, err := h.tasks.RescheduleMissed(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return textResult(result)
}
