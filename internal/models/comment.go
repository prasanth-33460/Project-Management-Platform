package models

import (
	"time"

	"github.com/google/uuid"
)

type Comment struct {
	ID        uuid.UUID     `json:"id"`
	IssueID   uuid.UUID     `json:"issue_id"`
	AuthorID  uuid.UUID     `json:"author_id"`
	Author    *UserResponse `json:"author,omitempty"`
	Body      string        `json:"body"`
	ParentID  *uuid.UUID    `json:"parent_id,omitempty"`
	Replies   []*Comment    `json:"replies,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// --- Request DTOs ---

type CreateCommentRequest struct {
	Body     string     `json:"body"      validate:"required,min=1"`
	ParentID *uuid.UUID `json:"parent_id"`
}

type UpdateCommentRequest struct {
	Body string `json:"body" validate:"required,min=1"`
}
