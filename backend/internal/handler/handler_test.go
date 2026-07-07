package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/PaulRychkov/task-planner/backend/internal/models"
	"github.com/PaulRychkov/task-planner/backend/internal/repository/memory"
	"github.com/PaulRychkov/task-planner/backend/internal/service"
)

type env struct {
	router http.Handler
	store  *memory.Store
}

func newEnv(t *testing.T) *env {
	t.Helper()
	store := memory.NewStore()
	clock := service.FixedClock{T: time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)}
	topics := service.NewTopicService(store)
	tasks := service.NewTaskService(store, clock, time.UTC, 60)
	occs := service.NewOccurrenceService(store, clock, time.UTC)
	plans := service.NewPlanService(store, clock, time.UTC)
	h := New(store, topics, tasks, occs, plans, clock, time.UTC, zap.NewNop())
	return &env{router: h.Router(), store: store}
}

func (e *env) do(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %s: %v", rec.Body.String(), err)
	}
	return v
}

func (e *env) createTask(t *testing.T, body map[string]any) models.Task {
	t.Helper()
	rec := e.do(t, http.MethodPost, "/api/v1/tasks", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create task: %d %s", rec.Code, rec.Body.String())
	}
	return decode[models.Task](t, rec)
}

func (e *env) todayOccurrence(t *testing.T, taskID string) models.TaskOccurrence {
	t.Helper()
	rec := e.do(t, http.MethodGet, "/api/v1/occurrences?from=2026-07-06&to=2026-07-06", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list occurrences: %d", rec.Code)
	}
	for _, o := range decode[[]models.TaskOccurrence](t, rec) {
		if o.TaskID.String() == taskID {
			return o
		}
	}
	t.Fatalf("no occurrence today for task %s", taskID)
	return models.TaskOccurrence{}
}

func TestHealthz(t *testing.T) {
	e := newEnv(t)
	rec := e.do(t, http.MethodGet, "/healthz", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz: %d", rec.Code)
	}
}

