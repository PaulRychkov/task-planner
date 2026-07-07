package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/PaulRychkov/task-planner/backend/internal/models"
)

type gormStore struct {
	db *gorm.DB
}

func NewGormStore(db *gorm.DB) Store {
	return &gormStore{db: db}
}

func (s *gormStore) InTx(ctx context.Context, fn func(Store) error) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&gormStore{db: tx})
	})
}

func (s *gormStore) Ping(ctx context.Context) error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("get sql db: %w", err)
	}
	return sqlDB.PingContext(ctx)
}

func (s *gormStore) Topics() TopicRepo           { return &gormTopics{db: s.db} }
func (s *gormStore) Tasks() TaskRepo             { return &gormTasks{db: s.db} }
func (s *gormStore) Occurrences() OccurrenceRepo { return &gormOccurrences{db: s.db} }
func (s *gormStore) Plans() PlanRepo             { return &gormPlans{db: s.db} }
func (s *gormStore) Outbox() OutboxRepo          { return &gormOutbox{db: s.db} }

func translate(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrConflict
	}
	return err
}

type gormTopics struct {
	db *gorm.DB
}

func (r *gormTopics) List(ctx context.Context, includeArchived bool) ([]models.Topic, error) {
	q := r.db.WithContext(ctx).Order("name")
	if !includeArchived {
		q = q.Where("is_archived = false")
	}
	var out []models.Topic
	if err := q.Find(&out).Error; err != nil {
		return nil, fmt.Errorf("list topics: %w", err)
	}
	return out, nil
}

func (r *gormTopics) Get(ctx context.Context, id uuid.UUID) (*models.Topic, error) {
	var t models.Topic
	if err := r.db.WithContext(ctx).First(&t, "id = ?", id).Error; err != nil {
		return nil, translate(err)
	}
	return &t, nil
}

func (r *gormTopics) GetByName(ctx context.Context, name string) (*models.Topic, error) {
	var t models.Topic
	if err := r.db.WithContext(ctx).First(&t, "name = ?", name).Error; err != nil {
		return nil, translate(err)
	}
	return &t, nil
}

func (r *gormTopics) Create(ctx context.Context, t *models.Topic) error {
	EnsureID(&t.ID)
	return translate(r.db.WithContext(ctx).Create(t).Error)
}

func (r *gormTopics) Update(ctx context.Context, t *models.Topic) error {
	return translate(r.db.WithContext(ctx).Save(t).Error)
}

