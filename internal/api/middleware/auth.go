package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/prasanth-33460/Project-Management-Platform/internal/apperror"
	"github.com/prasanth-33460/Project-Management-Platform/internal/service"
)

const UserIDKey = "userID"

// Auth validates the JWT and stores the user ID in Fiber locals.
func Auth(authSvc *service.AuthService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := c.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			return apperror.ErrUnauthorized
		}
		userID, err := authSvc.ValidateToken(header[7:])
		if err != nil {
			return err
		}
		c.Locals(UserIDKey, userID)
		return c.Next()
	}
}

// CurrentUser retrieves the authenticated user's UUID from Fiber locals.
func CurrentUser(c *fiber.Ctx) uuid.UUID {
	id, _ := c.Locals(UserIDKey).(uuid.UUID)
	return id
}
