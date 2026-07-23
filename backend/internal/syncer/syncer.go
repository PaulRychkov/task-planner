package syncer

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/PaulRychkov/task-planner/backend/internal/models"
)

type Changes struct {
	ServerTime  time.Time               `json:"server_time"`
	Topics      []models.Topic          `json:"topics"`
	Tasks       []models.Task           `json:"tasks"`
	Occurrences []models.TaskOccurrence `json:"occurrences"`
	Tombstones  []models.SyncTombstone  `json:"tombstones"`
}

type ApplyResult struct {
	Upserted int `json:"upserted"`
	Deleted  int `json:"deleted"`
	Skipped  int `json:"skipped"`
}

type Service struct {
	DB  *gorm.DB
	Log *zap.Logger
}

func (s *Service) Collect(ctx context.Context, since time.Time) (Changes, error) {
	out := Changes{ServerTime: time.Now().UTC()}
	db := s.DB.WithContext(ctx)
	if err := db.Where("updated_at > ?", since).Order("updated_at").Find(&out.Topics).Error; err != nil {
		return out, fmt.Errorf("collect topics: %w", err)
	}
	if err := db.Where("updated_at > ?", since).Order("updated_at").Find(&out.Tasks).Error; err != nil {
		return out, fmt.Errorf("collect tasks: %w", err)
	}
	if err := db.Where("updated_at > ?", since).Order("updated_at").Find(&out.Occurrences).Error; err != nil {
		return out, fmt.Errorf("collect occurrences: %w", err)
	}
	if err := db.Where("deleted_at > ?", since).Order("deleted_at").Find(&out.Tombstones).Error; err != nil {
		return out, fmt.Errorf("collect tombstones: %w", err)
	}
	return out, nil
}

func (s *Service) Apply(ctx context.Context, in Changes, recordTombstones bool) (ApplyResult, error) {
	var res ApplyResult
	db := s.DB.WithContext(ctx)
	for i := range in.Topics {
		s.applyTopic(db, &in.Topics[i], &res)
	}
	for i := range in.Tasks {
		s.applyTask(db, &in.Tasks[i], &res)
	}
	for i := range in.Occurrences {
		s.applyOccurrence(db, &in.Occurrences[i], &res)
	}
	for i := range in.Tombstones {
		s.applyTombstone(db, in.Tombstones[i], recordTombstones, &res)
	}
	return res, nil
}

func (s *Service) applyTopic(db *gorm.DB, in *models.Topic, res *ApplyResult) {
	var existing models.Topic
	err := db.Where("id = ?", in.ID).First(&existing).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		if createErr := db.Session(&gorm.Session{SkipHooks: true}).Create(in).Error; createErr != nil {
			var byName models.Topic
			if db.Where("name = ?", in.Name).First(&byName).Error == nil && in.UpdatedAt.After(byName.UpdatedAt) {
				s.updateAll(db, &models.Topic{}, byName.ID, in, res)
				return
			}
			s.Log.Warn("sync: топик не применён", zap.String("name", in.Name), zap.Error(createErr))
			res.Skipped++
			return
		}
		res.Upserted++
	case err != nil:
		res.Skipped++
	case in.UpdatedAt.After(existing.UpdatedAt):
		s.updateAll(db, &models.Topic{}, in.ID, in, res)
	default:
		res.Skipped++
	}
}

func (s *Service) applyTask(db *gorm.DB, in *models.Task, res *ApplyResult) {
	in.Topic = nil
	var existing models.Task
	err := db.Where("id = ?", in.ID).First(&existing).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		if createErr := db.Session(&gorm.Session{SkipHooks: true}).Omit("Topic").Create(in).Error; createErr != nil {
			s.Log.Warn("sync: задача не применена", zap.String("title", in.Title), zap.Error(createErr))
			res.Skipped++
			return
		}
		res.Upserted++
	case err != nil:
		res.Skipped++
	case in.UpdatedAt.After(existing.UpdatedAt):
		s.updateAll(db, &models.Task{}, in.ID, in, res)
	default:
		res.Skipped++
	}
}

func (s *Service) applyOccurrence(db *gorm.DB, in *models.TaskOccurrence, res *ApplyResult) {
	in.Task = nil
	var existing models.TaskOccurrence
	err := db.Where("id = ?", in.ID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = db.Where("task_id = ? AND date = ?", in.TaskID, in.Date).First(&existing).Error
		if err == nil {
			if in.UpdatedAt.After(existing.UpdatedAt) {
				s.updateAllExceptID(db, &models.TaskOccurrence{}, existing.ID, in, res)
			} else {
				res.Skipped++
			}
			return
		}
		if createErr := db.Session(&gorm.Session{SkipHooks: true}).Omit("Task").Create(in).Error; createErr != nil {
			s.Log.Warn("sync: вхождение не применено", zap.String("id", in.ID.String()), zap.Error(createErr))
			res.Skipped++
			return
		}
		res.Upserted++
		return
	}
	if err != nil {
		res.Skipped++
		return
	}
	if in.UpdatedAt.After(existing.UpdatedAt) {
		s.updateAll(db, &models.TaskOccurrence{}, in.ID, in, res)
	} else {
		res.Skipped++
	}
}

func (s *Service) applyTombstone(db *gorm.DB, ts models.SyncTombstone, record bool, res *ApplyResult) {
	var model any
	switch ts.Table {
	case "topics":
		model = &models.Topic{}
	case "tasks":
		model = &models.Task{}
	case "task_occurrences":
		model = &models.TaskOccurrence{}
	default:
		res.Skipped++
		return
	}
	q := db.Where("id = ? AND updated_at <= ?", ts.RowID, ts.DeletedAt).Delete(model)
	if q.Error != nil {
		s.Log.Warn("sync: удаление не применено", zap.String("table", ts.Table), zap.Error(q.Error))
		res.Skipped++
		return
	}
	if q.RowsAffected > 0 {
		res.Deleted++
	}
	if record {
		if err := db.Create(&models.SyncTombstone{Table: ts.Table, RowID: ts.RowID, DeletedAt: ts.DeletedAt}).Error; err != nil {
			s.Log.Warn("sync: томбстоун не записан", zap.Error(err))
		}
	}
}

func (s *Service) updateAll(db *gorm.DB, model any, id uuid.UUID, in any, res *ApplyResult) {
	err := db.Model(model).Where("id = ?", id).
		Session(&gorm.Session{SkipHooks: true}).
		Select("*").Omit("id", "created_at").UpdateColumns(in).Error
	if err != nil {
		s.Log.Warn("sync: обновление не применено", zap.String("id", id.String()), zap.Error(err))
		res.Skipped++
		return
	}
	res.Upserted++
}

func (s *Service) updateAllExceptID(db *gorm.DB, model any, keepID uuid.UUID, in any, res *ApplyResult) {
	s.updateAll(db, model, keepID, in, res)
}

func (s *Service) GetState(ctx context.Context, key string) time.Time {
	var st models.SyncState
	if err := s.DB.WithContext(ctx).Where("key = ?", key).First(&st).Error; err != nil {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, st.Value)
	if err != nil {
		return time.Time{}
	}
	return t
}

func (s *Service) SetState(ctx context.Context, key string, t time.Time) {
	st := models.SyncState{Key: key, Value: t.UTC().Format(time.RFC3339Nano)}
	err := s.DB.WithContext(ctx).Save(&st).Error
	if err != nil {
		s.Log.Warn("sync: курсор не сохранён", zap.String("key", key), zap.Error(err))
	}
}
