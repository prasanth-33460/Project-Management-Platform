package repository

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/prasanth-33460/Project-Management-Platform/internal/apperror"
	"github.com/prasanth-33460/Project-Management-Platform/internal/models"
)

type NotificationRepository struct{ db *DB }

func NewNotificationRepository(db *DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

func (r *NotificationRepository) Create(ctx context.Context, n *models.Notification) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO notifications (user_id, type, ref_id, ref_type, title, body)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`,
		n.UserID, n.Type, n.RefID, n.RefType, n.Title, n.Body,
	).Scan(&n.ID, &n.CreatedAt)
}

func (r *NotificationRepository) List(ctx context.Context, userID uuid.UUID, cursor string, limit int) ([]*models.Notification, int, error) {
	if limit <= 0 || limit > models.MaxPageSize {
		limit = models.DefaultPageSize
	}

	args := []interface{}{userID}
	cursorClause := ""
	if cursor != "" {
		c, err := models.DecodeCursor(cursor)
		if err == nil {
			cursorClause = " AND (created_at, id) < ($2, $3)"
			args = append(args, c.CreatedAt, c.ID)
		}
	}

	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM notifications WHERE user_id = $1`, userID).Scan(&total); err != nil {
		slog.WarnContext(ctx, "notification count failed", "user_id", userID, "error", err)
	}

	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT id, user_id, type, ref_id, ref_type, title, body, read, created_at
		FROM notifications
		WHERE user_id = $1 %s
		ORDER BY created_at DESC, id DESC
		LIMIT %d`, cursorClause, limit), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var notifs []*models.Notification
	for rows.Next() {
		n := &models.Notification{}
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.RefID, &n.RefType,
			&n.Title, &n.Body, &n.Read, &n.CreatedAt); err != nil {
			return nil, 0, err
		}
		notifs = append(notifs, n)
	}
	return notifs, total, rows.Err()
}

func (r *NotificationRepository) MarkRead(ctx context.Context, id, userID uuid.UUID) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE notifications SET read = TRUE WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("mark notification read: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

func (r *NotificationRepository) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE notifications SET read = TRUE WHERE user_id = $1 AND read = FALSE`, userID)
	return err
}

func (r *NotificationRepository) UnreadCount(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND read = FALSE`, userID).Scan(&count)
	return count, err
}
