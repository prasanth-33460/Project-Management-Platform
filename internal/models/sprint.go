package models

import (
	"github.com/google/uuid"
	"time"
)

type SprintStatus string

const (
	SprintStatusPlanned   SprintStatus = "planned"
	SprintStatusActive    SprintStatus = "active"
	SprintStatusCompleted SprintStatus = "completed"
)

type Sprint struct {
	ID        uuid.UUID    `json:"id"`
	ProjectID uuid.UUID    `json:"project_id"`
	Name      string       `json:"name"`
	Goal      string       `json:"goal"`
	StartDate *time.Time   `json:"start_date,omitempty"`
	EndDate   *time.Time   `json:"end_date,omitempty"`
	Status    SprintStatus `json:"status"`
	Velocity  *int         `json:"velocity,omitempty"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// SprintCompleteResult surfaces velocity and incomplete issues after closing a sprint.
type SprintCompleteResult struct {
	Sprint           *Sprint  `json:"sprint"`
	CompletedPoints  int      `json:"completed_points"`
	IncompleteIssues []*Issue `json:"incomplete_issues"`
}

type CreateSprintRequest struct {
	Name      string    `json:"name"       validate:"required,min=1,max=100"`
	Goal      string    `json:"goal"`
	StartDate *FlexDate `json:"start_date"`
	EndDate   *FlexDate `json:"end_date"`
}

type UpdateSprintRequest struct {
	Name      *string   `json:"name"       validate:"omitempty,min=1,max=100"`
	Goal      *string   `json:"goal"`
	StartDate *FlexDate `json:"start_date"`
	EndDate   *FlexDate `json:"end_date"`
}

type MoveIssueRequest struct {
	IssueID  uuid.UUID  `json:"issue_id"  validate:"required"`
	SprintID *uuid.UUID `json:"sprint_id"` // nil = move to backlog
}

type CompleteSprintRequest struct {
	CarryOverIssueIDs []uuid.UUID `json:"carry_over_issue_ids"` // move to next sprint
	NextSprintID      *uuid.UUID  `json:"next_sprint_id"`       // nil = backlog
}
