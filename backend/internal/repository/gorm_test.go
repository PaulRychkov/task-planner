package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"

	"github.com/PaulRychkov/task-planner/backend/internal/models"
	"github.com/PaulRychkov/task-planner/backend/internal/sqlitemigrate"
	migrationssqlite "github.com/PaulRychkov/task-planner/backend/migrations_sqlite"
)

func newSQLiteStore(t *testing.T) Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tasks.db")
	db, err := gorm.Open(gormlite.Open("file:"+path), &gorm.Config{
		TranslateError: true,
		NowFunc:        func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		t.Fatalf("открыть sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("достать sql.DB: %v", err)
	}
	if err := sqlitemigrate.Up(sqlDB, migrationssqlite.FS); err != nil {
		t.Fatalf("миграции: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return NewGormStore(db)
}

func newTask(title string, requiresPomodoro bool) *models.Task {
	return &models.Task{
		Title:          title,
		RecurrenceKind: models.RecurrenceDaily,
		StartDate:      models.NewDate(2026, time.August, 1),
		Priority:       models.MinPriority,
		Progress:       models.ProgressNeedsAction,
		IsActive:       true,

		RequiresPomodoro: requiresPomodoro,
	}
}

func TestCreateKeepsRequiresPomodoroFalse(t *testing.T) {
	store := newSQLiteStore(t)
	ctx := context.Background()

	task := newTask("Зал", false)
	if err := store.Tasks().Create(ctx, task); err != nil {
		t.Fatalf("создать задачу: %v", err)
	}
	stored, err := store.Tasks().Get(ctx, task.ID)
	if err != nil {
		t.Fatalf("прочитать задачу: %v", err)
	}
	if stored.RequiresPomodoro {
		t.Fatalf("флаг «не требует помидоров» потерян при создании: в базе %v", stored.RequiresPomodoro)
	}
}

func TestCreateKeepsRequiresPomodoroTrue(t *testing.T) {
	store := newSQLiteStore(t)
	ctx := context.Background()

	task := newTask("Поиск работы", true)
	if err := store.Tasks().Create(ctx, task); err != nil {
		t.Fatalf("создать задачу: %v", err)
	}
	stored, err := store.Tasks().Get(ctx, task.ID)
	if err != nil {
		t.Fatalf("прочитать задачу: %v", err)
	}
	if !stored.RequiresPomodoro {
		t.Fatalf("обычная задача обязана требовать помидоры")
	}
}

func TestUpdateTogglesRequiresPomodoroBothWays(t *testing.T) {
	store := newSQLiteStore(t)
	ctx := context.Background()

	task := newTask("Дорога из зала", true)
	if err := store.Tasks().Create(ctx, task); err != nil {
		t.Fatalf("создать задачу: %v", err)
	}
	task.RequiresPomodoro = false
	if err := store.Tasks().Update(ctx, task); err != nil {
		t.Fatalf("снять флаг: %v", err)
	}
	stored, _ := store.Tasks().Get(ctx, task.ID)
	if stored.RequiresPomodoro {
		t.Fatalf("снятие флага не доехало до базы")
	}
	stored.RequiresPomodoro = true
	if err := store.Tasks().Update(ctx, stored); err != nil {
		t.Fatalf("вернуть флаг: %v", err)
	}
	back, _ := store.Tasks().Get(ctx, task.ID)
	if !back.RequiresPomodoro {
		t.Fatalf("возврат флага не доехал до базы")
	}
}
