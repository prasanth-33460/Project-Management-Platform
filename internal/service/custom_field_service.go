package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/prasanth-33460/Project-Management-Platform/internal/apperror"
	"github.com/prasanth-33460/Project-Management-Platform/internal/models"
	"github.com/prasanth-33460/Project-Management-Platform/internal/repository"
)

// CustomFieldService manages field definitions for a project and values on issues.
type CustomFieldService struct {
	fields repository.CustomFieldStore
	issues repository.IssueStore
}

func NewCustomFieldService(fields repository.CustomFieldStore, issues repository.IssueStore) *CustomFieldService {
	return &CustomFieldService{fields: fields, issues: issues}
}

func (s *CustomFieldService) CreateDefinition(ctx context.Context, projectID uuid.UUID, req *models.CreateCustomFieldRequest) (*models.CustomFieldDefinition, error) {
	return s.fields.CreateDefinition(ctx, projectID, req)
}

func (s *CustomFieldService) ListDefinitions(ctx context.Context, projectID uuid.UUID) ([]*models.CustomFieldDefinition, error) {
	defs, err := s.fields.ListDefinitions(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if defs == nil {
		defs = []*models.CustomFieldDefinition{}
	}
	return defs, nil
}

func (s *CustomFieldService) DeleteDefinition(ctx context.Context, id uuid.UUID) error {
	def, err := s.fields.GetDefinition(ctx, id)
	if err != nil {
		return err
	}
	if def == nil {
		return apperror.ErrNotFound
	}
	return s.fields.DeleteDefinition(ctx, id)
}

// SetValues upserts a batch of custom field values on an issue.
func (s *CustomFieldService) SetValues(ctx context.Context, issueID uuid.UUID, reqs []models.SetCustomFieldValueRequest) ([]*models.CustomFieldValue, error) {
	issue, err := s.issues.GetByID(ctx, issueID)
	if err != nil {
		return nil, err
	}
	if issue == nil {
		return nil, apperror.ErrNotFound
	}

	for _, r := range reqs {
		// verify the field definition exists
		def, err := s.fields.GetDefinition(ctx, r.FieldID)
		if err != nil {
			return nil, err
		}
		if def == nil {
			return nil, apperror.WithDetails(400, "custom field not found", map[string]string{
				"field_id": r.FieldID.String(),
			})
		}
		if def.ProjectID != issue.ProjectID {
			return nil, apperror.WithDetails(400, "field does not belong to this project", map[string]string{
				"field_id": r.FieldID.String(),
			})
		}
		if err := s.fields.SetValue(ctx, issueID, r.FieldID, r.Value); err != nil {
			return nil, err
		}
	}

	return s.fields.GetValues(ctx, issueID)
}

func (s *CustomFieldService) GetValues(ctx context.Context, issueID uuid.UUID) ([]*models.CustomFieldValue, error) {
	issue, err := s.issues.GetByID(ctx, issueID)
	if err != nil {
		return nil, err
	}
	if issue == nil {
		return nil, apperror.ErrNotFound
	}

	values, err := s.fields.GetValues(ctx, issueID)
	if err != nil {
		return nil, err
	}
	if values == nil {
		values = []*models.CustomFieldValue{}
	}
	return values, nil
}
