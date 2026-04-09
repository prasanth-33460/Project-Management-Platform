package models

import (
	"time"

	"github.com/google/uuid"
)

type ActivityLog struct {
	ID        uuid.UUID     `json:"id"`
	IssueID   *uuid.UUID    `json:"issue_id,omitempty"`
	ProjectID uuid.UUID     `json:"project_id"`
	ActorID   uuid.UUID     `json:"actor_id"`
	Actor     *UserResponse `json:"actor,omitempty"`
	EventType string        `json:"event_type"`
	FieldName *string       `json:"field_name,omitempty"`
	OldValue  *string       `json:"old_value,omitempty"`
	NewValue  *string       `json:"new_value,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
}

// ActivityFeedResponse is a paginated activity feed.
type ActivityFeedResponse struct {
	Items      []*ActivityLog `json:"items"`
	NextCursor string         `json:"next_cursor,omitempty"`
	Total      int            `json:"total"`
}
