package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/PaulRychkov/task-planner/backend/internal/repository"
)

type Publisher interface {
	Publish(ctx context.Context, key string, value []byte) error
}

type CloudEvent struct {
	SpecVersion     string          `json:"specversion"`
	ID              string          `json:"id"`
	Source          string          `json:"source"`
	Type            string          `json:"type"`
	Subject         string          `json:"subject"`
	Time            time.Time       `json:"time"`
	DataContentType string          `json:"datacontenttype"`
	Data            json.RawMessage `json:"data"`
}

type OutboxRelay struct {
	store     repository.Store
	publisher Publisher
	clock     Clock
	log       *zap.Logger
	source    string
	interval  time.Duration
	batchSize int
}

func NewOutboxRelay(store repository.Store, publisher Publisher, clock Clock, log *zap.Logger, interval time.Duration) *OutboxRelay {
	return &OutboxRelay{
		store:     store,
		publisher: publisher,
		clock:     clock,
		log:       log,
		source:    "tasks",
		interval:  interval,
		batchSize: 100,
	}
}

func (r *OutboxRelay) Run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	r.relayPass(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.relayPass(ctx)
		}
	}
}

func (r *OutboxRelay) relayPass(ctx context.Context) {
	if n, err := r.RelayOnce(ctx); err != nil {
		r.log.Warn("outbox relay pass failed", zap.Int("published", n), zap.Error(err))
	} else if n > 0 {
		r.log.Info("outbox events published", zap.Int("count", n))
	}
}

func (r *OutboxRelay) RelayOnce(ctx context.Context) (int, error) {
	events, err := r.store.Outbox().ListUnpublished(ctx, r.batchSize)
	if err != nil {
		return 0, fmt.Errorf("list unpublished: %w", err)
	}
	published := 0
	for i := range events {
		e := events[i]
		envelope := CloudEvent{
			SpecVersion:     "1.0",
			ID:              e.ID.String(),
			Source:          r.source,
			Type:            e.EventType,
			Subject:         subjectFor(e.AggregateID.String(), e.Payload),
			Time:            e.CreatedAt,
			DataContentType: "application/json",
			Data:            json.RawMessage(e.Payload),
		}
		body, err := json.Marshal(envelope)
		if err != nil {
			return published, fmt.Errorf("marshal cloud event %s: %w", e.ID, err)
		}
		if err := r.publisher.Publish(ctx, e.AggregateID.String(), body); err != nil {
			if markErr := r.store.Outbox().MarkFailed(ctx, e.ID, err.Error()); markErr != nil {
				return published, fmt.Errorf("mark failed: %w", markErr)
			}
			return published, fmt.Errorf("publish %s: %w", e.EventType, err)
		}
		if err := r.store.Outbox().MarkPublished(ctx, e.ID, r.clock.Now()); err != nil {
			return published, fmt.Errorf("mark published: %w", err)
		}
		published++
	}
	return published, nil
}

func subjectFor(aggregateID string, payload []byte) string {
	var probe struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(payload, &probe); err == nil && probe.TaskID != "" {
		return probe.TaskID
	}
	return aggregateID
}
