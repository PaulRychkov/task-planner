package repository_test

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/PaulRychkov/task-planner/backend/internal/config"
	"github.com/PaulRychkov/task-planner/backend/internal/models"
	"github.com/PaulRychkov/task-planner/backend/internal/repository"
	"github.com/PaulRychkov/task-planner/backend/internal/service"
	"github.com/PaulRychkov/task-planner/backend/internal/sqlitemigrate"
	migrationssqlite "github.com/PaulRychkov/task-planner/backend/migrations_sqlite"
)

type sqliteEnv struct {
	store repository.Store
	tasks *service.TaskService
	occs  *service.OccurrenceService
}

func newSQLite(t *testing.T) *sqliteEnv {
	t.Helper()
	cfg := config.DBConfig{Driver: "sqlite", Path: filepath.Join(t.TempDir(), "tasks.db")}
	db, err := gorm.Open(gormlite.Open(cfg.SQLiteDSN()), &gorm.Config{
		TranslateError: true,
		Logger:         gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc:        func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("unwrap: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := sqlitemigrate.Up(sqlDB, migrationssqlite.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	store := repository.NewGormStore(db)
	clock := service.FixedClock{T: time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)}
	return &sqliteEnv{
		store: store,
		tasks: service.NewTaskService(store, clock, time.UTC, 60),
		occs:  service.NewOccurrenceService(store, clock, time.UTC),
	}
}

func boolPtr(b bool) *bool { return &b }

func TestSQLiteRequiresPomodoroPersisted(t *testing.T) {
	env := newSQLite(t)
	ctx := context.Background()
	cases := []struct {
		name string
		in   *bool
		want bool
	}{
		{"поле не передано — дефолт true", nil, true},
		{"явный false сохраняется", boolPtr(false), false},
		{"явный true", boolPtr(true), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			created, err := env.tasks.Create(ctx, service.TaskInput{
				Title:            tc.name,
				RecurrenceKind:   models.RecurrenceOnce,
				RequiresPomodoro: tc.in,
			})
			if err != nil {
				t.Fatalf("create: %v", err)
			}
			if created.RequiresPomodoro != tc.want {
				t.Errorf("ответ Create: requires_pomodoro=%v, want %v", created.RequiresPomodoro, tc.want)
			}
			loaded, err := env.tasks.Get(ctx, created.ID)
			if err != nil {
				t.Fatalf("get: %v", err)
			}
			if loaded.RequiresPomodoro != tc.want {
				t.Errorf("в БД: requires_pomodoro=%v, want %v", loaded.RequiresPomodoro, tc.want)
			}
		})
	}
}

func TestSQLiteConcurrentWritesDoNotFailWithBusy(t *testing.T) {
	env := newSQLite(t)
	ctx := context.Background()
	const n = 24
	var occIDs []models.TaskOccurrence
	for i := 0; i < n; i++ {
		task, err := env.tasks.Create(ctx, service.TaskInput{
			Title:            "sr",
			RecurrenceKind:   models.RecurrenceSpacedRepetition,
			RecurrenceParams: &models.RecurrenceParams{Intervals: []int{0, 1, 3}},
		})
		if err != nil {
			t.Fatalf("seed: %v", err)
		}
		occs, err := env.store.Occurrences().List(ctx, repository.OccurrenceFilter{TaskID: &task.ID})
		if err != nil || len(occs) == 0 {
			t.Fatalf("seed occurrences: %v", err)
		}
		occIDs = append(occIDs, occs[0])
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2*n)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			<-start
			if _, err := env.occs.Complete(ctx, occIDs[i].ID); err != nil {
				errs <- err
			}
		}(i)
		go func() {
			defer wg.Done()
			<-start
			if _, err := env.tasks.Create(ctx, service.TaskInput{Title: "parallel", RecurrenceKind: models.RecurrenceDaily}); err != nil {
				errs <- err
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	var failed []error
	for err := range errs {
		failed = append(failed, err)
	}
	if len(failed) > 0 {
		t.Fatalf("%d из %d параллельных записей упали, первая: %v", len(failed), 2*n, failed[0])
	}
}