func (r *gormTopics) Delete(ctx context.Context, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Delete(&models.Topic{}, "id = ?", id)
	if res.Error != nil {
		return translate(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

type gormTasks struct {
	db *gorm.DB
}

func (r *gormTasks) List(ctx context.Context) ([]models.Task, error) {
	var out []models.Task
	if err := r.db.WithContext(ctx).Preload("Topic").Order("created_at").Find(&out).Error; err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	return out, nil
}

func (r *gormTasks) ListActive(ctx context.Context) ([]models.Task, error) {
	var out []models.Task
	err := r.db.WithContext(ctx).Preload("Topic").
		Where("is_active = true AND progress IN ?", []models.TaskProgress{models.ProgressNeedsAction, models.ProgressInProcess}).
		Order("created_at").Find(&out).Error
	if err != nil {
		return nil, fmt.Errorf("list active tasks: %w", err)
	}
	return out, nil
}

func (r *gormTasks) ListOpenWithDue(ctx context.Context, from, to models.Date) ([]models.Task, error) {
	var out []models.Task
	err := r.db.WithContext(ctx).Preload("Topic").
		Where("due IS NOT NULL AND due >= ? AND due <= ? AND progress IN ?",
			from, to, []models.TaskProgress{models.ProgressNeedsAction, models.ProgressInProcess}).
		Order("due").Find(&out).Error
	if err != nil {
		return nil, fmt.Errorf("list tasks with due: %w", err)
	}
	return out, nil
}

func (r *gormTasks) Get(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	var t models.Task
	if err := r.db.WithContext(ctx).Preload("Topic").First(&t, "id = ?", id).Error; err != nil {
		return nil, translate(err)
	}
	return &t, nil
}

func (r *gormTasks) Create(ctx context.Context, t *models.Task) error {
	EnsureID(&t.ID)
	return translate(r.db.WithContext(ctx).Omit("Topic").Create(t).Error)
}

func (r *gormTasks) Update(ctx context.Context, t *models.Task) error {
	return translate(r.db.WithContext(ctx).Omit("Topic").Save(t).Error)
}

func (r *gormTasks) Delete(ctx context.Context, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Delete(&models.Task{}, "id = ?", id)
	if res.Error != nil {
		return translate(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

type gormOccurrences struct {
	db *gorm.DB
}

func (r *gormOccurrences) Get(ctx context.Context, id uuid.UUID) (*models.TaskOccurrence, error) {
	var o models.TaskOccurrence
	if err := r.db.WithContext(ctx).Preload("Task.Topic").First(&o, "id = ?", id).Error; err != nil {
		return nil, translate(err)
	}
	return &o, nil
}

func (r *gormOccurrences) List(ctx context.Context, f OccurrenceFilter) ([]models.TaskOccurrence, error) {
	q := r.db.WithContext(ctx).Preload("Task.Topic").Order("date, created_at")
	if f.TaskID != nil {
		q = q.Where("task_id = ?", *f.TaskID)
	}
	if f.From != nil {
		q = q.Where("date >= ?", *f.From)
	}
	if f.To != nil {
		q = q.Where("date <= ?", *f.To)
	}
	if f.Status != nil {
		q = q.Where("status = ?", *f.Status)
	}
	var out []models.TaskOccurrence
	if err := q.Find(&out).Error; err != nil {
		return nil, fmt.Errorf("list occurrences: %w", err)
	}
	return out, nil
}

func (r *gormOccurrences) CompletedSteps(ctx context.Context, taskID uuid.UUID) ([]int, error) {
	var steps []int
	err := r.db.WithContext(ctx).Model(&models.TaskOccurrence{}).
		Where("task_id = ? AND status = ? AND series_step IS NOT NULL", taskID, models.OccurrenceCompleted).
		Order("series_step").Pluck("series_step", &steps).Error
	if err != nil {
		return nil, fmt.Errorf("completed steps: %w", err)
	}
	return steps, nil
}

func (r *gormOccurrences) InsertIgnoreConflict(ctx context.Context, o *models.TaskOccurrence) (bool, error) {
	EnsureID(&o.ID)
	res := r.db.WithContext(ctx).Omit("Task").Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "task_id"}, {Name: "date"}},
		DoNothing: true,
	}).Create(o)
	if res.Error != nil {
		return false, translate(res.Error)
	}
	return res.RowsAffected > 0, nil
}

func (r *gormOccurrences) Update(ctx context.Context, o *models.TaskOccurrence) error {
	return translate(r.db.WithContext(ctx).Omit("Task").Save(o).Error)
}

func (r *gormOccurrences) Delete(ctx context.Context, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Delete(&models.TaskOccurrence{}, "id = ?", id)
	if res.Error != nil {
		return translate(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

type gormPlans struct {
	db *gorm.DB
}

func (r *gormPlans) GetByDate(ctx context.Context, date models.Date) (*models.DayPlan, error) {
	var p models.DayPlan
	err := r.db.WithContext(ctx).
		Preload("Items", func(db *gorm.DB) *gorm.DB { return db.Order("position") }).
		Preload("Items.Occurrence.Task.Topic").
		First(&p, "date = ?", date).Error
	if err != nil {
		return nil, translate(err)
	}
	return &p, nil
}

func (r *gormPlans) Create(ctx context.Context, p *models.DayPlan) error {
	EnsureID(&p.ID)
	return translate(r.db.WithContext(ctx).Omit("Items").Create(p).Error)
}

func (r *gormPlans) Update(ctx context.Context, p *models.DayPlan) error {
	return translate(r.db.WithContext(ctx).Omit("Items").Save(p).Error)
}

func (r *gormPlans) ReplaceItems(ctx context.Context, planID uuid.UUID, items []models.DayPlanItem) error {
	if err := r.db.WithContext(ctx).Delete(&models.DayPlanItem{}, "plan_id = ?", planID).Error; err != nil {
		return fmt.Errorf("delete plan items: %w", err)
	}
	if len(items) == 0 {
		return nil
	}
	for i := range items {
		EnsureID(&items[i].ID)
		items[i].PlanID = planID
	}
	return translate(r.db.WithContext(ctx).Omit("Occurrence").Create(&items).Error)
}

func (r *gormPlans) CommittedRefsForOccurrences(ctx context.Context, occurrenceIDs []uuid.UUID) ([]CommittedPlanRef, error) {
	if len(occurrenceIDs) == 0 {
		return nil, nil
	}
	var out []CommittedPlanRef
	err := r.db.WithContext(ctx).Table("day_plan_items").
		Select("day_plan_items.plan_id AS plan_id, day_plans.date AS plan_date, day_plan_items.occurrence_id AS occurrence_id").
		Joins("JOIN day_plans ON day_plans.id = day_plan_items.plan_id").
		Where("day_plans.committed_at IS NOT NULL AND day_plan_items.occurrence_id IN ?", occurrenceIDs).
		Scan(&out).Error
	if err != nil {
		return nil, fmt.Errorf("committed refs: %w", err)
	}
	return out, nil
}

type gormOutbox struct {
	db *gorm.DB
}

func (r *gormOutbox) Add(ctx context.Context, e *models.OutboxEvent) error {
	EnsureID(&e.ID)
	return translate(r.db.WithContext(ctx).Create(e).Error)
}

func (r *gormOutbox) ListUnpublished(ctx context.Context, limit int) ([]models.OutboxEvent, error) {
	var out []models.OutboxEvent
	err := r.db.WithContext(ctx).
		Where("published_at IS NULL").Order("created_at").Limit(limit).Find(&out).Error
	if err != nil {
		return nil, fmt.Errorf("list unpublished: %w", err)
	}
	return out, nil
}

func (r *gormOutbox) MarkPublished(ctx context.Context, id uuid.UUID, at time.Time) error {
	return translate(r.db.WithContext(ctx).Model(&models.OutboxEvent{}).
		Where("id = ?", id).Update("published_at", at).Error)
}

func (r *gormOutbox) MarkFailed(ctx context.Context, id uuid.UUID, msg string) error {
	return translate(r.db.WithContext(ctx).Model(&models.OutboxEvent{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"attempts":   gorm.Expr("attempts + 1"),
			"last_error": msg,
		}).Error)
}
