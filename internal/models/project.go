package models

import (
	"time"

	"github.com/google/uuid"
)

type Project struct {
	ID           uuid.UUID     `json:"id"`
	Key          string        `json:"key"`
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	LeadUserID   *uuid.UUID    `json:"lead_user_id,omitempty"`
	Lead         *UserResponse `json:"lead,omitempty"`
	IssueCounter int           `json:"issue_counter"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

type ProjectMember struct {
	ProjectID uuid.UUID     `json:"project_id"`
	UserID    uuid.UUID     `json:"user_id"`
	User      *UserResponse `json:"user,omitempty"`
	Role      string        `json:"role"` // admin | member | viewer
	JoinedAt  time.Time     `json:"joined_at"`
}

// --- Request DTOs ---

type CreateProjectRequest struct {
	Key         string     `json:"key"         validate:"required,min=2,max=10,alphanum,uppercase"`
	Name        string     `json:"name"        validate:"required,min=1,max=100"`
	Description string     `json:"description"`
	LeadUserID  *uuid.UUID `json:"lead_user_id"`
}

type UpdateProjectRequest struct {
	Name        *string    `json:"name"        validate:"omitempty,min=1,max=100"`
	Description *string    `json:"description"`
	LeadUserID  *uuid.UUID `json:"lead_user_id"`
}
