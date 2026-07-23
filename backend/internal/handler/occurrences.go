package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/PaulRychkov/task-planner/backend/internal/models"
)

func (h *Handler) listOccurrences(c *gin.Context) {
	var from, to *models.Date
	var status *models.OccurrenceStatus

	if s := c.Query("from"); s != "" {
		d, err := models.ParseDate(s)
		if err != nil {
			h.badRequest(c, "invalid from date, expected YYYY-MM-DD")
			return
		}
		from = &d
	}
	if s := c.Query("to"); s != "" {
		d, err := models.ParseDate(s)
		if err != nil {
			h.badRequest(c, "invalid to date, expected YYYY-MM-DD")
			return
		}
		to = &d
	}
	if s := c.Query("status"); s != "" {
		st := models.OccurrenceStatus(s)
		if !st.Valid() {
			h.badRequest(c, fmt.Sprintf("unknown status %q", s))
			return
		}
		status = &st
	}

	occs, err := h.occs.List(c.Request.Context(), from, to, status)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, occs)
}

func (h *Handler) completeOccurrence(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	occ, err := h.occs.Complete(c.Request.Context(), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, occ)
}

func (h *Handler) occurrenceProgress(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var in struct {
		Minutes int `json:"minutes"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		h.badRequest(c, "invalid json body: "+err.Error())
		return
	}
	occ, err := h.occs.AddProgress(c.Request.Context(), id, in.Minutes)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, occ)
}

func (h *Handler) skipOccurrence(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	occ, err := h.occs.Skip(c.Request.Context(), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, occ)
}
