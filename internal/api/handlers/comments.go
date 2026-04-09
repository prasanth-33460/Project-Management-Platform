package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/prasanth-33460/Project-Management-Platform/internal/api/middleware"
	"github.com/prasanth-33460/Project-Management-Platform/internal/apperror"
	"github.com/prasanth-33460/Project-Management-Platform/internal/models"
	"github.com/prasanth-33460/Project-Management-Platform/internal/service"
)

type CommentHandler struct{ svc *service.CollaborationService }

func NewCommentHandler(svc *service.CollaborationService) *CommentHandler {
	return &CommentHandler{svc: svc}
}

// GET /api/issues/:id/comments
func (h *CommentHandler) List(c *fiber.Ctx) error {
	issueID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperror.New(400, "invalid issue id")
	}
	comments, err := h.svc.ListComments(c.Context(), issueID)
	if err != nil {
		return err
	}
	if comments == nil {
		comments = []*models.Comment{}
	}
	return c.JSON(comments)
}

// POST /api/issues/:id/comments
func (h *CommentHandler) Create(c *fiber.Ctx) error {
	issueID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperror.New(400, "invalid issue id")
	}
	var req models.CreateCommentRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.ErrBadRequest
	}
	if err := validateStruct(req); err != nil {
		return err
	}
	userID := middleware.CurrentUser(c)
	comment, err := h.svc.AddComment(c.Context(), issueID, userID, &req)
	if err != nil {
		return err
	}
	return c.Status(201).JSON(comment)
}

// PATCH /api/comments/:id
func (h *CommentHandler) Update(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperror.New(400, "invalid comment id")
	}
	var req models.UpdateCommentRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.ErrBadRequest
	}
	if err := validateStruct(req); err != nil {
		return err
	}
	userID := middleware.CurrentUser(c)
	comment, err := h.svc.UpdateComment(c.Context(), id, userID, &req)
	if err != nil {
		return err
	}
	return c.JSON(comment)
}

// DELETE /api/comments/:id
func (h *CommentHandler) Delete(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return apperror.New(400, "invalid comment id")
	}
	userID := middleware.CurrentUser(c)
	if err := h.svc.DeleteComment(c.Context(), id, userID); err != nil {
		return err
	}
	return c.SendStatus(204)
}
