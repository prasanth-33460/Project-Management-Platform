package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/prasanth-33460/Project-Management-Platform/internal/models"
)

type IssueRepository struct{ db *DB }

func NewIssueRepository(db *DB) *IssueRepository { return &IssueRepository{db: db} }

// issueSelectCols is the full SELECT projection for issue rows, including joined status, assignee, and reporter.
const issueSelectCols = `
	i.id, i.project_id, i.issue_key, i.sprint_id, i.parent_id,
	i.type, i.title, i.description, i.status_id,
	i.priority, i.assignee_id, i.reporter_id,
	i.story_points, i.labels, i.version, i.created_at, i.updated_at,
	ws.id, ws.project_id, ws.name, ws.color, ws.position, ws.is_default, ws.is_done, ws.created_at,
	a.id,  a.email,  a.display_name,  a.avatar_url,
	rp.id, rp.email, rp.display_name, rp.avatar_url`

const issueJoins = `
	FROM issues i
	JOIN  workflow_statuses ws ON ws.id  = i.status_id
	LEFT JOIN users a          ON a.id   = i.assignee_id
	LEFT JOIN users rp         ON rp.id  = i.reporter_id`

// scanIssue reads a single issue row. Works with both pgx.Row and pgx.Rows.
func scanIssue(row pgx.Row) (*models.Issue, error) {
	issue := &models.Issue{
		Status:   &models.WorkflowStatus{},
		Reporter: &models.UserResponse{},
	}
	// nullable FK columns
	var (
		sprintID   *uuid.UUID
		parentID   *uuid.UUID
		assigneeID *uuid.UUID
		aID        *uuid.UUID
		aEmail     *string
		aName      *string
		aAvatar    *string
	)
	err := row.Scan(
		&issue.ID, &issue.ProjectID, &issue.IssueKey, &sprintID, &parentID,
		&issue.Type, &issue.Title, &issue.Description, &issue.StatusID,
		&issue.Priority, &assigneeID, &issue.ReporterID,
		&issue.StoryPoints, &issue.Labels, &issue.Version, &issue.CreatedAt, &issue.UpdatedAt,
		&issue.Status.ID, &issue.Status.ProjectID, &issue.Status.Name, &issue.Status.Color,
		&issue.Status.Position, &issue.Status.IsDefault, &issue.Status.IsDone, &issue.Status.CreatedAt,
		&aID, &aEmail, &aName, &aAvatar,
		&issue.Reporter.ID, &issue.Reporter.Email, &issue.Reporter.DisplayName, &issue.Reporter.AvatarURL,
	)
	if err != nil {
		return nil, err
	}
	issue.SprintID = sprintID
	issue.ParentID = parentID
	issue.AssigneeID = assigneeID
	if aID != nil {
		issue.Assignee = &models.UserResponse{ID: *aID, AvatarURL: aAvatar}
		if aEmail != nil {
			issue.Assignee.Email = *aEmail
		}
		if aName != nil {
			issue.Assignee.DisplayName = *aName
		}
	}
	if issue.Labels == nil {
		issue.Labels = []string{}
	}
	return issue, nil
}

func (r *IssueRepository) Create(ctx context.Context, issue *models.Issue) (*models.Issue, error) {
	if issue.Labels == nil {
		issue.Labels = []string{}
	}
	err := r.db.QueryRow(ctx, `
		INSERT INTO issues
		    (project_id, issue_key, sprint_id, parent_id, type, title, description,
		     status_id, priority, assignee_id, reporter_id, story_points, labels)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		RETURNING id, created_at, updated_at, version`,
		issue.ProjectID, issue.IssueKey, issue.SprintID, issue.ParentID,
		issue.Type, issue.Title, issue.Description,
		issue.StatusID, issue.Priority, issue.AssigneeID, issue.ReporterID,
		issue.StoryPoints, issue.Labels,
	).Scan(&issue.ID, &issue.CreatedAt, &issue.UpdatedAt, &issue.Version)
	if err != nil {
		return nil, fmt.Errorf("insert issue: %w", err)
	}
	return r.GetByID(ctx, issue.ID)
}

func (r *IssueRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Issue, error) {
	row := r.db.QueryRow(ctx, `SELECT `+issueSelectCols+issueJoins+` WHERE i.id = $1`, id)
	issue, err := scanIssue(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get issue %s: %w", id, err)
	}
	return issue, nil
}