func TestTopicsCRUD(t *testing.T) {
	e := newEnv(t)

	rec := e.do(t, http.MethodPost, "/api/v1/topics", map[string]any{"name": "Go", "description": "язык"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	topic := decode[models.Topic](t, rec)

	rec = e.do(t, http.MethodPost, "/api/v1/topics", map[string]any{"name": "Go"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate: %d, want 409", rec.Code)
	}

	rec = e.do(t, http.MethodGet, "/api/v1/topics", nil)
	if rec.Code != http.StatusOK || len(decode[[]models.Topic](t, rec)) != 1 {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}

	rec = e.do(t, http.MethodPut, "/api/v1/topics/"+topic.ID.String(), map[string]any{"name": "Golang"})
	if rec.Code != http.StatusOK {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}

	rec = e.do(t, http.MethodDelete, "/api/v1/topics/"+topic.ID.String(), nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}

	rec = e.do(t, http.MethodGet, "/api/v1/topics/"+topic.ID.String(), nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get deleted: %d, want 404", rec.Code)
	}
	body := decode[map[string]map[string]string](t, rec)
	if body["error"]["code"] != "not_found" {
		t.Errorf("error envelope: %s", rec.Body.String())
	}
}

func TestTaskLifecycleOverHTTP(t *testing.T) {
	e := newEnv(t)
	task := e.createTask(t, map[string]any{
		"title":              "медитация",
		"recurrence_kind":    "daily",
		"start_time_minutes": 480,
	})

	rec := e.do(t, http.MethodGet, "/api/v1/tasks", nil)
	if rec.Code != http.StatusOK || len(decode[[]models.Task](t, rec)) != 1 {
		t.Fatalf("list tasks: %d %s", rec.Code, rec.Body.String())
	}

	occ := e.todayOccurrence(t, task.ID.String())
	rec = e.do(t, http.MethodPost, "/api/v1/occurrences/"+occ.ID.String()+"/complete", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("complete: %d %s", rec.Code, rec.Body.String())
	}
	completed := decode[models.TaskOccurrence](t, rec)
	if completed.Status != models.OccurrenceCompleted || completed.CompletedAt == nil {
		t.Errorf("completed occurrence: %+v", completed)
	}

	rec = e.do(t, http.MethodPost, "/api/v1/occurrences/"+occ.ID.String()+"/complete", nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("double complete: %d, want 409", rec.Code)
	}

	rec = e.do(t, http.MethodPost, "/api/v1/occurrences/"+occ.ID.String()+"/skip", nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("skip completed: %d, want 409", rec.Code)
	}

	rec = e.do(t, http.MethodDelete, "/api/v1/tasks/"+task.ID.String(), nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete task: %d", rec.Code)
	}
}

func TestTaskValidationOverHTTP(t *testing.T) {
	e := newEnv(t)
	tests := []struct {
		name string
		body map[string]any
	}{
		{"no title", map[string]any{"recurrence_kind": "daily"}},
		{"bad kind", map[string]any{"title": "x", "recurrence_kind": "hourly"}},
		{"due with daily", map[string]any{"title": "x", "recurrence_kind": "daily", "due": "2026-08-01"}},
		{"sr without intervals", map[string]any{"title": "x", "recurrence_kind": "spaced_repetition", "recurrence_params": map[string]any{}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := e.do(t, http.MethodPost, "/api/v1/tasks", tt.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("code %d, want 400: %s", rec.Code, rec.Body.String())
			}
			body := decode[map[string]map[string]string](t, rec)
			if body["error"]["code"] != "validation" {
				t.Errorf("error envelope: %s", rec.Body.String())
			}
		})
	}
}

func TestOccurrenceFiltersOverHTTP(t *testing.T) {
	e := newEnv(t)
	task := e.createTask(t, map[string]any{"title": "бег", "recurrence_kind": "daily"})

	rec := e.do(t, http.MethodGet, "/api/v1/occurrences?from=2026-07-06&to=2026-07-08&status=pending", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("filtered list: %d", rec.Code)
	}
	occs := decode[[]models.TaskOccurrence](t, rec)
	if len(occs) != 3 {
		t.Fatalf("got %d occurrences, want 3", len(occs))
	}
	for _, o := range occs {
		if o.TaskID != task.ID {
			t.Errorf("foreign occurrence in filter result")
		}
	}

	rec = e.do(t, http.MethodGet, "/api/v1/occurrences?from=bad-date", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad from: %d, want 400", rec.Code)
	}
	rec = e.do(t, http.MethodGet, "/api/v1/occurrences?status=lost", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad status: %d, want 400", rec.Code)
	}
}

func TestPlanFlowOverHTTP(t *testing.T) {
	e := newEnv(t)
	task := e.createTask(t, map[string]any{"title": "план", "recurrence_kind": "daily"})
	occ := e.todayOccurrence(t, task.ID.String())

	rec := e.do(t, http.MethodGet, "/api/v1/plans/2026-07-06", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("empty plan: %d, want 404", rec.Code)
	}

	rec = e.do(t, http.MethodPut, "/api/v1/plans/2026-07-06/items", map[string]any{
		"items": []map[string]any{{"occurrence_id": occ.ID.String(), "planned_start_minutes": 600}},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("put items: %d %s", rec.Code, rec.Body.String())
	}
	plan := decode[models.DayPlan](t, rec)
	if len(plan.Items) != 1 || plan.CommittedAt != nil {
		t.Fatalf("draft plan: %+v", plan)
	}

	rec = e.do(t, http.MethodPost, "/api/v1/plans/2026-07-06/commit", map[string]any{"committed_by": "app"})
	if rec.Code != http.StatusOK {
		t.Fatalf("commit: %d %s", rec.Code, rec.Body.String())
	}
	committed := decode[models.DayPlan](t, rec)
	if committed.CommittedAt == nil || *committed.CommittedBy != models.ActorApp {
		t.Fatalf("committed plan: %+v", committed)
	}

	rec = e.do(t, http.MethodPost, "/api/v1/plans/2026-07-06/commit", map[string]any{"committed_by": "app"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("double commit: %d, want 409", rec.Code)
	}

	rec = e.do(t, http.MethodGet, "/api/v1/plans/not-a-date", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad date: %d, want 400", rec.Code)
	}
}

func TestRescheduleMissedOverHTTP(t *testing.T) {
	e := newEnv(t)
	task := e.createTask(t, map[string]any{
		"title":           "просрочка",
		"recurrence_kind": "daily",
		"start_date":      "2026-07-01",
	})

	rec := e.do(t, http.MethodPost, "/api/v1/tasks/"+task.ID.String()+"/reschedule-missed", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("reschedule: %d %s", rec.Code, rec.Body.String())
	}
	res := decode[service.RescheduleResult](t, rec)
	if res.ShiftDays != 0 {
		t.Errorf("shift %d, want 0 (window starts today)", res.ShiftDays)
	}

	rec = e.do(t, http.MethodPost, "/api/v1/tasks/not-a-uuid/reschedule-missed", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad uuid: %d, want 400", rec.Code)
	}
}

func TestCalendarICS(t *testing.T) {
	e := newEnv(t)
	e.createTask(t, map[string]any{
		"title":                      "встреча; статус",
		"recurrence_kind":            "once",
		"start_date":                 "2026-07-06",
		"start_time_minutes":         600,
		"estimated_duration_minutes": 45,
	})
	e.createTask(t, map[string]any{
		"title":           "весь день",
		"recurrence_kind": "once",
		"start_date":      "2026-07-07",
		"all_day":         true,
	})
	e.createTask(t, map[string]any{
		"title":           "очень длинное название задачи, чтобы строка SUMMARY гарантированно превысила лимит в семьдесят пять октетов",
		"recurrence_kind": "once",
		"start_date":      "2026-07-08",
		"all_day":         true,
	})

	rec := e.do(t, http.MethodGet, "/calendar.ics", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("ics: %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"BEGIN:VCALENDAR",
		"END:VCALENDAR",
		"SUMMARY:встреча\\; статус",
		"DTSTART:20260706T100000Z",
		"DTEND:20260706T104500Z",
		"DTSTART;VALUE=DATE:20260707",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("ics missing %q", want)
		}
	}
	for _, line := range strings.Split(body, "\r\n") {
		if len(line) > 75 {
			t.Errorf("line exceeds 75 octets: %q", line)
		}
	}
	unfolded := strings.ReplaceAll(body, "\r\n ", "")
	if !strings.Contains(unfolded, "превысила лимит в семьдесят пять октетов") {
		t.Error("folded summary lost content")
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/calendar") {
		t.Errorf("content type %s", ct)
	}
}

func TestNotFoundEnvelope(t *testing.T) {
	e := newEnv(t)
	rec := e.do(t, http.MethodGet, fmt.Sprintf("/api/v1/tasks/%s", "11111111-1111-1111-1111-111111111111"), nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code %d, want 404", rec.Code)
	}
	body := decode[map[string]map[string]string](t, rec)
	if body["error"]["code"] != "not_found" || body["error"]["message"] == "" {
		t.Errorf("error envelope: %s", rec.Body.String())
	}
}
