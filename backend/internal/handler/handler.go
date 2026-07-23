package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/PaulRychkov/task-planner/backend/internal/models"
	"github.com/PaulRychkov/task-planner/backend/internal/service"
	"github.com/PaulRychkov/task-planner/backend/internal/syncer"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

type Handler struct {
	db        Pinger
	topics    *service.TopicService
	tasks     *service.TaskService
	occs      *service.OccurrenceService
	plans     *service.PlanService
	clock     service.Clock
	loc       *time.Location
	log       *zap.Logger
	sync      *syncer.Service
	syncToken string
}

func New(
	db Pinger,
	topics *service.TopicService,
	tasks *service.TaskService,
	occs *service.OccurrenceService,
	plans *service.PlanService,
	clock service.Clock,
	loc *time.Location,
	log *zap.Logger,
) *Handler {
	return &Handler{
		db:     db,
		topics: topics,
		tasks:  tasks,
		occs:   occs,
		plans:  plans,
		clock:  clock,
		loc:    loc,
		log:    log,
	}
}

func (h *Handler) WithSync(svc *syncer.Service, token string) *Handler {
	h.sync = svc
	h.syncToken = token
	return h
}

func (h *Handler) Router() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), cors())

	r.GET("/healthz", h.healthz)
	r.GET("/calendar.ics", h.calendarICS)

	api := r.Group("/api/v1")

	if h.sync != nil {
		sg := api.Group("/sync", h.syncAuth())
		sg.GET("/changes", h.syncChanges)
		sg.POST("/changes", h.syncApply)
	}

	api.GET("/topics", h.listTopics)
	api.POST("/topics", h.createTopic)
	api.GET("/topics/:id", h.getTopic)
	api.PUT("/topics/:id", h.updateTopic)
	api.DELETE("/topics/:id", h.deleteTopic)

	api.GET("/tasks", h.listTasks)
	api.POST("/tasks", h.createTask)
	api.GET("/tasks/:id", h.getTask)
	api.PUT("/tasks/:id", h.updateTask)
	api.DELETE("/tasks/:id", h.deleteTask)
	api.POST("/tasks/:id/reschedule-missed", h.rescheduleMissed)

	api.GET("/occurrences", h.listOccurrences)
	api.POST("/occurrences/:id/complete", h.completeOccurrence)
	api.POST("/occurrences/:id/skip", h.skipOccurrence)
	api.POST("/occurrences/:id/progress", h.occurrenceProgress)

	api.GET("/plans/:date", h.getPlan)
	api.PUT("/plans/:date/items", h.putPlanItems)
	api.POST("/plans/:date/commit", h.commitPlan)

	mcpHandler := h.MCPHandler()
	r.Any("/mcp", gin.WrapH(mcpHandler))

	return r
}

func cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Accept, Authorization, Mcp-Session-Id, Mcp-Protocol-Version")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func (h *Handler) healthz(c *gin.Context) {
	if err := h.db.Ping(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "down", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func errBody(code, message string) gin.H {
	return gin.H{"error": gin.H{"code": code, "message": message}}
}

func (h *Handler) fail(c *gin.Context, err error) {
	switch {
	case service.IsNotFound(err):
		c.JSON(http.StatusNotFound, errBody("not_found", err.Error()))
	case service.IsValidation(err):
		c.JSON(http.StatusBadRequest, errBody("validation", err.Error()))
	case service.IsConflict(err):
		c.JSON(http.StatusConflict, errBody("conflict", err.Error()))
	default:
		h.log.Error("request failed", zap.String("path", c.Request.URL.Path), zap.Error(err))
		c.JSON(http.StatusInternalServerError, errBody("internal", "internal error"))
	}
}

func (h *Handler) badRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, errBody("validation", msg))
}

func parseID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errBody("validation", "invalid uuid in path"))
		return uuid.Nil, false
	}
	return id, true
}

func parsePathDate(c *gin.Context) (models.Date, bool) {
	d, err := models.ParseDate(c.Param("date"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errBody("validation", "invalid date in path, expected YYYY-MM-DD"))
		return models.Date{}, false
	}
	return d, true
}
