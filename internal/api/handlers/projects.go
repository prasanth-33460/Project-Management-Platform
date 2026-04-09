package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/prasanth-33460/Project-Management-Platform/internal/api/middleware"
	"github.com/prasanth-33460/Project-Management-Platform/internal/apperror"
	"github.com/prasanth-33460/Project-Management-Platform/internal/models"
	"github.com/prasanth-33460/Project-Management-Platform/internal/service"
)

type ProjectHandler struct {
	svc      *service.ProjectService
	issueSvc *service.IssueService
}

func NewProjectHandler(svc *service.ProjectService, issueSvc *service.IssueService) *ProjectHandler {
	return &ProjectHandler{svc: svc, issueSvc: issueSvc}
}

// POST /api/projects
func (h *ProjectHandler) Create(c *fiber.Ctx) error {
	var req models.CreateProjectRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.ErrBadRequest
	}
	if err := validateStruct(req); err != nil {
		return err
	}
	userID := middleware.CurrentUser(c)
	project, err := h.svc.Create(c.Context(), &req, userID)
	if err != nil {
		return err
	}
	return c.Status(201).JSON(project)
}

// GET /api/projects
func (h *ProjectHandler) List(c *fiber.Ctx) error {
	userID := middleware.CurrentUser(c)
	projects, err := h.svc.List(c.Context(), userID)
	if err != nil {
		return err
	}
	if projects == nil {
		projects = []*models.Project{}
	}
	return c.JSON(projects)
}

// GET /api/projects/:id
func (h *ProjectHandler) Get(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperror.New(400, "invalid project id")
	}
	project, err := h.svc.Get(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(project)
}

// PATCH /api/projects/:id
func (h *ProjectHandler) Update(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperror.New(400, "invalid project id")
	}
	var req models.UpdateProjectRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.ErrBadRequest
	}
	project, err := h.svc.Update(c.Context(), id, &req)
	if err != nil {
		return err
	}
	return c.JSON(project)
}

// DELETE /api/projects/:id
func (h *ProjectHandler) Delete(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperror.New(400, "invalid project id")
	}
	if err := h.svc.Delete(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(204)
}

// GET /api/projects/:id/board
func (h *ProjectHandler) GetBoard(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperror.New(400, "invalid project id")
	}

	project, err := h.svc.Get(c.Context(), id)
	if err != nil {
		return err
	}

	filter := models.IssueFilter{
		ProjectID: &id,
		Limit:     200, // board shows all issues
	}
	result, err := h.issueSvc.List(c.Context(), filter)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"project": project,
		"issues":  result.Items,
		"total":   result.Total,
	})
}

// POST /api/projects/:id/issues
func (h *ProjectHandler) CreateIssue(c *fiber.Ctx) error {
	projectID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperror.New(400, "invalid project id")
	}
	var req models.CreateIssueRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.ErrBadRequest
	}
	if err := validateStruct(req); err != nil {
		return err
	}
	userID := middleware.CurrentUser(c)
	issue, err := h.issueSvc.Create(c.Context(), projectID, &req, userID)
	if err != nil {
		return err
	}
	return c.Status(201).JSON(issue)
}

// GET /api/projects/:id/activity
func (h *ProjectHandler) GetActivity(c *fiber.Ctx) error {
	projectID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperror.New(400, "invalid project id")
	}
	cursor := c.Query("cursor")
	limit := c.QueryInt("limit", 20)

	feed, err := h.issueSvc.GetActivityFeed(c.Context(), projectID, cursor, limit)
	if err != nil {
		return err
	}
	return c.JSON(feed)
}

// GET /api/projects/:id/backlog
func (h *ProjectHandler) GetBacklog(c *fiber.Ctx) error {
	projectID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperror.New(400, "invalid project id")
	}
	issues, err := h.issueSvc.GetBacklog(c.Context(), projectID)
	if err != nil {
		return err
	}
	if issues == nil {
		issues = []*models.Issue{}
	}
	return c.JSON(issues)
}
