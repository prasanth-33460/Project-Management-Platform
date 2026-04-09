package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/prasanth-33460/Project-Management-Platform/internal/api/middleware"
	"github.com/prasanth-33460/Project-Management-Platform/internal/apperror"
	"github.com/prasanth-33460/Project-Management-Platform/internal/models"
	"github.com/prasanth-33460/Project-Management-Platform/internal/service"
)

type IssueHandler struct {
	svc      *service.IssueService
	workflow *service.WorkflowEngine
}

func NewIssueHandler(svc *service.IssueService, workflow *service.WorkflowEngine) *IssueHandler {
	return &IssueHandler{svc: svc, workflow: workflow}
}

// GET /api/issues/:id
func (h *IssueHandler) Get(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperror.New(400, "invalid issue id")
	}
	issue, err := h.svc.GetByID(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(issue)
}

// PATCH /api/issues/:id
func (h *IssueHandler) Update(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperror.New(400, "invalid issue id")
	}
	var req models.UpdateIssueRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.ErrBadRequest
	}
	userID := middleware.CurrentUser(c)
	issue, err := h.svc.Update(c.Context(), id, &req, userID)
	if err != nil {
		return err
	}
	return c.JSON(issue)
}

// DELETE /api/issues/:id
func (h *IssueHandler) Delete(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperror.New(400, "invalid issue id")
	}
	userID := middleware.CurrentUser(c)
	if err := h.svc.Delete(c.Context(), id, userID); err != nil {
		return err
	}
	return c.SendStatus(204)
}

// POST /api/issues/:id/transitions
// 422 if the target status isn't reachable from the current one; 409 on version conflict.
func (h *IssueHandler) Transition(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperror.New(400, "invalid issue id")
	}
	var req models.TransitionRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.ErrBadRequest
	}
	if err := validateStruct(req); err != nil {
		return err
	}
	userID := middleware.CurrentUser(c)
	issue, err := h.workflow.Transition(c.Context(), id, &req, userID)
	if err != nil {
		return err
	}
	return c.JSON(issue)
}

// POST /api/issues/:id/watch
func (h *IssueHandler) Watch(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperror.New(400, "invalid issue id")
	}
	userID := middleware.CurrentUser(c)
	if err := h.svc.Watch(c.Context(), id, userID); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"watched": true})
}

// DELETE /api/issues/:id/watch
func (h *IssueHandler) Unwatch(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperror.New(400, "invalid issue id")
	}
	userID := middleware.CurrentUser(c)
	if err := h.svc.Unwatch(c.Context(), id, userID); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"watched": false})
}
