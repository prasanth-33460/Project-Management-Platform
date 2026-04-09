package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/prasanth-33460/Project-Management-Platform/internal/models"
	"github.com/prasanth-33460/Project-Management-Platform/internal/service"
)

type SearchHandler struct{ svc *service.IssueService }

func NewSearchHandler(svc *service.IssueService) *SearchHandler { return &SearchHandler{svc: svc} }

// GET /api/search
// Query params: q, project_id, status_id, assignee_id, priority, type, cursor, limit
func (h *SearchHandler) Search(c *fiber.Ctx) error {
	filter := models.IssueFilter{
		Query:  c.Query("q"),
		Cursor: c.Query("cursor"),
		Limit:  c.QueryInt("limit", models.DefaultPageSize),
	}

	if pid := c.Query("project_id"); pid != "" {
		if id, err := uuid.Parse(pid); err == nil {
			filter.ProjectID = &id
		}
	}
	if sid := c.Query("status_id"); sid != "" {
		if id, err := uuid.Parse(sid); err == nil {
			filter.StatusID = &id
		}
	}
	if aid := c.Query("assignee_id"); aid != "" {
		if id, err := uuid.Parse(aid); err == nil {
			filter.AssigneeID = &id
		}
	}
	if p := c.Query("priority"); p != "" {
		prio := models.Priority(p)
		filter.Priority = &prio
	}
	if t := c.Query("type"); t != "" {
		it := models.IssueType(t)
		filter.Type = &it
	}

	result, err := h.svc.List(c.Context(), filter)
	if err != nil {
		return err
	}
	return c.JSON(result)
}
