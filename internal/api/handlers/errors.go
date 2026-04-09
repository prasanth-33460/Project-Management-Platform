package handlers

import (
	"errors"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/prasanth-33460/Project-Management-Platform/internal/apperror"
)

// ErrorHandler is a centralized Fiber error handler.
func ErrorHandler(c *fiber.Ctx, err error) error {
	// AppError: domain errors with explicit HTTP codes
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return c.Status(appErr.Code).JSON(appErr)
	}

	// Validation errors from go-playground/validator
	var validationErrs validator.ValidationErrors
	if errors.As(err, &validationErrs) {
		fields := make(map[string]string, len(validationErrs))
		for _, fe := range validationErrs {
			fields[fe.Field()] = fe.Tag()
		}
		return c.Status(400).JSON(fiber.Map{
			"message": "validation failed",
			"fields":  fields,
		})
	}

	// Fiber built-in errors
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		return c.Status(fiberErr.Code).JSON(fiber.Map{"message": fiberErr.Message})
	}

	// Unexpected error — don't leak internals
	return c.Status(500).JSON(fiber.Map{"message": "internal server error"})
}
