package models

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const DefaultPageSize = 20
const MaxPageSize = 100

// PagedResult wraps a paginated list response.
type PagedResult[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
	Total      int    `json:"total"`
}

// Cursor encodes/decodes a (created_at, id) cursor for stable pagination.
type Cursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

func EncodeCursor(createdAt time.Time, id uuid.UUID) string {
	raw := fmt.Sprintf("%s|%s", createdAt.UTC().Format(time.RFC3339Nano), id.String())
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

func DecodeCursor(encoded string) (*Cursor, error) {
	b, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	parts := strings.SplitN(string(b), "|", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid cursor format")
	}
	t, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return nil, err
	}
	return &Cursor{CreatedAt: t, ID: id}, nil
}
