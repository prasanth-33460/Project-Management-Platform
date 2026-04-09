package models

import (
	"time"

	"github.com/google/uuid"
)

type Notification struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"user_id"`
	Type      string     `json:"type"` // assigned | mentioned | status_changed | comment_added
	RefID     *uuid.UUID `json:"ref_id,omitempty"`
	RefType   *string    `json:"ref_type,omitempty"` // issue | comment
	Title     string     `json:"title"`
	Body      *string    `json:"body,omitempty"`
	Read      bool       `json:"read"`
	CreatedAt time.Time  `json:"created_at"`
}

type NotificationListResponse struct {
	Items      []*Notification `json:"items"`
	NextCursor string          `json:"next_cursor,omitempty"`
	UnreadCount int            `json:"unread_count"`
}
