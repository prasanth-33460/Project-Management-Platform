package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/prasanth-33460/Project-Management-Platform/internal/apperror"
	"github.com/prasanth-33460/Project-Management-Platform/internal/models"
)

type ProjectRepository struct{ db *DB }

func NewProjectRepository(db *DB) *ProjectRepository { return &ProjectRepository{db: db} }

func (r *ProjectRepository) Create(ctx context.Context, req *models.CreateProjectRequest) (*models.Project, error) {
	p := &models.Project{}
	err := r.db.QueryRow(ctx, `
		INSERT INTO projects (key, name, description, lead_user_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id, key, name, description, lead_user_id, issue_counter, created_at, updated_at`,
		req.Key, req.Name, req.Description, req.LeadUserID,
	).Scan(&p.ID, &p.Key, &p.Name, &p.Description, &p.LeadUserID, &p.IssueCounter, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return nil, apperror.WithDetails(409, "project key already in use", map[string]string{"key": req.Key})
		}
		return nil, fmt.Errorf("create project: %w", err)
	}
	return p, nil
}

func (r *ProjectRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Project, error) {
	p := &models.Project{}
	err := r.db.QueryRow(ctx, `
		SELECT id, key, name, description, lead_user_id, issue_counter, created_at, updated_at
		FROM projects WHERE id = $1`, id,
	).Scan(&p.ID, &p.Key, &p.Name, &p.Description, &p.LeadUserID, &p.IssueCounter, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return p, err
}

func (r *ProjectRepository) List(ctx context.Context, userID uuid.UUID) ([]*models.Project, error) {
	rows, err := r.db.Query(ctx, `
		SELECT p.id, p.key, p.name, p.description, p.lead_user_id, p.issue_counter, p.created_at, p.updated_at
		FROM projects p
		JOIN project_members pm ON pm.project_id = p.id
		WHERE pm.user_id = $1
		ORDER BY p.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProjects(rows)
}

func (r *ProjectRepository) Update(ctx context.Context, id uuid.UUID, req *models.UpdateProjectRequest) (*models.Project, error) {
	p := &models.Project{}
	err := r.db.QueryRow(ctx, `
		UPDATE projects
		SET name        = COALESCE($2, name),
		    description = COALESCE($3, description),
		    lead_user_id = COALESCE($4, lead_user_id),
		    updated_at   = NOW()
		WHERE id = $1
		RETURNING id, key, name, description, lead_user_id, issue_counter, created_at, updated_at`,
		id, req.Name, req.Description, req.LeadUserID,
	).Scan(&p.ID, &p.Key, &p.Name, &p.Description, &p.LeadUserID, &p.IssueCounter, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return p, err
}

func (r *ProjectRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM projects WHERE id = $1`, id)
	return err
}

// AddMember adds a user to a project or updates their role if already a member.
func (r *ProjectRepository) AddMember(ctx context.Context, projectID, userID uuid.UUID, role string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO project_members (project_id, user_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (project_id, user_id) DO UPDATE SET role = EXCLUDED.role`,
		projectID, userID, role)
	return err
}

func (r *ProjectRepository) IsMember(ctx context.Context, projectID, userID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM project_members WHERE project_id=$1 AND user_id=$2)`,
		projectID, userID).Scan(&exists)
	return exists, err
}

// NextIssueKey atomically increments the counter and returns the new key, e.g. "PROJ-42".
func (r *ProjectRepository) NextIssueKey(ctx context.Context, projectID uuid.UUID) (string, error) {
	var key string
	var counter int
	err := r.db.QueryRow(ctx, `
		UPDATE projects SET issue_counter = issue_counter + 1
		WHERE id = $1
		RETURNING key, issue_counter`, projectID,
	).Scan(&key, &counter)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%d", key, counter), nil
}

func scanProjects(rows pgx.Rows) ([]*models.Project, error) {
	var projects []*models.Project
	for rows.Next() {
		p := &models.Project{}
		if err := rows.Scan(&p.ID, &p.Key, &p.Name, &p.Description,
			&p.LeadUserID, &p.IssueCounter, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}