// Update patches an issue. Status changes must go through WorkflowEngine — status_id is excluded here.
func (r *IssueRepository) Update(ctx context.Context, id uuid.UUID, req *models.UpdateIssueRequest) (*models.Issue, error) {
	setClauses := []string{"updated_at = NOW()", "version = version + 1"}
	args := []any{id}
	argIdx := 2

	if req.Title != nil {
		setClauses = append(setClauses, fmt.Sprintf("title = $%d", argIdx))
		args = append(args, *req.Title)
		argIdx++
	}
	if req.Description != nil {
		setClauses = append(setClauses, fmt.Sprintf("description = $%d", argIdx))
		args = append(args, *req.Description)
		argIdx++
	}
	if req.Priority != nil {
		setClauses = append(setClauses, fmt.Sprintf("priority = $%d", argIdx))
		args = append(args, string(*req.Priority))
		argIdx++
	}
	if req.AssigneeID != nil {
		setClauses = append(setClauses, fmt.Sprintf("assignee_id = $%d", argIdx))
		args = append(args, *req.AssigneeID)
		argIdx++
	}
	if req.StoryPoints != nil {
		setClauses = append(setClauses, fmt.Sprintf("story_points = $%d", argIdx))
		args = append(args, *req.StoryPoints)
		argIdx++
	}
	if req.Labels != nil {
		setClauses = append(setClauses, fmt.Sprintf("labels = $%d", argIdx))
		args = append(args, req.Labels)
		argIdx++
	}

	if len(setClauses) == 2 {
		// nothing to update, just return current state
		return r.GetByID(ctx, id)
	}

	query := fmt.Sprintf(`UPDATE issues SET %s WHERE id = $1`, strings.Join(setClauses, ", "))
	if _, err := r.db.Exec(ctx, query, args...); err != nil {
		return nil, fmt.Errorf("update issue %s: %w", id, err)
	}
	return r.GetByID(ctx, id)
}

// Delete removes an issue; children are cleaned up via ON DELETE CASCADE.
func (r *IssueRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := r.db.Exec(ctx, `DELETE FROM issues WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete issue %s: %w", id, err)
	}
	return nil
}

// List runs a filtered, cursor-paginated query.
// Full-text search uses the pre-computed tsvector GIN index on search_vector.
// The tsquery arg is reused for both the WHERE predicate and ORDER BY ts_rank.
func (r *IssueRepository) List(ctx context.Context, f models.IssueFilter) ([]*models.Issue, int, error) {
	where := []string{"1=1"}
	args := []any{}
	idx := 1
	searchArgIdx := 0 // position of the tsquery arg in args slice

	if f.ProjectID != nil {
		where = append(where, fmt.Sprintf("i.project_id = $%d", idx))
		args = append(args, *f.ProjectID)
		idx++
	}
	if f.SprintID != nil {
		where = append(where, fmt.Sprintf("i.sprint_id = $%d", idx))
		args = append(args, *f.SprintID)
		idx++
	}
	if f.StatusID != nil {
		where = append(where, fmt.Sprintf("i.status_id = $%d", idx))
		args = append(args, *f.StatusID)
		idx++
	}
	if f.AssigneeID != nil {
		where = append(where, fmt.Sprintf("i.assignee_id = $%d", idx))
		args = append(args, *f.AssigneeID)
		idx++
	}
	if f.Priority != nil {
		where = append(where, fmt.Sprintf("i.priority = $%d", idx))
		args = append(args, string(*f.Priority))
		idx++
	}
	if f.Type != nil {
		where = append(where, fmt.Sprintf("i.type = $%d", idx))
		args = append(args, string(*f.Type))
		idx++
	}
	if f.Query != "" {
		// match issues directly or issues that have a comment containing the query
		where = append(where, fmt.Sprintf(`(
			i.search_vector @@ plainto_tsquery('english', $%d)
			OR EXISTS (
				SELECT 1 FROM comments c
				WHERE c.issue_id = i.id
				  AND to_tsvector('english', coalesce(c.body, '')) @@ plainto_tsquery('english', $%d)
			)
		)`, idx, idx))
		args = append(args, f.Query)
		searchArgIdx = idx
		idx++
	}

	// snapshot before cursor clause so count query is unaffected by pagination
	countWhere := make([]string, len(where))
	copy(countWhere, where)
	countArgs := make([]any, len(args))
	copy(countArgs, args)

	if f.Cursor != "" {
		if cursor, err := models.DecodeCursor(f.Cursor); err == nil {
			where = append(where, fmt.Sprintf("(i.created_at, i.id) < ($%d, $%d)", idx, idx+1))
			args = append(args, cursor.CreatedAt, cursor.ID)
			idx += 2
		}
	}

	limit := f.Limit
	if limit <= 0 || limit > models.MaxPageSize {
		limit = models.DefaultPageSize
	}

	// count without cursor so total is stable across pages
	var total int
	if err := r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM issues i WHERE %s`, strings.Join(countWhere, " AND ")),
		countArgs...).Scan(&total); err != nil {
		slog.WarnContext(ctx, "issue count query failed", "error", err)
	}

	// rank by relevance when searching, otherwise newest-first
	orderBy := "i.created_at DESC, i.id DESC"
	if f.Query != "" {
		orderBy = fmt.Sprintf(
			"ts_rank(i.search_vector, plainto_tsquery('english', $%d)) DESC, i.created_at DESC, i.id DESC",
			searchArgIdx,
		)
	}

	query := fmt.Sprintf(`SELECT %s %s WHERE %s ORDER BY %s LIMIT %d`,
		issueSelectCols, issueJoins, strings.Join(where, " AND "), orderBy, limit)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list issues: %w", err)
	}
	defer rows.Close()

	var issues []*models.Issue
	for rows.Next() {
		issue, err := scanIssue(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan issue row: %w", err)
		}
		issues = append(issues, issue)
	}
	return issues, total, rows.Err()
}

