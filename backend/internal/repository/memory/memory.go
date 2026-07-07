package memory

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/PaulRychkov/task-planner/backend/internal/models"
	"github.com/PaulRychkov/task-planner/backend/internal/repository"
)

type Store struct {
	mu      sync.Mutex
	topics  map[uuid.UUID]models.Topic
	tasks   map[uuid.UUID]models.Task
	occs    map[uuid.UUID]models.TaskOccurrence
	plans   map[uuid.UUID]models.DayPlan
	items   map[uuid.UUID]models.DayPlanItem
	outbox  map[uuid.UUID]models.OutboxEvent
	nowFunc func() time.Time
}

func NewStore() *Store {
	return &Store{
		topics:  map[uuid.UUID]models.Topic{},
		tasks:   map[uuid.UUID]models.Task{},
		occs:    map[uuid.UUID]models.TaskOccurrence{},
		plans:   map[uuid.UUID]models.DayPlan{},
		items:   map[uuid.UUID]models.DayPlanItem{},
		outbox:  map[uuid.UUID]models.OutboxEvent{},
		nowFunc: time.Now,
	}
}

func (s *Store) InTx(ctx context.Context, fn func(repository.Store) error) error {
	return fn(s)
}

func (s *Store) Ping(ctx context.Context) error { return nil }

func (s *Store) Topics() repository.TopicRepo           { return &topicRepo{s} }
func (s *Store) Tasks() repository.TaskRepo             { return &taskRepo{s} }
func (s *Store) Occurrences() repository.OccurrenceRepo { return &occRepo{s} }
func (s *Store) Plans() repository.PlanRepo             { return &planRepo{s} }
func (s *Store) Outbox() repository.OutboxRepo          { return &outboxRepo{s} }

