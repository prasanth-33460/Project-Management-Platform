package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/prasanth-33460/Project-Management-Platform/internal/models"
)

type SprintRepository struct{ db *DB }

func NewSprintRepository(db *DB) *SprintRepository { return &SprintRepository{db: db} }

const sprintCols = `id, project_id, name, goal, start_date, end_date, status, velocity, created_at, updated_at`

func scanSprint(row pgx.Row) (*models.Sprint, error) {
	s := &models.Sprint{}
	err := row.Scan(&s.ID, &s.ProjectID, &s.Name, &s.Goal,
		&s.StartDate, &s.EndDate, &s.Status, &s.Velocity, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return s, err
}

func (r *SprintRepository) Create(ctx context.Context, projectID uuid.UUID, req *models.CreateSprintRequest) (*models.Sprint, error) {
	return scanSprint(r.db.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO sprints (project_id, name, goal, start_date, end_date)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING %s`, sprintCols),
		projectID, req.Name, req.Goal, flexDatePtr(req.StartDate), flexDatePtr(req.EndDate)))
}

func (r *SprintRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Sprint, error) {
	return scanSprint(r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT %s FROM sprints WHERE id = $1`, sprintCols), id))
}

func (r *SprintRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]*models.Sprint, error) {
	rows, err := r.db.Query(ctx,
		fmt.Sprintf(`SELECT %s FROM sprints WHERE project_id = $1 ORDER BY created_at DESC`, sprintCols),
		projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sprints []*models.Sprint
	for rows.Next() {
		s, err := scanSprint(rows)
		if err != nil {
			return nil, err
		}
		sprints = append(sprints, s)
	}
	return sprints, rows.Err()
}

func (r *SprintRepository) GetActiveSprint(ctx context.Context, projectID uuid.UUID) (*models.Sprint, error) {
	return scanSprint(r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT %s FROM sprints WHERE project_id = $1 AND status = 'active' LIMIT 1`, sprintCols),
		projectID))
}

func (r *SprintRepository) Update(ctx context.Context, id uuid.UUID, req *models.UpdateSprintRequest) (*models.Sprint, error) {
	return scanSprint(r.db.QueryRow(ctx, fmt.Sprintf(`
		UPDATE sprints
		SET name       = COALESCE($2, name),
		    goal       = COALESCE($3, goal),
		    start_date = COALESCE($4, start_date),
		    end_date   = COALESCE($5, end_date),
		    updated_at = NOW()
		WHERE id = $1
		RETURNING %s`, sprintCols),
		id, req.Name, req.Goal, flexDatePtr(req.StartDate), flexDatePtr(req.EndDate)))
}

// flexDatePtr converts *FlexDate to *time.Time; nil in means NULL out.
func flexDatePtr(d *models.FlexDate) *time.Time {
	if d == nil {
		return nil
	}
	t := d.Time
	return &t
}

func (r *SprintRepository) Start(ctx context.Context, id uuid.UUID) (*models.Sprint, error) {
	return scanSprint(r.db.QueryRow(ctx, fmt.Sprintf(`
		UPDATE sprints SET status = 'active', updated_at = NOW()
		WHERE id = $1 AND status = 'planned'
		RETURNING %s`, sprintCols), id))
}

func (r *SprintRepository) Complete(ctx context.Context, id uuid.UUID, velocity int) (*models.Sprint, error) {
	return scanSprint(r.db.QueryRow(ctx, fmt.Sprintf(`
		UPDATE sprints SET status = 'completed', velocity = $2, updated_at = NOW()
		WHERE id = $1
		RETURNING %s`, sprintCols), id, velocity))
}

func (r *SprintRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := r.db.Exec(ctx, `DELETE FROM sprints WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete sprint %s: %w", id, err)
	}
	return nil
}