// GetBacklog returns issues with no sprint assigned, ordered newest first.
func (r *IssueRepository) GetBacklog(ctx context.Context, projectID uuid.UUID) ([]*models.Issue, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+issueSelectCols+issueJoins+
			` WHERE i.project_id = $1 AND i.sprint_id IS NULL ORDER BY i.created_at DESC`,
		projectID)
	if err != nil {
		return nil, fmt.Errorf("get backlog: %w", err)
	}
	defer rows.Close()
	return collectIssueRows(rows)
}

func (r *IssueRepository) GetIncompleteBySprintID(ctx context.Context, sprintID uuid.UUID) ([]*models.Issue, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+issueSelectCols+issueJoins+
			` WHERE i.sprint_id = $1 AND ws.is_done = FALSE ORDER BY i.created_at ASC`,
		sprintID)
	if err != nil {
		return nil, fmt.Errorf("get incomplete issues: %w", err)
	}
	defer rows.Close()
	return collectIssueRows(rows)
}

func (r *IssueRepository) SumCompletedPoints(ctx context.Context, sprintID uuid.UUID) (int, error) {
	var total int
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(i.story_points), 0)
		FROM issues i
		JOIN workflow_statuses ws ON ws.id = i.status_id
		WHERE i.sprint_id = $1 AND ws.is_done = TRUE`, sprintID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("sum completed points: %w", err)
	}
	return total, nil
}

func (r *IssueRepository) GetWatchers(ctx context.Context, issueID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.db.Query(ctx, `SELECT user_id FROM watchers WHERE issue_id = $1`, issueID)
	if err != nil {
		return nil, fmt.Errorf("get watchers: %w", err)
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan watcher: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// AddWatcher subscribes a user to an issue. Idempotent.
func (r *IssueRepository) AddWatcher(ctx context.Context, issueID, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO watchers (issue_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		issueID, userID)
	if err != nil {
		return fmt.Errorf("add watcher: %w", err)
	}
	return nil
}

func (r *IssueRepository) RemoveWatcher(ctx context.Context, issueID, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM watchers WHERE issue_id = $1 AND user_id = $2`,
		issueID, userID)
	if err != nil {
		return fmt.Errorf("remove watcher: %w", err)
	}
	return nil
}

// LogActivity inserts an audit entry outside a transaction.
// Use TxStore.LogActivity when inside a transaction.
func (r *IssueRepository) LogActivity(ctx context.Context, entry *models.ActivityLog) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO activity_log
		    (issue_id, project_id, actor_id, event_type, field_name, old_value, new_value)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		entry.IssueID, entry.ProjectID, entry.ActorID, entry.EventType,
		entry.FieldName, entry.OldValue, entry.NewValue)
	if err != nil {
		return fmt.Errorf("log activity: %w", err)
	}
	return nil
}

func (r *IssueRepository) GetActivityFeed(ctx context.Context, projectID uuid.UUID, cursor string, limit int) ([]*models.ActivityLog, int, error) {
	if limit <= 0 || limit > models.MaxPageSize {
		limit = models.DefaultPageSize
	}

	args := []any{projectID}
	cursorClause := ""
	if cursor != "" {
		if c, err := models.DecodeCursor(cursor); err == nil {
			cursorClause = " AND (al.created_at, al.id) < ($2, $3)"
			args = append(args, c.CreatedAt, c.ID)
		}
	}

	var total int
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM activity_log WHERE project_id = $1`, projectID).Scan(&total); err != nil {
		slog.WarnContext(ctx, "activity feed count failed", "error", err, "project_id", projectID)
	}

	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT al.id, al.issue_id, al.project_id, al.actor_id,
		       u.id, u.email, u.display_name, u.avatar_url,
		       al.event_type, al.field_name, al.old_value, al.new_value, al.created_at
		FROM activity_log al
		JOIN users u ON u.id = al.actor_id
		WHERE al.project_id = $1 %s
		ORDER BY al.created_at DESC, al.id DESC
		LIMIT %d`, cursorClause, limit), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("get activity feed: %w", err)
	}
	defer rows.Close()

	var logs []*models.ActivityLog
	for rows.Next() {
		l := &models.ActivityLog{Actor: &models.UserResponse{}}
		if err := rows.Scan(
			&l.ID, &l.IssueID, &l.ProjectID, &l.ActorID,
			&l.Actor.ID, &l.Actor.Email, &l.Actor.DisplayName, &l.Actor.AvatarURL,
			&l.EventType, &l.FieldName, &l.OldValue, &l.NewValue, &l.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan activity log: %w", err)
		}
		logs = append(logs, l)
	}
	return logs, total, rows.Err()
}

// Pool exposes the underlying connection pool so the Transactor can open transactions.
func (r *IssueRepository) Pool() *Database { return r.db }

func collectIssueRows(rows pgx.Rows) ([]*models.Issue, error) {
	var issues []*models.Issue
	for rows.Next() {
		issue, err := scanIssue(rows)
		if err != nil {
			return nil, fmt.Errorf("scan issue: %w", err)
		}
		issues = append(issues, issue)
	}
	return issues, rows.Err()
}