func (s *Store) OutboxEvents() []models.OutboxEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]models.OutboxEvent, 0, len(s.outbox))
	for _, e := range s.outbox {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

func (s *Store) EventTypes() []string {
	events := s.OutboxEvents()
	out := make([]string, 0, len(events))
	for _, e := range events {
		out = append(out, e.EventType)
	}
	return out
}

func copyTopic(t models.Topic) *models.Topic { c := t; return &c }

func (s *Store) taskWithTopic(t models.Task) *models.Task {
	c := t
	if t.TopicID != nil {
		if topic, ok := s.topics[*t.TopicID]; ok {
			c.Topic = copyTopic(topic)
		}
	}
	return &c
}

func (s *Store) occWithTask(o models.TaskOccurrence) *models.TaskOccurrence {
	c := o
	if task, ok := s.tasks[o.TaskID]; ok {
		c.Task = s.taskWithTopic(task)
	}
	return &c
}

type topicRepo struct{ s *Store }

func (r *topicRepo) List(ctx context.Context, includeArchived bool) ([]models.Topic, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []models.Topic
	for _, t := range r.s.topics {
		if includeArchived || !t.IsArchived {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out, nil
}

func (r *topicRepo) Get(ctx context.Context, id uuid.UUID) (*models.Topic, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	t, ok := r.s.topics[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return copyTopic(t), nil
}

func (r *topicRepo) GetByName(ctx context.Context, name string) (*models.Topic, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, t := range r.s.topics {
		if t.Name == name {
			return copyTopic(t), nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *topicRepo) Create(ctx context.Context, t *models.Topic) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	repository.EnsureID(&t.ID)
	for _, existing := range r.s.topics {
		if existing.Name == t.Name {
			return repository.ErrConflict
		}
	}
	now := r.s.nowFunc()
	t.CreatedAt, t.UpdatedAt = now, now
	r.s.topics[t.ID] = *t
	return nil
}

func (r *topicRepo) Update(ctx context.Context, t *models.Topic) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if _, ok := r.s.topics[t.ID]; !ok {
		return repository.ErrNotFound
	}
	for _, existing := range r.s.topics {
		if existing.ID != t.ID && existing.Name == t.Name {
			return repository.ErrConflict
		}
	}
	t.UpdatedAt = r.s.nowFunc()
	r.s.topics[t.ID] = *t
	return nil
}

func (r *topicRepo) Delete(ctx context.Context, id uuid.UUID) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if _, ok := r.s.topics[id]; !ok {
		return repository.ErrNotFound
	}
	delete(r.s.topics, id)
	for tid, task := range r.s.tasks {
		if task.TopicID != nil && *task.TopicID == id {
			task.TopicID = nil
			r.s.tasks[tid] = task
		}
	}
	return nil
}

type taskRepo struct{ s *Store }

func (r *taskRepo) sorted(filter func(models.Task) bool) []models.Task {
	var out []models.Task
	for _, t := range r.s.tasks {
		if filter(t) {
			out = append(out, *r.s.taskWithTopic(t))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID.String() < out[j].ID.String()
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

func (r *taskRepo) List(ctx context.Context) ([]models.Task, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	return r.sorted(func(models.Task) bool { return true }), nil
}

func (r *taskRepo) ListActive(ctx context.Context) ([]models.Task, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	return r.sorted(func(t models.Task) bool { return t.IsActive && t.Progress.Open() }), nil
}

func (r *taskRepo) ListOpenWithDue(ctx context.Context, from, to models.Date) ([]models.Task, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	return r.sorted(func(t models.Task) bool {
		return t.Due != nil && !t.Due.Before(from) && !t.Due.After(to) && t.Progress.Open()
	}), nil
}

func (r *taskRepo) Get(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	t, ok := r.s.tasks[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return r.s.taskWithTopic(t), nil
}

func (r *taskRepo) checkSourceUnique(t *models.Task) error {
	if t.Source == nil {
		return nil
	}
	for _, existing := range r.s.tasks {
		if existing.ID != t.ID && existing.Source != nil &&
			*existing.Source == *t.Source && *existing.ExternalID == *t.ExternalID {
			return repository.ErrConflict
		}
	}
	return nil
}

func (r *taskRepo) Create(ctx context.Context, t *models.Task) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	repository.EnsureID(&t.ID)
	if err := r.checkSourceUnique(t); err != nil {
		return err
	}
	now := r.s.nowFunc()
	t.CreatedAt, t.UpdatedAt = now, now
	stored := *t
	stored.Topic = nil
	r.s.tasks[t.ID] = stored
	return nil
}

func (r *taskRepo) Update(ctx context.Context, t *models.Task) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if _, ok := r.s.tasks[t.ID]; !ok {
		return repository.ErrNotFound
	}
	if err := r.checkSourceUnique(t); err != nil {
		return err
	}
	t.UpdatedAt = r.s.nowFunc()
	stored := *t
	stored.Topic = nil
	r.s.tasks[t.ID] = stored
	return nil
}

func (r *taskRepo) Delete(ctx context.Context, id uuid.UUID) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if _, ok := r.s.tasks[id]; !ok {
		return repository.ErrNotFound
	}
	delete(r.s.tasks, id)
	for oid, occ := range r.s.occs {
		if occ.TaskID == id {
			delete(r.s.occs, oid)
			r.s.deleteItemsByOccurrenceLocked(oid)
		}
	}
	return nil
}

func (s *Store) deleteItemsByOccurrenceLocked(occurrenceID uuid.UUID) {
	for iid, item := range s.items {
		if item.OccurrenceID == occurrenceID {
			delete(s.items, iid)
		}
	}
}

type occRepo struct{ s *Store }

func (r *occRepo) Get(ctx context.Context, id uuid.UUID) (*models.TaskOccurrence, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	o, ok := r.s.occs[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return r.s.occWithTask(o), nil
}

func (r *occRepo) List(ctx context.Context, f repository.OccurrenceFilter) ([]models.TaskOccurrence, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []models.TaskOccurrence
	for _, o := range r.s.occs {
		if f.TaskID != nil && o.TaskID != *f.TaskID {
			continue
		}
		if f.From != nil && o.Date.Before(*f.From) {
			continue
		}
		if f.To != nil && o.Date.After(*f.To) {
			continue
		}
		if f.Status != nil && o.Status != *f.Status {
			continue
		}
		out = append(out, *r.s.occWithTask(o))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Date == out[j].Date {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].Date.Before(out[j].Date)
	})
	return out, nil
}

func (r *occRepo) CompletedSteps(ctx context.Context, taskID uuid.UUID) ([]int, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var steps []int
	for _, o := range r.s.occs {
		if o.TaskID == taskID && o.Status == models.OccurrenceCompleted && o.SeriesStep != nil {
			steps = append(steps, *o.SeriesStep)
		}
	}
	sort.Ints(steps)
	return steps, nil
}

func (r *occRepo) InsertIgnoreConflict(ctx context.Context, o *models.TaskOccurrence) (bool, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	repository.EnsureID(&o.ID)
	for _, existing := range r.s.occs {
		if existing.TaskID == o.TaskID && existing.Date == o.Date {
			return false, nil
		}
	}
	now := r.s.nowFunc()
	o.CreatedAt, o.UpdatedAt = now, now
	stored := *o
	stored.Task = nil
	r.s.occs[o.ID] = stored
	return true, nil
}

func (r *occRepo) Update(ctx context.Context, o *models.TaskOccurrence) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if _, ok := r.s.occs[o.ID]; !ok {
		return repository.ErrNotFound
	}
	if o.Status == models.OccurrenceCompleted && o.SeriesStep != nil {
		for _, existing := range r.s.occs {
			if existing.ID != o.ID && existing.TaskID == o.TaskID &&
				existing.Status == models.OccurrenceCompleted &&
				existing.SeriesStep != nil && *existing.SeriesStep == *o.SeriesStep {
				return repository.ErrConflict
			}
		}
	}
	o.UpdatedAt = r.s.nowFunc()
	stored := *o
	stored.Task = nil
	r.s.occs[o.ID] = stored
	return nil
}

func (r *occRepo) Delete(ctx context.Context, id uuid.UUID) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if _, ok := r.s.occs[id]; !ok {
		return repository.ErrNotFound
	}
	delete(r.s.occs, id)
	r.s.deleteItemsByOccurrenceLocked(id)
	return nil
}

type planRepo struct{ s *Store }

func (r *planRepo) planWithItemsLocked(p models.DayPlan) *models.DayPlan {
	c := p
	c.Items = nil
	for _, item := range r.s.items {
		if item.PlanID == p.ID {
			ic := item
			if occ, ok := r.s.occs[item.OccurrenceID]; ok {
				ic.Occurrence = r.s.occWithTask(occ)
			}
			c.Items = append(c.Items, ic)
		}
	}
	sort.Slice(c.Items, func(i, j int) bool { return c.Items[i].Position < c.Items[j].Position })
	return &c
}

func (r *planRepo) GetByDate(ctx context.Context, date models.Date) (*models.DayPlan, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, p := range r.s.plans {
		if p.Date == date {
			return r.planWithItemsLocked(p), nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *planRepo) Create(ctx context.Context, p *models.DayPlan) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	repository.EnsureID(&p.ID)
	for _, existing := range r.s.plans {
		if existing.Date == p.Date {
			return repository.ErrConflict
		}
	}
	now := r.s.nowFunc()
	p.CreatedAt, p.UpdatedAt = now, now
	stored := *p
	stored.Items = nil
	r.s.plans[p.ID] = stored
	return nil
}

func (r *planRepo) Update(ctx context.Context, p *models.DayPlan) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if _, ok := r.s.plans[p.ID]; !ok {
		return repository.ErrNotFound
	}
	p.UpdatedAt = r.s.nowFunc()
	stored := *p
	stored.Items = nil
	r.s.plans[p.ID] = stored
	return nil
}

func (r *planRepo) ReplaceItems(ctx context.Context, planID uuid.UUID, items []models.DayPlanItem) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for iid, item := range r.s.items {
		if item.PlanID == planID {
			delete(r.s.items, iid)
		}
	}
	seen := map[uuid.UUID]bool{}
	for i := range items {
		repository.EnsureID(&items[i].ID)
		items[i].PlanID = planID
		if seen[items[i].OccurrenceID] {
			return repository.ErrConflict
		}
		seen[items[i].OccurrenceID] = true
		items[i].CreatedAt = r.s.nowFunc()
		stored := items[i]
		stored.Occurrence = nil
		r.s.items[items[i].ID] = stored
	}
	return nil
}

func (r *planRepo) CommittedRefsForOccurrences(ctx context.Context, occurrenceIDs []uuid.UUID) ([]repository.CommittedPlanRef, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	wanted := map[uuid.UUID]bool{}
	for _, id := range occurrenceIDs {
		wanted[id] = true
	}
	var out []repository.CommittedPlanRef
	for _, item := range r.s.items {
		if !wanted[item.OccurrenceID] {
			continue
		}
		plan, ok := r.s.plans[item.PlanID]
		if !ok || plan.CommittedAt == nil {
			continue
		}
		out = append(out, repository.CommittedPlanRef{
			PlanID:       plan.ID,
			PlanDate:     plan.Date,
			OccurrenceID: item.OccurrenceID,
		})
	}
	return out, nil
}

type outboxRepo struct{ s *Store }

func (r *outboxRepo) Add(ctx context.Context, e *models.OutboxEvent) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	repository.EnsureID(&e.ID)
	if e.CreatedAt.IsZero() {
		e.CreatedAt = r.s.nowFunc().Add(time.Duration(len(r.s.outbox)) * time.Microsecond)
	}
	r.s.outbox[e.ID] = *e
	return nil
}

func (r *outboxRepo) ListUnpublished(ctx context.Context, limit int) ([]models.OutboxEvent, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []models.OutboxEvent
	for _, e := range r.s.outbox {
		if e.PublishedAt == nil {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *outboxRepo) MarkPublished(ctx context.Context, id uuid.UUID, at time.Time) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	e, ok := r.s.outbox[id]
	if !ok {
		return repository.ErrNotFound
	}
	e.PublishedAt = &at
	r.s.outbox[id] = e
	return nil
}

func (r *outboxRepo) MarkFailed(ctx context.Context, id uuid.UUID, msg string) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	e, ok := r.s.outbox[id]
	if !ok {
		return repository.ErrNotFound
	}
	e.Attempts++
	e.LastError = &msg
	r.s.outbox[id] = e
	return nil
}
