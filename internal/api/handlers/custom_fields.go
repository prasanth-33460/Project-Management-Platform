package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/prasanth-33460/Project-Management-Platform/internal/apperror"
	"github.com/prasanth-33460/Project-Management-Platform/internal/models"
	"github.com/prasanth-33460/Project-Management-Platform/internal/service"
)

type CustomFieldHandler struct{ svc *service.CustomFieldService }

func NewCustomFieldHandler(svc *service.CustomFieldService) *CustomFieldHandler {
	return &CustomFieldHandler{svc: svc}
}

// GET /api/projects/:id/custom-fields
func (h *CustomFieldHandler) ListDefinitions(c *fiber.Ctx) error {
	projectID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperror.New(400, "invalid project id")
	}
	defs, err := h.svc.ListDefinitions(c.Context(), projectID)
	if err != nil {
		return err
	}
	return c.JSON(defs)
}

// POST /api/projects/:id/custom-fields
func (h *CustomFieldHandler) CreateDefinition(c *fiber.Ctx) error {
	projectID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperror.New(400, "invalid project id")
	}
	var req models.CreateCustomFieldRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.ErrBadRequest
	}
	if err := validateStruct(req); err != nil {
		return err
	}
	def, err := h.svc.CreateDefinition(c.Context(), projectID, &req)
	if err != nil {
		return err
	}
	return c.Status(201).JSON(def)
}

// DELETE /api/projects/:id/custom-fields/:fieldId
func (h *CustomFieldHandler) DeleteDefinition(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("fieldId"))
	if err != nil {
		return apperror.New(400, "invalid field id")
	}
	if err := h.svc.DeleteDefinition(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(204)
}

// GET /api/issues/:id/custom-fields
func (h *CustomFieldHandler) GetValues(c *fiber.Ctx) error {
	issueID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperror.New(400, "invalid issue id")
	}
	values, err := h.svc.GetValues(c.Context(), issueID)
	if err != nil {
		return err
	}
	return c.JSON(values)
}

// PUT /api/issues/:id/custom-fields
// Body: array of {field_id, value} — upserts each one
func (h *CustomFieldHandler) SetValues(c *fiber.Ctx) error {
	issueID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperror.New(400, "invalid issue id")
	}
	var reqs []models.SetCustomFieldValueRequest
	if err := c.BodyParser(&reqs); err != nil {
		return fiber.ErrBadRequest
	}
	if len(reqs) == 0 {
		return apperror.New(400, "request body must be a non-empty array")
	}
	values, err := h.svc.SetValues(c.Context(), issueID, reqs)
	if err != nil {
		return err
	}
	return c.JSON(values)
}
