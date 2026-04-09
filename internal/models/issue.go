package models

import (
	"time"

	"github.com/google/uuid"
)

type IssueType string

const (
	IssueTypeEpic    IssueType = "epic"
	IssueTypeStory   IssueType = "story"
	IssueTypeTask    IssueType = "task"
	IssueTypeBug     IssueType = "bug"
	IssueTypeSubtask IssueType = "subtask"
)

type Priority string

const (
	PriorityLow      Priority = "low"
	PriorityMedium   Priority = "medium"
	PriorityHigh     Priority = "high"
	PriorityCritical Priority = "critical"
)

type Issue struct {
	ID          uuid.UUID       `json:"id"`
	ProjectID   uuid.UUID       `json:"project_id"`
	IssueKey    string          `json:"issue_key"`
	SprintID    *uuid.UUID      `json:"sprint_id,omitempty"`
	Sprint      *Sprint         `json:"sprint,omitempty"`
	ParentID    *uuid.UUID      `json:"parent_id,omitempty"`
	Type        IssueType       `json:"type"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	StatusID    uuid.UUID       `json:"status_id"`
	Status      *WorkflowStatus `json:"status,omitempty"`
	Priority    Priority        `json:"priority"`
	AssigneeID  *uuid.UUID      `json:"assignee_id,omitempty"`
	Assignee    *UserResponse   `json:"assignee,omitempty"`
	ReporterID  uuid.UUID       `json:"reporter_id"`
	Reporter    *UserResponse   `json:"reporter,omitempty"`
	StoryPoints *int            `json:"story_points,omitempty"`
	Labels      []string        `json:"labels"`
	Version     int             `json:"version"`
	Watchers    []uuid.UUID     `json:"watchers,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// BoardColumn groups issues by status for the board view.
type BoardColumn struct {
	Status *WorkflowStatus `json:"status"`
	Issues []*Issue        `json:"issues"`
}

type BoardState struct {
	Project *Project       `json:"project"`
	Sprint  *Sprint        `json:"sprint,omitempty"`
	Columns []*BoardColumn `json:"columns"`
}

type CreateIssueRequest struct {
	Type        IssueType  `json:"type"        validate:"required,oneof=epic story task bug subtask"`
	Title       string     `json:"title"       validate:"required,min=1,max=500"`
	Description string     `json:"description"`
	Priority    Priority   `json:"priority"    validate:"omitempty,oneof=low medium high critical"`
	AssigneeID  *uuid.UUID `json:"assignee_id"`
	SprintID    *uuid.UUID `json:"sprint_id"`
	ParentID    *uuid.UUID `json:"parent_id"`
	StoryPoints *int       `json:"story_points" validate:"omitempty,min=0"`
	Labels      []string   `json:"labels"`
	StatusID    *uuid.UUID `json:"status_id"` // optional; defaults to project's default status
}

type UpdateIssueRequest struct {
	Title       *string    `json:"title"        validate:"omitempty,min=1,max=500"`
	Description *string    `json:"description"`
	Priority    *Priority  `json:"priority"     validate:"omitempty,oneof=low medium high critical"`
	AssigneeID  *uuid.UUID `json:"assignee_id"`
	SprintID    *uuid.UUID `json:"sprint_id"`
	StoryPoints *int       `json:"story_points" validate:"omitempty,min=0"`
	Labels      []string   `json:"labels"`
}

// IssueFilter holds structured filter params for the list/search endpoint.
type IssueFilter struct {
	ProjectID  *uuid.UUID
	SprintID   *uuid.UUID
	StatusID   *uuid.UUID
	AssigneeID *uuid.UUID
	Priority   *Priority
	Type       *IssueType
	Query      string // full-text search
	Cursor     string // base64-encoded (created_at,id) cursor
	Limit      int
}
