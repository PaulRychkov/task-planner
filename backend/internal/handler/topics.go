package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/PaulRychkov/task-planner/backend/internal/service"
)

func (h *Handler) listTopics(c *gin.Context) {
	includeArchived := c.Query("include_archived") == "true"
	topics, err := h.topics.List(c.Request.Context(), includeArchived)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, topics)
}

func (h *Handler) getTopic(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	topic, err := h.topics.Get(c.Request.Context(), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, topic)
}

func (h *Handler) createTopic(c *gin.Context) {
	var in service.TopicInput
	if err := c.ShouldBindJSON(&in); err != nil {
		h.badRequest(c, "invalid json body: "+err.Error())
		return
	}
	topic, err := h.topics.Create(c.Request.Context(), in)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, topic)
}

func (h *Handler) updateTopic(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var in service.TopicInput
	if err := c.ShouldBindJSON(&in); err != nil {
		h.badRequest(c, "invalid json body: "+err.Error())
		return
	}
	topic, err := h.topics.Update(c.Request.Context(), id, in)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, topic)
}

func (h *Handler) deleteTopic(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.topics.Delete(c.Request.Context(), id); err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
