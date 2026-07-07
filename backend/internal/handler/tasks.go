package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/PaulRychkov/task-planner/backend/internal/service"
)

func (h *Handler) listTasks(c *gin.Context) {
	tasks, err := h.tasks.List(c.Request.Context())
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, tasks)
}

func (h *Handler) getTask(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	task, err := h.tasks.Get(c.Request.Context(), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, task)
}

func (h *Handler) createTask(c *gin.Context) {
	var in service.TaskInput
	if err := c.ShouldBindJSON(&in); err != nil {
		h.badRequest(c, "invalid json body: "+err.Error())
		return
	}
	task, err := h.tasks.Create(c.Request.Context(), in)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, task)
}

func (h *Handler) updateTask(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var in service.TaskInput
	if err := c.ShouldBindJSON(&in); err != nil {
		h.badRequest(c, "invalid json body: "+err.Error())
		return
	}
	task, err := h.tasks.Update(c.Request.Context(), id, in)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, task)
}

func (h *Handler) deleteTask(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.tasks.Delete(c.Request.Context(), id); err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) rescheduleMissed(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	result, err := h.tasks.RescheduleMissed(c.Request.Context(), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}
