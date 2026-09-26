package mobile

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"
	_ "time/tzdata"

	"github.com/ncruces/go-sqlite3/gormlite"
	_ "github.com/ncruces/go-sqlite3/embed"
	"go.uber.org/zap"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/PaulRychkov/task-planner/backend/internal/handler"
	"github.com/PaulRychkov/task-planner/backend/internal/repository"
	"github.com/PaulRychkov/task-planner/backend/internal/service"
	"github.com/PaulRychkov/task-planner/backend/internal/sqlitemigrate"
	"github.com/PaulRychkov/task-planner/backend/internal/syncer"
	migrationssqlite "github.com/PaulRychkov/task-planner/backend/migrations_sqlite"
)

//go:embed all:webdist
var webFS embed.FS

const Addr = "127.0.0.1:18081"

var (
	mu     sync.Mutex
	srv    *http.Server
	cancel context.CancelFunc
)

func Start(dataDir, syncURL, syncToken string) string {
	mu.Lock()
	defer mu.Unlock()
	if srv != nil {
		return ""
	}
	log, err := zap.NewProduction()
	if err != nil {
		return "logger: " + err.Error()
	}
	dbPath := filepath.Join(dataDir, "tasks.db")
	db, err := gorm.Open(gormlite.Open("file:"+strings.ReplaceAll(dbPath, "\\", "/")+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_txlock=immediate"), &gorm.Config{
		TranslateError: true,
		Logger:         gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc:        func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return "open db: " + err.Error()
	}
	sqlDB, err := db.DB()
	if err != nil {
		return "unwrap db: " + err.Error()
	}
	if err := sqlitemigrate.Up(sqlDB, migrationssqlite.FS); err != nil {
		return "migrate: " + err.Error()
	}

	loc := time.Local
	store := repository.NewGormStore(db)
	clock := service.NewClock()
	topics := service.NewTopicService(store)
	tasks := service.NewTaskService(store, clock, loc, 60)
	occs := service.NewOccurrenceService(store, clock, loc)
	plans := service.NewPlanService(store, clock, loc)
	syncSvc := &syncer.Service{DB: db, Log: log}

	static, err := fs.Sub(webFS, "webdist")
	if err != nil {
		return "webdist: " + err.Error()
	}
	h := handler.New(store, topics, tasks, occs, plans, clock, loc, log).
		WithSync(syncSvc, syncToken).
		WithStatic(static)

	ctx, stop := context.WithCancel(context.Background())
	cancel = stop

	if _, err := tasks.MarkMissed(ctx); err != nil {
		log.Warn("mark missed", zap.Error(err))
	}
	if err := tasks.GenerateAll(ctx); err != nil {
		log.Warn("generate occurrences", zap.Error(err))
	}
	go runDailyJobs(ctx, tasks, loc, clock, log)
	if syncURL != "" {
		go syncer.NewClient(syncSvc, syncURL, syncToken, 60*time.Second, log).Run(ctx)
	}

	srv = &http.Server{Addr: Addr, Handler: h.Router()}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("mobile http server", zap.Error(err))
		}
	}()
	return ""
}

func Stop() {
	mu.Lock()
	defer mu.Unlock()
	if srv == nil {
		return
	}
	if cancel != nil {
		cancel()
	}
	shutdownCtx, done := context.WithTimeout(context.Background(), 3*time.Second)
	defer done()
	_ = srv.Shutdown(shutdownCtx)
	srv = nil
}

func BaseURL() string {
	return fmt.Sprintf("http://%s/", Addr)
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
			if _, err := tasks.MarkMissed(ctx); err != nil {
				log.Warn("mark missed", zap.Error(err))
			}
			if err := tasks.GenerateAll(ctx); err != nil {
				log.Warn("generate occurrences", zap.Error(err))
			}
		}
	}
}
