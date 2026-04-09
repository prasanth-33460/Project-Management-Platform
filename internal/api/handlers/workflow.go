package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/prasanth-33460/Project-Management-Platform/internal/apperror"
	"github.com/prasanth-33460/Project-Management-Platform/internal/models"
	"github.com/prasanth-33460/Project-Management-Platform/internal/repository"
)

type WorkflowHandler struct{ repo repository.WorkflowStore }

func NewWorkflowHandler(repo repository.WorkflowStore) *WorkflowHandler {
	return &WorkflowHandler{repo: repo}
}

// GET /api/projects/:id/workflow/statuses
func (h *WorkflowHandler) ListStatuses(c *fiber.Ctx) error {
	projectID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperror.New(400, "invalid project id")
	}
	statuses, err := h.repo.ListStatuses(c.Context(), projectID)
	if err != nil {
		return err
	}
	if statuses == nil {
		statuses = []*models.WorkflowStatus{}
	}
	return c.JSON(statuses)
}

// POST /api/projects/:id/workflow/statuses
func (h *WorkflowHandler) CreateStatus(c *fiber.Ctx) error {
	projectID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperror.New(400, "invalid project id")
	}
	var req models.CreateStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.ErrBadRequest
	}
	if err := validateStruct(req); err != nil {
		return err
	}
	status, err := h.repo.CreateStatus(c.Context(), projectID, &req)
	if err != nil {
		return err
	}
	return c.Status(201).JSON(status)
}

// POST /api/projects/:id/workflow/transitions
func (h *WorkflowHandler) CreateTransition(c *fiber.Ctx) error {
	projectID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperror.New(400, "invalid project id")
	}
	var req models.CreateTransitionRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.ErrBadRequest
	}
	if err := validateStruct(req); err != nil {
		return err
	}
	transition, err := h.repo.CreateTransition(c.Context(), projectID, &req)
	if err != nil {
		return err
	}
	return c.Status(201).JSON(transition)
}

// DELETE /api/projects/:id/workflow/transitions/:transitionId
func (h *WorkflowHandler) DeleteTransition(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("transitionId"))
	if err != nil {
		return apperror.New(400, "invalid transition id")
	}
	if err := h.repo.DeleteTransition(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(204)
}
