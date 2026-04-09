package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/prasanth-33460/Project-Management-Platform/internal/api/middleware"
	"github.com/prasanth-33460/Project-Management-Platform/internal/apperror"
	"github.com/prasanth-33460/Project-Management-Platform/internal/models"
	"github.com/prasanth-33460/Project-Management-Platform/internal/service"
)

type NotificationHandler struct{ svc *service.NotificationService }

func NewNotificationHandler(svc *service.NotificationService) *NotificationHandler {
	return &NotificationHandler{svc: svc}
}

// GET /api/notifications
func (h *NotificationHandler) List(c *fiber.Ctx) error {
	userID := middleware.CurrentUser(c)
	cursor := c.Query("cursor")
	limit := c.QueryInt("limit", models.DefaultPageSize)

	result, err := h.svc.List(c.Context(), userID, cursor, limit)
	if err != nil {
		return err
	}
	return c.JSON(result)
}

// POST /api/notifications/:id/read
func (h *NotificationHandler) MarkRead(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperror.New(400, "invalid notification id")
	}
	userID := middleware.CurrentUser(c)
	if err := h.svc.MarkRead(c.Context(), id, userID); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"read": true})
}

// POST /api/notifications/read-all
func (h *NotificationHandler) MarkAllRead(c *fiber.Ctx) error {
	userID := middleware.CurrentUser(c)
	if err := h.svc.MarkAllRead(c.Context(), userID); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"read": true})
}
