package models

import (
	"time"

	"github.com/google/uuid"
)

type WorkflowStatus struct {
	ID        uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"project_id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	Position  int       `json:"position"`
	IsDefault bool      `json:"is_default"`
	IsDone    bool      `json:"is_done"`
	CreatedAt time.Time `json:"created_at"`
}

type AutoAction struct {
	Type    string            `json:"type"`    // assign_field | notify
	Field   string            `json:"field"`   // e.g. "assignee_id"
	Value   string            `json:"value"`   // static value or placeholder
	Payload map[string]string `json:"payload"` // extra context
}

type WorkflowTransition struct {
	ID           uuid.UUID       `json:"id"`
	ProjectID    uuid.UUID       `json:"project_id"`
	FromStatusID uuid.UUID       `json:"from_status_id"`
	ToStatusID   uuid.UUID       `json:"to_status_id"`
	ToStatus     *WorkflowStatus `json:"to_status,omitempty"`
	AutoActions  []AutoAction    `json:"auto_actions"`
}

type CreateStatusRequest struct {
	Name      string `json:"name"      validate:"required,min=1,max=50"`
	Color     string `json:"color"     validate:"omitempty,len=7"`
	Position  int    `json:"position"`
	IsDefault bool   `json:"is_default"`
	IsDone    bool   `json:"is_done"`
}

type CreateTransitionRequest struct {
	FromStatusID uuid.UUID    `json:"from_status_id" validate:"required"`
	ToStatusID   uuid.UUID    `json:"to_status_id"   validate:"required"`
	AutoActions  []AutoAction `json:"auto_actions"`
}

type TransitionRequest struct {
	TargetStatusID uuid.UUID `json:"target_status_id" validate:"required"`
}

// TransitionError is returned when a transition is not allowed from the current status.
type TransitionError struct {
	Message            string           `json:"message"`
	AllowedTransitions []WorkflowStatus `json:"allowed_transitions"`
}
