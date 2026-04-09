package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/prasanth-33460/Project-Management-Platform/internal/models"
)

type WorkflowRepository struct{ db *DB }

func NewWorkflowRepository(db *DB) *WorkflowRepository { return &WorkflowRepository{db: db} }

func (r *WorkflowRepository) CreateStatus(ctx context.Context, projectID uuid.UUID, req *models.CreateStatusRequest) (*models.WorkflowStatus, error) {
	s := &models.WorkflowStatus{}
	err := r.db.QueryRow(ctx, `
		INSERT INTO workflow_statuses (project_id, name, color, position, is_default, is_done)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, project_id, name, color, position, is_default, is_done, created_at`,
		projectID, req.Name, req.Color, req.Position, req.IsDefault, req.IsDone,
	).Scan(&s.ID, &s.ProjectID, &s.Name, &s.Color, &s.Position, &s.IsDefault, &s.IsDone, &s.CreatedAt)
	return s, err
}

func (r *WorkflowRepository) GetStatusByID(ctx context.Context, id uuid.UUID) (*models.WorkflowStatus, error) {
	s := &models.WorkflowStatus{}
	err := r.db.QueryRow(ctx, `
		SELECT id, project_id, name, color, position, is_default, is_done, created_at
		FROM workflow_statuses WHERE id = $1`, id,
	).Scan(&s.ID, &s.ProjectID, &s.Name, &s.Color, &s.Position, &s.IsDefault, &s.IsDone, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return s, err
}

func (r *WorkflowRepository) ListStatuses(ctx context.Context, projectID uuid.UUID) ([]*models.WorkflowStatus, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, project_id, name, color, position, is_default, is_done, created_at
		FROM workflow_statuses
		WHERE project_id = $1
		ORDER BY position ASC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var statuses []*models.WorkflowStatus
	for rows.Next() {
		s := &models.WorkflowStatus{}
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.Name, &s.Color, &s.Position, &s.IsDefault, &s.IsDone, &s.CreatedAt); err != nil {
			return nil, err
		}
		statuses = append(statuses, s)
	}
	return statuses, rows.Err()
}

func (r *WorkflowRepository) GetDefaultStatus(ctx context.Context, projectID uuid.UUID) (*models.WorkflowStatus, error) {
	s := &models.WorkflowStatus{}
	err := r.db.QueryRow(ctx, `
		SELECT id, project_id, name, color, position, is_default, is_done, created_at
		FROM workflow_statuses
		WHERE project_id = $1 AND is_default = TRUE
		LIMIT 1`, projectID,
	).Scan(&s.ID, &s.ProjectID, &s.Name, &s.Color, &s.Position, &s.IsDefault, &s.IsDone, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		// Fallback to position 0
		err = r.db.QueryRow(ctx, `
			SELECT id, project_id, name, color, position, is_default, is_done, created_at
			FROM workflow_statuses WHERE project_id = $1 ORDER BY position ASC LIMIT 1`, projectID,
		).Scan(&s.ID, &s.ProjectID, &s.Name, &s.Color, &s.Position, &s.IsDefault, &s.IsDone, &s.CreatedAt)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return s, err
}

func (r *WorkflowRepository) CreateTransition(ctx context.Context, projectID uuid.UUID, req *models.CreateTransitionRequest) (*models.WorkflowTransition, error) {
	actionsJSON, _ := json.Marshal(req.AutoActions)
	rulesJSON, _ := json.Marshal(req.ValidationRules)
	t := &models.WorkflowTransition{}
	var actionsRaw, rulesRaw []byte
	err := r.db.QueryRow(ctx, `
		INSERT INTO workflow_transitions (project_id, from_status_id, to_status_id, auto_actions, validation_rules)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (project_id, from_status_id, to_status_id) DO UPDATE
		    SET auto_actions = EXCLUDED.auto_actions,
		        validation_rules = EXCLUDED.validation_rules
		RETURNING id, project_id, from_status_id, to_status_id, auto_actions, validation_rules`,
		projectID, req.FromStatusID, req.ToStatusID, actionsJSON, rulesJSON,
	).Scan(&t.ID, &t.ProjectID, &t.FromStatusID, &t.ToStatusID, &actionsRaw, &rulesRaw)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(actionsRaw, &t.AutoActions)
	_ = json.Unmarshal(rulesRaw, &t.ValidationRules)
	return t, nil
}

// GetAllowedTransitions returns all transitions available from a given status.
func (r *WorkflowRepository) GetAllowedTransitions(ctx context.Context, projectID, fromStatusID uuid.UUID) ([]*models.WorkflowTransition, error) {
	rows, err := r.db.Query(ctx, `
		SELECT wt.id, wt.project_id, wt.from_status_id, wt.to_status_id,
		       wt.auto_actions, wt.validation_rules,
		       ws.id, ws.project_id, ws.name, ws.color, ws.position, ws.is_default, ws.is_done, ws.created_at
		FROM workflow_transitions wt
		JOIN workflow_statuses ws ON ws.id = wt.to_status_id
		WHERE wt.project_id = $1 AND wt.from_status_id = $2`,
		projectID, fromStatusID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transitions []*models.WorkflowTransition
	for rows.Next() {
		t := &models.WorkflowTransition{ToStatus: &models.WorkflowStatus{}}
		var actionsRaw, rulesRaw []byte
		if err := rows.Scan(
			&t.ID, &t.ProjectID, &t.FromStatusID, &t.ToStatusID,
			&actionsRaw, &rulesRaw,
			&t.ToStatus.ID, &t.ToStatus.ProjectID, &t.ToStatus.Name, &t.ToStatus.Color,
			&t.ToStatus.Position, &t.ToStatus.IsDefault, &t.ToStatus.IsDone, &t.ToStatus.CreatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(actionsRaw, &t.AutoActions)
		_ = json.Unmarshal(rulesRaw, &t.ValidationRules)
		transitions = append(transitions, t)
	}
	return transitions, rows.Err()
}

func (r *WorkflowRepository) DeleteTransition(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM workflow_transitions WHERE id = $1`, id)
	return err
}
