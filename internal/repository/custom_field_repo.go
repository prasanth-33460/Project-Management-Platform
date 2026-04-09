package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/prasanth-33460/Project-Management-Platform/internal/models"
)

type CustomFieldRepository struct{ db *DB }

func NewCustomFieldRepository(db *DB) *CustomFieldRepository {
	return &CustomFieldRepository{db: db}
}

func (r *CustomFieldRepository) CreateDefinition(ctx context.Context, projectID uuid.UUID, req *models.CreateCustomFieldRequest) (*models.CustomFieldDefinition, error) {
	if req.Options == nil {
		req.Options = []string{}
	}
	optionsJSON, _ := json.Marshal(req.Options)

	f := &models.CustomFieldDefinition{}
	var optRaw []byte
	err := r.db.QueryRow(ctx, `
		INSERT INTO custom_field_definitions (project_id, name, field_type, options, required)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, project_id, name, field_type, options, required, created_at`,
		projectID, req.Name, string(req.FieldType), optionsJSON, req.Required,
	).Scan(&f.ID, &f.ProjectID, &f.Name, &f.FieldType, &optRaw, &f.Required, &f.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create custom field definition: %w", err)
	}
	_ = json.Unmarshal(optRaw, &f.Options)
	if f.Options == nil {
		f.Options = []string{}
	}
	return f, nil
}

func (r *CustomFieldRepository) GetDefinition(ctx context.Context, id uuid.UUID) (*models.CustomFieldDefinition, error) {
	f := &models.CustomFieldDefinition{}
	var optRaw []byte
	err := r.db.QueryRow(ctx, `
		SELECT id, project_id, name, field_type, options, required, created_at
		FROM custom_field_definitions WHERE id = $1`, id,
	).Scan(&f.ID, &f.ProjectID, &f.Name, &f.FieldType, &optRaw, &f.Required, &f.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get custom field definition: %w", err)
	}
	_ = json.Unmarshal(optRaw, &f.Options)
	if f.Options == nil {
		f.Options = []string{}
	}
	return f, nil
}

func (r *CustomFieldRepository) ListDefinitions(ctx context.Context, projectID uuid.UUID) ([]*models.CustomFieldDefinition, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, project_id, name, field_type, options, required, created_at
		FROM custom_field_definitions
		WHERE project_id = $1
		ORDER BY created_at ASC`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list custom field definitions: %w", err)
	}
	defer rows.Close()

	var fields []*models.CustomFieldDefinition
	for rows.Next() {
		f := &models.CustomFieldDefinition{}
		var optRaw []byte
		if err := rows.Scan(&f.ID, &f.ProjectID, &f.Name, &f.FieldType, &optRaw, &f.Required, &f.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan custom field: %w", err)
		}
		_ = json.Unmarshal(optRaw, &f.Options)
		if f.Options == nil {
			f.Options = []string{}
		}
		fields = append(fields, f)
	}
	return fields, rows.Err()
}

func (r *CustomFieldRepository) DeleteDefinition(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM custom_field_definitions WHERE id = $1`, id)
	return err
}

// SetValue upserts a custom field value for an issue.
func (r *CustomFieldRepository) SetValue(ctx context.Context, issueID, fieldID uuid.UUID, value *string) error {
	if value == nil {
		// clear the value
		_, err := r.db.Exec(ctx,
			`DELETE FROM issue_custom_field_values WHERE issue_id = $1 AND field_id = $2`,
			issueID, fieldID)
		return err
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO issue_custom_field_values (issue_id, field_id, value)
		VALUES ($1, $2, $3)
		ON CONFLICT (issue_id, field_id) DO UPDATE SET value = EXCLUDED.value`,
		issueID, fieldID, *value)
	return err
}

// GetValues returns all custom field values for an issue, joined with field metadata.
func (r *CustomFieldRepository) GetValues(ctx context.Context, issueID uuid.UUID) ([]*models.CustomFieldValue, error) {
	rows, err := r.db.Query(ctx, `
		SELECT cfd.id, cfd.name, cfd.field_type, icfv.value
		FROM custom_field_definitions cfd
		JOIN issue_custom_field_values icfv ON icfv.field_id = cfd.id
		WHERE icfv.issue_id = $1
		ORDER BY cfd.created_at ASC`, issueID)
	if err != nil {
		return nil, fmt.Errorf("get custom field values: %w", err)
	}
	defer rows.Close()

	var values []*models.CustomFieldValue
	for rows.Next() {
		v := &models.CustomFieldValue{}
		if err := rows.Scan(&v.FieldID, &v.FieldName, &v.FieldType, &v.Value); err != nil {
			return nil, fmt.Errorf("scan custom field value: %w", err)
		}
		values = append(values, v)
	}
	return values, rows.Err()
}
