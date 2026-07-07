package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/PaulRychkov/task-planner/backend/internal/models"
	"github.com/PaulRychkov/task-planner/backend/internal/service"
)

type planItemsRequest struct {
	Items []service.PlanItemInput `json:"items"`
}

type commitPlanRequest struct {
	CommittedBy models.ActorKind `json:"committed_by"`
}

func (h *Handler) getPlan(c *gin.Context) {
	date, ok := parsePathDate(c)
	if !ok {
		return
	}
	plan, err := h.plans.Get(c.Request.Context(), date)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, plan)
}

func (h *Handler) putPlanItems(c *gin.Context) {
	date, ok := parsePathDate(c)
	if !ok {
		return
	}
	var req planItemsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.badRequest(c, "invalid json body: "+err.Error())
		return
	}
	plan, err := h.plans.PutItems(c.Request.Context(), date, req.Items)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, plan)
}

func (h *Handler) commitPlan(c *gin.Context) {
	date, ok := parsePathDate(c)
	if !ok {
		return
	}
	var req commitPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.badRequest(c, "invalid json body: "+err.Error())
		return
	}
	plan, err := h.plans.Commit(c.Request.Context(), date, req.CommittedBy)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, plan)
}
