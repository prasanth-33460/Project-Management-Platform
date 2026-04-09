package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	DisplayName  string    `json:"display_name"`
	Handle       *string   `json:"handle,omitempty"` // @mention handle e.g. "jane_smith"
	AvatarURL    *string   `json:"avatar_url,omitempty"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// UserResponse is a safe public representation (no password hash).
type UserResponse struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Handle      *string   `json:"handle,omitempty"`
	AvatarURL   *string   `json:"avatar_url,omitempty"`
}

func (u *User) ToResponse() *UserResponse {
	return &UserResponse{
		ID:          u.ID,
		Email:       u.Email,
		DisplayName: u.DisplayName,
		Handle:      u.Handle,
		AvatarURL:   u.AvatarURL,
	}
}

// --- Request DTOs ---

type RegisterRequest struct {
	Email       string `json:"email"        validate:"required,email"`
	DisplayName string `json:"display_name" validate:"required,min=2,max=100"`
	Password    string `json:"password"     validate:"required,min=8"`
}

type LoginRequest struct {
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type AuthResponse struct {
	Token string        `json:"token"`
	User  *UserResponse `json:"user"`
}
