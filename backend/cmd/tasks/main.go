package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ncruces/go-sqlite3/gormlite"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/golang-migrate/migrate/v4"
	migratepg "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/PaulRychkov/task-planner/backend/internal/config"
	"github.com/PaulRychkov/task-planner/backend/internal/handler"
	"github.com/PaulRychkov/task-planner/backend/internal/kafka"
	"github.com/PaulRychkov/task-planner/backend/internal/repository"
	"github.com/PaulRychkov/task-planner/backend/internal/service"
	"github.com/PaulRychkov/task-planner/backend/internal/sqlitemigrate"
	"github.com/PaulRychkov/task-planner/backend/internal/syncer"
	"github.com/PaulRychkov/task-planner/backend/migrations"
	migrationssqlite "github.com/PaulRychkov/task-planner/backend/migrations_sqlite"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log, err := zap.NewProduction()
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer func() { _ = log.Sync() }()

	var dialector gorm.Dialector
	if cfg.DB.IsSQLite() {
		dialector = gormlite.Open(cfg.DB.SQLiteDSN())
	} else {
		dialector = postgres.Open(cfg.DB.DSN())
	}
	db, err := gorm.Open(dialector, &gorm.Config{
		TranslateError: true,
		Logger:         gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc:        func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return fmt.Errorf("connect %s: %w", cfg.DB.Driver, err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("unwrap sql db: %w", err)
	}
	defer func() { _ = sqlDB.Close() }()

	if err := runMigrations(sqlDB, cfg.DB.IsSQLite()); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	log.Info("migrations applied")

	store := repository.NewGormStore(db)
	clock := service.NewClock()
	topics := service.NewTopicService(store)
	tasks := service.NewTaskService(store, clock, cfg.Location, cfg.WindowDays)
	occs := service.NewOccurrenceService(store, clock, cfg.Location)
	plans := service.NewPlanService(store, clock, cfg.Location)

	syncSvc := &syncer.Service{DB: db, Log: log}
	h := handler.New(store, topics, tasks, occs, plans, clock, cfg.Location, log).WithSync(syncSvc, cfg.SyncToken)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runGeneration(ctx, tasks, log)

	producer := kafka.NewProducer(cfg.KafkaBrokers, cfg.KafkaTopic, log)
	defer func() { _ = producer.Close() }()
	relay := service.NewOutboxRelay(store, producer, clock, log, cfg.OutboxInterval)
	go relay.Run(ctx)
	go runDailyJobs(ctx, tasks, cfg.Location, clock, log)
	if cfg.SyncURL != "" {
		go syncer.NewClient(syncSvc, cfg.SyncURL, cfg.SyncToken, cfg.SyncInterval, log).Run(ctx)
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler: h.Router(),
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Warn("http shutdown", zap.Error(err))
		}
	}()

	log.Info("tasks backend listening", zap.Int("port", cfg.HTTPPort))
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http server: %w", err)
	}
	log.Info("tasks backend stopped")
	return nil
}

func runMigrations(sqlDB *sql.DB, isSQLite bool) error {
	if isSQLite {
		return sqlitemigrate.Up(sqlDB, migrationssqlite.FS)
	}
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("open migrations fs: %w", err)
	}
	driver, err := migratepg.WithInstance(sqlDB, &migratepg.Config{})
	if err != nil {
		return fmt.Errorf("init migrate driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		return fmt.Errorf("init migrate: %w", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

func runGeneration(ctx context.Context, tasks *service.TaskService, log *zap.Logger) {
	if n, err := tasks.MarkMissed(ctx); err != nil {
		log.Warn("mark missed failed", zap.Error(err))
	} else if n > 0 {
		log.Info("occurrences marked missed", zap.Int("count", n))
	}
	if err := tasks.GenerateAll(ctx); err != nil {
		log.Warn("occurrence generation failed", zap.Error(err))
	} else {
		log.Info("occurrence window generated")
	}
}

func runDailyJobs(ctx context.Context, tasks *service.TaskService, loc *time.Location, clock service.Clock, log *zap.Logger) {
	for {
		now := clock.Now().In(loc)
		next := time.Date(now.Year(), now.Month(), now.Day(), 0, 5, 0, 0, loc).AddDate(0, 0, 1)
		timer := time.NewTimer(next.Sub(now))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			log.Info("running nightly jobs")
			runGeneration(ctx, tasks, log)
		}
	}
}
