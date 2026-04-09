package models

import (
	"time"

	"github.com/google/uuid"
)

type FieldType string

const (
	FieldTypeText     FieldType = "text"
	FieldTypeNumber   FieldType = "number"
	FieldTypeDropdown FieldType = "dropdown"
	FieldTypeDate     FieldType = "date"
)

// CustomFieldDefinition describes a per-project custom field schema.
type CustomFieldDefinition struct {
	ID        uuid.UUID  `json:"id"`
	ProjectID uuid.UUID  `json:"project_id"`
	Name      string     `json:"name"`
	FieldType FieldType  `json:"field_type"`
	Options   []string   `json:"options"` // valid choices for dropdown
	Required  bool       `json:"required"`
	CreatedAt time.Time  `json:"created_at"`
}

// CustomFieldValue is the resolved value of a field on a specific issue.
type CustomFieldValue struct {
	FieldID   uuid.UUID `json:"field_id"`
	FieldName string    `json:"field_name"`
	FieldType FieldType `json:"field_type"`
	Value     *string   `json:"value"`
}

type CreateCustomFieldRequest struct {
	Name      string    `json:"name"       validate:"required,min=1,max=100"`
	FieldType FieldType `json:"field_type" validate:"required,oneof=text number dropdown date"`
	Options   []string  `json:"options"`
	Required  bool      `json:"required"`
}

type SetCustomFieldValueRequest struct {
	FieldID uuid.UUID `json:"field_id" validate:"required"`
	Value   *string   `json:"value"`
}
