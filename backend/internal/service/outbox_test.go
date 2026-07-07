package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"go.uber.org/zap"
)

type mockPublisher struct {
	keys     []string
	payloads [][]byte
	failN    int
}

func (m *mockPublisher) Publish(_ context.Context, key string, value []byte) error {
	if m.failN > 0 {
		m.failN--
		return errors.New("kafka down")
	}
	m.keys = append(m.keys, key)
	m.payloads = append(m.payloads, value)
	return nil
}

func TestOutboxRelayPublishesCloudEvents(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	task := f.mustCreate(t, dailyInput("отчёт"))
	occ := f.occurrenceByDate(t, task.ID, "2026-07-06")
	if _, err := f.occs.Complete(ctx, occ.ID); err != nil {
		t.Fatalf("complete: %v", err)
	}

	pub := &mockPublisher{}
	relay := NewOutboxRelay(f.store, pub, f.clock, zap.NewNop(), 0)

	n, err := relay.RelayOnce(ctx)
	if err != nil {
		t.Fatalf("relay: %v", err)
	}
	if n != 2 {
		t.Fatalf("published %d, want 2", n)
	}

	var first CloudEvent
	if err := json.Unmarshal(pub.payloads[0], &first); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if first.SpecVersion != "1.0" || first.Source != "tasks" || first.DataContentType != "application/json" {
		t.Errorf("bad envelope: %+v", first)
	}
	if first.Type != "task.created" {
		t.Errorf("first event type %s, want task.created (created_at order)", first.Type)
	}

	var second CloudEvent
	if err := json.Unmarshal(pub.payloads[1], &second); err != nil {
		t.Fatalf("unmarshal second: %v", err)
	}
	if second.Type != "occurrence.completed" {
		t.Errorf("second event type %s, want occurrence.completed", second.Type)
	}
	if second.Subject != task.ID.String() {
		t.Errorf("subject %s, want task id %s (task-scoped subject)", second.Subject, task.ID)
	}
	var data struct {
		TaskID       string  `json:"task_id"`
		OccurrenceID string  `json:"occurrence_id"`
		Date         string  `json:"date"`
		Title        string  `json:"title"`
		SeriesStep   *int    `json:"series_step"`
		CompletedAt  *string `json:"completed_at"`
	}
	if err := json.Unmarshal(second.Data, &data); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	if data.TaskID != task.ID.String() || data.OccurrenceID != occ.ID.String() ||
		data.Date != "2026-07-06" || data.Title != "отчёт" || data.CompletedAt == nil {
		t.Errorf("bad occurrence.completed payload: %+v", data)
	}

	for _, e := range f.store.OutboxEvents() {
		if e.PublishedAt == nil {
			t.Errorf("event %s still unpublished", e.EventType)
		}
	}

	n, err = relay.RelayOnce(ctx)
	if err != nil || n != 0 {
		t.Errorf("second pass: n=%d err=%v, want 0 nil", n, err)
	}
}

func TestOutboxRelayKeepsEventsWhenKafkaDown(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.mustCreate(t, dailyInput("бэкап"))

	pub := &mockPublisher{failN: 1}
	relay := NewOutboxRelay(f.store, pub, f.clock, zap.NewNop(), 0)

	if _, err := relay.RelayOnce(ctx); err == nil {
		t.Fatal("expected error when publisher fails")
	}

	events := f.store.OutboxEvents()
	if len(events) != 1 {
		t.Fatalf("outbox size %d, want 1", len(events))
	}
	e := events[0]
	if e.PublishedAt != nil {
		t.Error("failed event must stay unpublished")
	}
	if e.Attempts != 1 || e.LastError == nil {
		t.Errorf("attempts=%d lastError=%v, want 1 and message", e.Attempts, e.LastError)
	}

	n, err := relay.RelayOnce(ctx)
	if err != nil {
		t.Fatalf("retry pass: %v", err)
	}
	if n != 1 {
		t.Fatalf("retry published %d, want 1", n)
	}
	if got := f.store.OutboxEvents()[0]; got.PublishedAt == nil {
		t.Error("event not marked published after retry")
	}
}

func TestOutboxPartitionKeyIsAggregateID(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	task := f.mustCreate(t, dailyInput("ключи"))

	pub := &mockPublisher{}
	relay := NewOutboxRelay(f.store, pub, f.clock, zap.NewNop(), 0)
	if _, err := relay.RelayOnce(ctx); err != nil {
		t.Fatalf("relay: %v", err)
	}
	if len(pub.keys) != 1 || pub.keys[0] != task.ID.String() {
		t.Errorf("partition key %v, want aggregate id %s", pub.keys, task.ID)
	}

	var env CloudEvent
	if err := json.Unmarshal(pub.payloads[0], &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	events := f.store.OutboxEvents()
	if env.ID != events[0].ID.String() {
		t.Error("cloud event id must equal outbox id")
	}
	var snapshot map[string]any
	if err := json.Unmarshal(env.Data, &snapshot); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}
	for _, field := range []string{"task_id", "due", "start_time_minutes", "estimated_duration_minutes", "priority", "progress"} {
		if _, ok := snapshot[field]; !ok {
			t.Errorf("task snapshot missing %s", field)
		}
	}
}
