package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/prasanth-33460/Project-Management-Platform/internal/api/middleware"
	"github.com/prasanth-33460/Project-Management-Platform/internal/apperror"
	"github.com/prasanth-33460/Project-Management-Platform/internal/models"
	"github.com/prasanth-33460/Project-Management-Platform/internal/service"
)

type SprintHandler struct{ svc *service.SprintService }

func NewSprintHandler(svc *service.SprintService) *SprintHandler { return &SprintHandler{svc: svc} }

// GET /api/sprints/:id
func (h *SprintHandler) GetByID(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperror.New(400, "invalid sprint id")
	}
	sprint, err := h.svc.GetByID(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(sprint)
}

// GET /api/projects/:id/sprints
func (h *SprintHandler) ListByProject(c *fiber.Ctx) error {
	projectID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperror.New(400, "invalid project id")
	}
	sprints, err := h.svc.ListByProject(c.Context(), projectID)
	if err != nil {
		return err
	}
	if sprints == nil {
		sprints = []*models.Sprint{}
	}
	return c.JSON(sprints)
}

// POST /api/projects/:id/sprints
func (h *SprintHandler) Create(c *fiber.Ctx) error {
	projectID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperror.New(400, "invalid project id")
	}
	var req models.CreateSprintRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.ErrBadRequest
	}
	if err := validateStruct(req); err != nil {
		return err
	}
	sprint, err := h.svc.Create(c.Context(), projectID, &req)
	if err != nil {
		return err
	}
	return c.Status(201).JSON(sprint)
}

// PATCH /api/sprints/:id
func (h *SprintHandler) Update(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperror.New(400, "invalid sprint id")
	}
	var req models.UpdateSprintRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.ErrBadRequest
	}
	sprint, err := h.svc.Update(c.Context(), id, &req)
	if err != nil {
		return err
	}
	return c.JSON(sprint)
}

// POST /api/sprints/:id/start
func (h *SprintHandler) Start(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperror.New(400, "invalid sprint id")
	}
	sprint, err := h.svc.Start(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(sprint)
}

// POST /api/sprints/:id/complete
func (h *SprintHandler) Complete(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperror.New(400, "invalid sprint id")
	}
	var req models.CompleteSprintRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.ErrBadRequest
	}
	userID := middleware.CurrentUser(c)
	result, err := h.svc.Complete(c.Context(), id, &req, userID)
	if err != nil {
		return err
	}
	return c.JSON(result)
}

// DELETE /api/sprints/:id
func (h *SprintHandler) Delete(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperror.New(400, "invalid sprint id")
	}
	if err := h.svc.Delete(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(204)
}

// POST /api/sprints/:id/move-issue
func (h *SprintHandler) MoveIssue(c *fiber.Ctx) error {
	sprintID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperror.New(400, "invalid sprint id")
	}
	var req models.MoveIssueRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.ErrBadRequest
	}
	if err := validateStruct(req); err != nil {
		return err
	}
	// Override sprint ID from URL
	req.SprintID = &sprintID
	userID := middleware.CurrentUser(c)
	if err := h.svc.MoveIssue(c.Context(), &req, userID); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"moved": true})
}

// POST /api/sprints/move-to-backlog  (sprint_id in body is nil)
func (h *SprintHandler) MoveToBacklog(c *fiber.Ctx) error {
	var req models.MoveIssueRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.ErrBadRequest
	}
	if err := validateStruct(req); err != nil {
		return err
	}
	req.SprintID = nil
	userID := middleware.CurrentUser(c)
	if err := h.svc.MoveIssue(c.Context(), &req, userID); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"moved": true})
}
